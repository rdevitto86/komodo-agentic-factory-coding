package eval

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// emptyClone makes a repo at dir on main with one empty commit and a local bare origin, neither with hooks.
func emptyClone(_ context.Context, _, _, dir string) error {
	steps := [][]string{
		{"init", "-q", "--template=", "-b", "main", dir},
		{"init", "-q", "--bare", "--template=", dir + ".origin.git"},
		{"-C", dir + ".origin.git", "config", "receive.autogc", "false"},
		{"-C", dir + ".origin.git", "config", "maintenance.auto", "false"},
		{"-C", dir, "remote", "add", "origin", dir + ".origin.git"},
		{"-C", dir, "-c", "user.name=eval", "-c", "user.email=eval@example.com", "commit", "-q", "--allow-empty", "-m", "base"},
	}
	for _, args := range steps {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %w: %s", args, err, out)
		}
	}
	return nil
}

// standIn writes a komodo that logs its arguments to calls, succeeds on install, and otherwise prints PROBE
// and exits 3.
func standIn(t *testing.T) (komodo, calls string) {
	t.Helper()
	calls = filepath.Join(t.TempDir(), "calls")
	komodo = filepath.Join(t.TempDir(), "komodo")
	script := "#!/bin/sh\necho \"$@\" >> " + calls + "\n[ \"$1\" = install ] && exit 0\necho \"probe=$PROBE\"\nexit 3\n"
	if err := os.WriteFile(komodo, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return komodo, calls
}

// TestNewLiveMountsCommitsAndDrivesTheGroupsClone proves the live env's clone, hooks, mount, commits and commands.
func TestNewLiveMountsCommitsAndDrivesTheGroupsClone(t *testing.T) {
	suite := offlineSuite(t)
	komodo, calls := standIn(t)
	dir := filepath.Join(t.TempDir(), "case-0")
	live, err := NewLive(context.Background(), LiveOptions{
		Suite: suite, Group: suite.Groups[0], Executable: komodo, Host: "h", Dir: dir, Clone: emptyClone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if live.Dir() != dir || live.Group() != "TG-01.1" {
		t.Fatalf("live = %s %s", live.Dir(), live.Group())
	}
	if hooks := gitIn(t, dir, "config", "core.hooksPath"); hooks != os.DevNull {
		t.Fatalf("core.hooksPath = %q, want the clone's hooks off", hooks)
	}
	if subject := gitIn(t, dir+".origin.git", "log", "-1", "--format=%s", "main"); subject != "eval: TG-01.1" {
		t.Fatalf("origin main = %q, want the group and mount committed and pushed", subject)
	}
	ran := live.Komodo(context.Background(), []string{"PROBE=one", "PROBE=two"}, "run", "TG-01.1")
	if ran.Code != 3 || strings.TrimSpace(ran.Output) != "probe=two" {
		t.Fatalf("Komodo = %+v, want exit 3 and the last PROBE", ran)
	}
	if logged, err := os.ReadFile(calls); err != nil || string(logged) != "install --host h\nrun TG-01.1\n" {
		t.Fatalf("komodo calls = %q, %v", logged, err)
	}
	if branch := live.Git(context.Background(), "branch", "--show-current"); branch.Code != 0 ||
		strings.TrimSpace(branch.Output) != "main" {
		t.Fatalf("Git = %+v, want main", branch)
	}
	if err := live.AddGroup(context.Background(), "TG-99.1", probeGroup("TG-99.1", "Probe", "p.txt", "write p.txt")); err != nil {
		t.Fatal(err)
	}
	if subject := gitIn(t, dir+".origin.git", "log", "-1", "--format=%s", "main"); subject != "eval: TG-99.1" {
		t.Fatalf("origin main = %q, want the added group pushed", subject)
	}
	backlog, err := os.ReadFile(filepath.Join(dir, "BACKLOG.md"))
	if err != nil || !strings.Contains(string(backlog), "#### [TSK-99.1.1]") {
		t.Fatalf("backlog = %q, %v, want the added group", backlog, err)
	}
	ended, cancel := context.WithCancel(context.Background())
	cancel()
	if err := live.AddGroup(ended, "TG-99.2", probeGroup("TG-99.2", "Probe", "q.txt", "write q.txt")); err == nil {
		t.Fatal("AddGroup after its context ended = nil, want the commit stopped")
	}
	gone := &Live{dir: dir, executable: filepath.Join(t.TempDir(), "absent")}
	if ran := gone.Komodo(context.Background(), nil, "run"); ran.Code != -1 {
		t.Fatalf("Komodo with no binary = %+v, want -1", ran)
	}
}

// TestNewLiveNamesTheStepThatFailed proves a clone, group, mount or commit failure stops the live env.
func TestNewLiveNamesTheStepThatFailed(t *testing.T) {
	komodo, _ := standIn(t)
	broken := func(context.Context, string, string, string) error { return errors.New("no route") }
	unborn := func(_ context.Context, _, _, dir string) error {
		return exec.Command("git", "init", "-q", "--template=", "-b", "main", dir).Run()
	}
	// A .git file naming no repo makes git stop there rather than walk up to a real checkout.
	unreadable := func(_ context.Context, _, _, dir string) error {
		return copyBytes(filepath.Join(dir, ".git"), "gitdir: "+filepath.Join(dir, "absent")+"\n")
	}
	suite := offlineSuite(t)
	lost := suite
	lost.Dir = t.TempDir()
	local := suite
	local.Repos = []Repo{{Name: "greet", URL: filepath.Join(t.TempDir(), "absent"), Language: "Go"}}
	cases := []struct {
		name       string
		suite      Suite
		executable string
		clone      Clone
		wants      string
	}{
		{"the clone fails", suite, komodo, broken, "clone: no route"},
		{"the default clone fails", local, komodo, nil, "clone: "},
		{"the clone is no repo", suite, komodo, unreadable, "core.hooksPath"},
		{"the group file is missing", lost, komodo, emptyClone, "TG-01.1.md"},
		{"the mount fails", suite, filepath.Join(t.TempDir(), "absent"), emptyClone, "absent install"},
		{"the push fails", suite, komodo, unborn, "push"},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			_, err := NewLive(context.Background(), LiveOptions{
				Suite: each.suite, Group: each.suite.Groups[0], Executable: each.executable,
				Dir: filepath.Join(t.TempDir(), "case-0"), Clone: each.clone,
			})
			if err == nil || !strings.Contains(err.Error(), each.wants) {
				t.Fatalf("NewLive = %v, want a failure naming %q", err, each.wants)
			}
		})
	}
}

// TestLiveCasesRunsEachCaseInAFreshCloneAndFailsOnAnyFailure proves the command's case run end to end.
func TestLiveCasesRunsEachCaseInAFreshCloneAndFailsOnAnyFailure(t *testing.T) {
	komodo, _ := standIn(t)
	suite := offlineSuite(t)
	live := LiveOptions{Suite: suite, Executable: komodo, Clone: emptyClone}
	var dirs []string
	proves := Case{Name: "proves", Requirement: "REQ-1", Run: func(_ context.Context, env Env) error {
		dirs = append(dirs, env.Dir())
		if env.Group() != "TG-01.1" {
			return errors.New("the wrong group")
		}
		return nil
	}}
	disproves := Case{Name: "disproves", Requirement: "REQ-2", Run: func(context.Context, Env) error {
		return errors.New("the run shipped")
	}}
	work := t.TempDir()
	var out strings.Builder
	if err := LiveCases(context.Background(), CaseOptions{Cases: []Case{proves, proves}, Stdout: &out}, live, work); err != nil {
		t.Fatalf("LiveCases = %v\n%s", err, out.String())
	}
	if !slices.Equal(dirs, []string{filepath.Join(work, "case-0"), filepath.Join(work, "case-1")}) {
		t.Fatalf("dirs = %q, want one fresh clone per case", dirs)
	}
	err := LiveCases(context.Background(), CaseOptions{Cases: []Case{proves, disproves}}, live, t.TempDir())
	if err == nil || err.Error() != "1 of 2 eval cases failed" {
		t.Fatalf("LiveCases = %v, want the one failure counted", err)
	}
	live.Clone = func(context.Context, string, string, string) error { return errors.New("no route") }
	if err := LiveCases(context.Background(), CaseOptions{Cases: []Case{proves}}, live, t.TempDir()); err == nil ||
		!strings.Contains(err.Error(), "no route") {
		t.Fatalf("LiveCases = %v, want the clone's failure", err)
	}
	if err := LiveCases(context.Background(), CaseOptions{}, LiveOptions{}, t.TempDir()); err == nil {
		t.Fatal("LiveCases with no group = nil, want a refusal")
	}
}

// TestLiveCasesSkipsTheCanaryWithNoInstructionsFile proves a missing instructions file notes the canary and
// fails nothing.
func TestLiveCasesSkipsTheCanaryWithNoInstructionsFile(t *testing.T) {
	komodo, _ := standIn(t)
	live := LiveOptions{Suite: offlineSuite(t), Executable: komodo, Clone: emptyClone}
	ran := false
	planted := Case{Name: canaryCase, Requirement: "REQ-3", Run: func(context.Context, Env) error {
		ran = true
		return errors.New("the canary ran with nowhere to plant it")
	}}
	var out strings.Builder
	if err := LiveCases(context.Background(), CaseOptions{Cases: []Case{planted}, Stdout: &out}, live, t.TempDir()); err != nil {
		t.Fatalf("LiveCases = %v, want the canary skipped\n%s", err, out.String())
	}
	if ran || !strings.Contains(out.String(), "case canary (REQ-3): skipped") {
		t.Fatalf("ran %v, stdout %q; want the canary skipped with a note", ran, out.String())
	}
	live.Instructions = filepath.Join(t.TempDir(), "instructions.md")
	if err := LiveCases(context.Background(), CaseOptions{Cases: []Case{planted}}, live, t.TempDir()); err == nil || !ran {
		t.Fatalf("LiveCases = %v, ran %v; want the canary run once a file is named", err, ran)
	}
}

// TestLiveCasesAddGroupEndsWithTheCase proves a case's AddGroup stops once the case's budget runs out.
func TestLiveCasesAddGroupEndsWithTheCase(t *testing.T) {
	komodo, _ := standIn(t)
	live := LiveOptions{Suite: offlineSuite(t), Executable: komodo, Clone: emptyClone}
	var added error
	late := Case{Name: "late", Requirement: "REQ-1", Run: func(ctx context.Context, env Env) error {
		<-ctx.Done()
		added = env.AddGroup("TG-99.1", probeGroup("TG-99.1", "Probe", "p.txt", "write p.txt"))
		return nil
	}}
	options := CaseOptions{Cases: []Case{late}, Budget: 50 * time.Millisecond}
	if err := LiveCases(context.Background(), options, live, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if added == nil {
		t.Fatal("AddGroup after the case's budget = nil, want the push stopped with the case")
	}
}

// TestLiveHidesCommandsFromPathAndLaysAnOverlayOverTheHome proves PathWithout and Overlay keep all else.
func TestLiveHidesCommandsFromPathAndLaysAnOverlayOverTheHome(t *testing.T) {
	live := &Live{dir: filepath.Join(t.TempDir(), "case-0")}
	plain, tools := t.TempDir(), t.TempDir()
	for _, name := range []string{"sandbox-exec", "git"} {
		if err := copyBytes(filepath.Join(tools, name), "#!/bin/sh\n"); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", plain+string(os.PathListSeparator)+tools)
	path, err := live.PathWithout("sandbox-exec", "bwrap")
	if err != nil {
		t.Fatal(err)
	}
	dirs := filepath.SplitList(path)
	if len(dirs) != 2 || dirs[0] != plain || dirs[1] == tools {
		t.Fatalf("PATH = %q, want %s kept and %s mirrored", path, plain, tools)
	}
	if _, err := os.Stat(filepath.Join(dirs[1], "git")); err != nil {
		t.Fatalf("the mirror lost git: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dirs[1], "sandbox-exec")); err == nil {
		t.Fatal("the mirror kept sandbox-exec")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	files := map[string]string{"login/token": "kept\n", ".komodo/config.json": `{"sandbox": true, "pause_at": 0.9}`,
		".komodo/scores.json": "{}\n"}
	for name, body := range files {
		if err := copyBytes(filepath.Join(home, name), body); err != nil {
			t.Fatal(err)
		}
	}
	extra, err := live.Overlay(`{"pause_at": 0.01}`)
	if err != nil {
		t.Fatal(err)
	}
	scratch, ok := strings.CutPrefix(extra[0], "HOME=")
	if !ok || scratch == home {
		t.Fatalf("extra = %q, want a scratch HOME", extra)
	}
	if data, err := os.ReadFile(filepath.Join(scratch, "login", "token")); err != nil || string(data) != "kept\n" {
		t.Fatalf("the scratch home lost the login: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(scratch, ".komodo", "scores.json")); err != nil {
		t.Fatalf("the scratch home lost the overlay's neighbours: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(scratch, ".komodo", "config.json"))
	if err != nil || !strings.Contains(string(data), `"pause_at": 0.01`) || !strings.Contains(string(data), `"sandbox": true`) {
		t.Fatalf("overlay = %q, %v, want the lowered pause mark over the real overlay", data, err)
	}
	if real, err := os.ReadFile(filepath.Join(home, ".komodo", "config.json")); err != nil ||
		!strings.Contains(string(real), "0.9") {
		t.Fatalf("the real overlay changed: %q, %v", real, err)
	}
	if _, err := live.Overlay("{"); err == nil {
		t.Fatal("Overlay with broken JSON = nil, want a refusal")
	}
	t.Setenv("HOME", t.TempDir())
	extra, err = live.Overlay(`{"pause_at": 0.01}`)
	if err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(strings.TrimPrefix(extra[0], "HOME="), ".komodo", "config.json")
	if data, err := os.ReadFile(fresh); err != nil || !strings.Contains(string(data), `"pause_at": 0.01`) {
		t.Fatalf("overlay with no real one = %q, %v, want the case's keys alone", data, err)
	}
}

// TestLiveHelpersStopWhenTheirFilesCannotBeMade proves each helper returns the failure of the file it needs.
func TestLiveHelpersStopWhenTheirFilesCannotBeMade(t *testing.T) {
	tools := t.TempDir()
	if err := copyBytes(filepath.Join(tools, "bwrap"), "#!/bin/sh\n"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools)
	homeless := &Live{dir: filepath.Join(t.TempDir(), "absent", "case-0")}
	if _, err := homeless.PathWithout("bwrap"); err == nil {
		t.Fatal("PathWithout with nowhere to mirror = nil, want the failure")
	}
	if _, err := homeless.Overlay("{}"); err == nil {
		t.Fatal("Overlay with nowhere for the home = nil, want the failure")
	}
	if err := homeless.AddGroup(context.Background(), "TG-99.1", "x"); err == nil {
		t.Fatal("AddGroup with nowhere for the section = nil, want the failure")
	}
	t.Setenv("HOME", "")
	if _, err := (&Live{dir: filepath.Join(t.TempDir(), "case-0")}).Overlay("{}"); err == nil {
		t.Fatal("Overlay with no home = nil, want the failure")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := copyBytes(filepath.Join(home, ".komodo", "config.json"), "{"); err != nil {
		t.Fatal(err)
	}
	if _, err := (&Live{dir: filepath.Join(t.TempDir(), "case-0")}).Overlay("{}"); err == nil ||
		!strings.Contains(err.Error(), "config.json") {
		t.Fatalf("Overlay over a broken real overlay = %v, want it named", err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := copyBytes(file, "x\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := (&Live{instructions: filepath.Join(file, "instructions.md")}).Plant("x"); err == nil {
		t.Fatal("Plant under a file = nil, want the failure")
	}
	unborn := &Live{dir: filepath.Join(t.TempDir(), "case-0")}
	if err := unborn.AddGroup(context.Background(), "TG-99/1", "x"); err == nil {
		t.Fatal("AddGroup with a section it cannot write = nil, want the failure")
	}
	if err := unborn.AddGroup(context.Background(), "TG-99.1", "x"); err == nil {
		t.Fatal("AddGroup with no clone to hold the backlog = nil, want the failure")
	}
	notDir := filepath.Join(t.TempDir(), "home")
	if err := copyBytes(filepath.Join(notDir, ".komodo"), "a file\n"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", notDir)
	if _, err := (&Live{dir: filepath.Join(t.TempDir(), "case-0")}).Overlay("{}"); err == nil {
		t.Fatal("Overlay over a .komodo that is a file = nil, want the failure")
	}
}

// TestLiveHelpersStopOnADirectoryTheyCannotReadOrWrite proves the mirror, Overlay and Plant return a denied
// directory's failure.
func TestLiveHelpersStopOnADirectoryTheyCannotReadOrWrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads and writes every directory")
	}
	locked := func(dir string) {
		t.Helper()
		if err := os.Chmod(dir, 0o100); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	}
	tools := t.TempDir()
	if err := copyBytes(filepath.Join(tools, "bwrap"), "#!/bin/sh\n"); err != nil {
		t.Fatal(err)
	}
	locked(tools)
	t.Setenv("PATH", tools)
	live := &Live{dir: filepath.Join(t.TempDir(), "case-0")}
	if _, err := live.PathWithout("bwrap"); err == nil {
		t.Fatal("PathWithout over an unlistable directory = nil, want the failure")
	}
	home := t.TempDir()
	locked(home)
	t.Setenv("HOME", home)
	if _, err := live.Overlay("{}"); err == nil {
		t.Fatal("Overlay over an unlistable home = nil, want the failure")
	}
	readOnly := t.TempDir()
	if err := os.Chmod(readOnly, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o755) })
	for _, path := range []string{filepath.Join(readOnly, "instructions.md"), filepath.Join(readOnly, "sub", "instructions.md")} {
		if _, err := (&Live{instructions: path}).Plant("x"); err == nil {
			t.Fatalf("Plant at %s = nil, want the failure", path)
		}
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\necho gho_fake\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	parent := t.TempDir()
	if err := os.Mkdir(filepath.Join(parent, "case-0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
	if _, _, err := (&Live{dir: filepath.Join(parent, "case-0")}).Credential(); err == nil {
		t.Fatal("Credential with nowhere for the gh config = nil, want the failure")
	}
}

// TestLiveKomodoKillsARunWhenItsContextEnds proves a cancelled run's process ends and reports no exit code.
func TestLiveKomodoKillsARunWhenItsContextEnds(t *testing.T) {
	sleeper := filepath.Join(t.TempDir(), "komodo")
	if err := os.WriteFile(sleeper, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	started := time.Now()
	ran := (&Live{dir: t.TempDir(), executable: sleeper}).Komodo(ctx, nil, "run")
	if ran.Code != -1 || time.Since(started) > 10*time.Second {
		t.Fatalf("Komodo = %+v after %s, want the run killed with no exit code", ran, time.Since(started))
	}
}

// TestLivePlantsAndRestoresThePersonalInstructions proves Plant restores a file it found and removes one it made.
func TestLivePlantsAndRestoresThePersonalInstructions(t *testing.T) {
	if _, err := (&Live{}).Plant("x"); err == nil {
		t.Fatal("Plant with no instructions file = nil, want a refusal")
	}
	for _, was := range []string{"", "# mine\n"} {
		path := filepath.Join(t.TempDir(), "home", "instructions.md")
		if was != "" {
			if err := copyBytes(path, was); err != nil {
				t.Fatal(err)
			}
		}
		restore, err := (&Live{instructions: path}).Plant("say CANARY")
		if err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(path); err != nil || !strings.HasPrefix(string(data), was) ||
			!strings.Contains(string(data), "say CANARY") {
			t.Fatalf("planted = %q, %v", data, err)
		}
		if err := restore(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if was == "" && !errors.Is(err, os.ErrNotExist) || was != "" && string(data) != was {
			t.Fatalf("restored = %q, %v, want %q", data, err, was)
		}
	}
}

// TestLiveCredentialHandsTheRunATokenItCanTakeAway proves the scratch gh config holds the token until removed.
func TestLiveCredentialHandsTheRunATokenItCanTakeAway(t *testing.T) {
	live := &Live{dir: t.TempDir()}
	for _, token := range []string{"gho_fake", ""} {
		bin := t.TempDir()
		if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\necho "+token+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		extra, remove, err := live.Credential()
		if token == "" {
			if err == nil {
				t.Fatal("Credential with no token = nil, want a refusal")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		config, ok := strings.CutPrefix(extra[len(extra)-1], "GH_CONFIG_DIR=")
		if !ok || !slices.Contains(extra, "GH_TOKEN=") {
			t.Fatalf("extra = %q, want the env token cleared and a scratch gh config", extra)
		}
		if data, err := os.ReadFile(filepath.Join(config, "hosts.yml")); err != nil || !strings.Contains(string(data), token) {
			t.Fatalf("hosts.yml = %q, %v", data, err)
		}
		if err := remove(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(config); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("the credential outlived its removal: %v", err)
		}
	}
}

// TestLiveCasesProveEveryRequirementThroughRealSessions runs every eval case through the whole line on this host.
func TestLiveCasesProveEveryRequirementThroughRealSessions(t *testing.T) {
	if os.Getenv(liveEnv) != "1" {
		t.Skip("set KOMODO_LIVE=1 to run real sessions; this spends plan tokens")
	}
	source, parent, _ := sourceRepo(t)
	suite, err := Load(testSuite)
	if err != nil {
		t.Fatal(err)
	}
	group := suite.Groups[0]
	group.Commit = parent
	suite.Repos = []Repo{{Name: group.Repo, URL: source, Language: "Go"}}
	suite.Groups = []Group{group}
	binary := filepath.Join(t.TempDir(), "komodo")
	if out, err := exec.Command("go", "build", "-o", binary, "komodo/cmd/komodo").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v: %s", err, out)
	}
	var out strings.Builder
	options := CaseOptions{Cases: Cases(), Stdout: &out}
	if err := LiveCases(context.Background(), options, LiveOptions{Suite: suite, Executable: binary}, t.TempDir()); err != nil {
		t.Fatalf("LiveCases = %v\n%s", err, out.String())
	}
}
