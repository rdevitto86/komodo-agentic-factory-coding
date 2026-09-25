package run

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"komodo/internal/install"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/pr"
)

// env reads one key back out of a scrubbed environment.
func env(list []string, key string) (string, bool) {
	for _, entry := range list {
		if name, value, found := strings.Cut(entry, "="); found && name == key {
			return value, true
		}
	}
	return "", false
}

func TestScrubDropsEveryPushCredential(t *testing.T) {
	base := []string{
		"GH_TOKEN=secret", "GITHUB_TOKEN=secret", "GH_ENTERPRISE_TOKEN=secret",
		"GIT_ASKPASS=/bin/askpass", "SSH_AUTH_SOCK=/tmp/agent.sock", "PATH=/usr/bin",
	}
	scrubbed := Scrub(base)
	for _, key := range dropped {
		if _, found := env(scrubbed, key); found {
			t.Fatalf("%s survived the scrub", key)
		}
	}
	if path, _ := env(scrubbed, "PATH"); path != "/usr/bin" {
		t.Fatalf("PATH = %q, want the inherited value", path)
	}
}

func TestScrubLeavesGitUnableToPromptOrAuthenticate(t *testing.T) {
	scrubbed := Scrub([]string{"PATH=/usr/bin"})
	want := map[string]string{
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_CONFIG_COUNT":    "1",
		"GIT_CONFIG_KEY_0":    "credential.helper",
		"GIT_CONFIG_VALUE_0":  "",
	}
	for key, value := range want {
		if got, found := env(scrubbed, key); !found || got != value {
			t.Fatalf("%s = %q, %v; want %q", key, got, found, value)
		}
	}
	if ssh, _ := env(scrubbed, "GIT_SSH_COMMAND"); !strings.Contains(ssh, "IdentitiesOnly=yes") {
		t.Fatalf("GIT_SSH_COMMAND = %q", ssh)
	}
}

func TestScrubIgnoresAnIdentityFileConfiguredOutsideTheEnvironment(t *testing.T) {
	scrubbed := Scrub([]string{"PATH=/usr/bin"})
	ssh, _ := env(scrubbed, "GIT_SSH_COMMAND")
	if !strings.Contains(ssh, "-F "+os.DevNull) {
		t.Fatalf("GIT_SSH_COMMAND = %q, want -F %s so ~/.ssh/config never applies", ssh, os.DevNull)
	}
}

func TestScrubDropsATokenByItsNameShapeNotJustAnExactSpelling(t *testing.T) {
	base := []string{
		"GITHUB_PAT=secret", "HOMEBREW_GITHUB_API_TOKEN=secret", "GIT_CONFIG_PARAMETERS=secret",
		"PATH=/usr/bin",
	}
	scrubbed := Scrub(base)
	for _, key := range []string{"GITHUB_PAT", "HOMEBREW_GITHUB_API_TOKEN", "GIT_CONFIG_PARAMETERS"} {
		if _, found := env(scrubbed, key); found {
			t.Fatalf("%s survived the scrub", key)
		}
	}
}

func TestScrubKeepsTheModelHostsOwnLoginAndDropsEveryForgeSecret(t *testing.T) {
	base := []string{"MODELHOST_OAUTH_TOKEN=login", "PATH=/usr/bin", "GITLAB_TOKEN=secret", "BITBUCKET_PASSWORD=secret"}
	scrubbed := Scrub(base)
	for _, key := range []string{"MODELHOST_OAUTH_TOKEN", "PATH"} {
		if _, found := env(scrubbed, key); !found {
			t.Fatalf("%s was scrubbed; only a forge's push credential may be", key)
		}
	}
	for _, key := range []string{"GITLAB_TOKEN", "BITBUCKET_PASSWORD"} {
		if _, found := env(scrubbed, key); found {
			t.Fatalf("%s survived the scrub", key)
		}
	}
}

func TestScrubDoesNotLetAnInheritedOverrideSurvive(t *testing.T) {
	scrubbed := Scrub([]string{"GIT_TERMINAL_PROMPT=1", "GIT_SSH_COMMAND=ssh -i /home/me/.ssh/id_ed25519"})
	if got, _ := env(scrubbed, "GIT_TERMINAL_PROMPT"); got != "0" {
		t.Fatalf("GIT_TERMINAL_PROMPT = %q, want the launcher's own value", got)
	}
	if ssh, _ := env(scrubbed, "GIT_SSH_COMMAND"); strings.Contains(ssh, "id_ed25519") {
		t.Fatalf("an inherited SSH identity survived: %q", ssh)
	}
	count := 0
	for _, entry := range scrubbed {
		if strings.HasPrefix(entry, "GIT_SSH_COMMAND=") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("GIT_SSH_COMMAND appears %d times", count)
	}
}

func TestTheRunsPathFindsKomodoAsTheRunningBinaryAndReplacesAStaleLink(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(t.TempDir(), "komodo-built")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(root, line.StateDir, "bin")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "gone"), filepath.Join(stale, "komodo")); err != nil {
		t.Fatal(err)
	}
	out, err := withBinPath([]string{"PATH=/usr/bin", "HOME=/h"}, root, executable)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := env(out, "PATH")
	dirs := filepath.SplitList(path)
	if len(dirs) != 3 || dirs[1] != "/usr/bin" || dirs[2] != filepath.Join(root, "bin") {
		t.Fatalf("PATH = %q, want the link dir, the inherited PATH, then root/bin", path)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(dirs[0], "komodo"))
	if err != nil {
		t.Fatalf("no komodo on the run's PATH: %v", err)
	}
	want, _ := filepath.EvalSymlinks(executable)
	if resolved != want {
		t.Fatalf("komodo on PATH = %s, want %s", resolved, want)
	}
}

func TestTheRunsPathCopiesKomodoWhenASymlinkIsRefused(t *testing.T) {
	saved := symlink
	t.Cleanup(func() { symlink = saved })
	symlink = func(string, string) error { return os.ErrPermission }
	root := t.TempDir()
	executable := filepath.Join(t.TempDir(), "komodo-built")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := withBinPath([]string{"PATH=/usr/bin"}, root, executable)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := env(out, "PATH")
	dirs := filepath.SplitList(path)
	link := filepath.Join(root, line.StateDir, "bin")
	if len(dirs) != 3 || dirs[0] != link {
		t.Fatalf("PATH = %q, want the copy's dir first", path)
	}
	names, _ := filepath.Glob(filepath.Join(link, "komodo*"))
	if len(names) != 1 {
		t.Fatalf("no copy of komodo in %s", link)
	}
	if data, _ := os.ReadFile(names[0]); string(data) != "#!/bin/sh\n" {
		t.Fatalf("copy holds %q", data)
	}
}

func TestTheRunsPathFallsBackToTheBinarysOwnDirWhenNothingCanBeWritten(t *testing.T) {
	saved := symlink
	t.Cleanup(func() { symlink = saved })
	symlink = func(string, string) error { return os.ErrPermission }
	root := t.TempDir()
	executable := filepath.Join(t.TempDir(), "missing", "komodo")
	out, err := withBinPath([]string{"PATH=/usr/bin"}, root, executable)
	if err != nil {
		t.Fatalf("a refused link and copy failed the run: %v", err)
	}
	path, _ := env(out, "PATH")
	dirs := filepath.SplitList(path)
	if len(dirs) != 3 || dirs[0] != filepath.Dir(executable) || dirs[1] != "/usr/bin" {
		t.Fatalf("PATH = %q, want the binary's own dir first", path)
	}
}

func TestLaunchWithNoMountSaysToInstall(t *testing.T) {
	code, err := Launch(Options{Root: t.TempDir(), Target: "TG-01.1", DryRun: true})
	if code == 0 || err == nil {
		t.Fatalf("code = %d, err = %v; want a refusal", code, err)
	}
	if !strings.Contains(err.Error(), "komodo install") {
		t.Fatalf("err = %v", err)
	}
}

func TestLaunchKillsARunThatPassesItsBudget(t *testing.T) {
	if _, err := os.Stat("/bin/sleep"); err != nil {
		t.Skip("no sleep on this machine")
	}
	var out bytes.Buffer
	code, err := launch(Options{
		Root: t.TempDir(), Budget: 50 * time.Millisecond,
		Stdout: &out, Stderr: &out, Env: []string{"PATH=/usr/bin:/bin"},
	}, "/bin/sleep", []string{"30"})
	if code != 124 || err == nil {
		t.Fatalf("code = %d, err = %v; want the budget kill", code, err)
	}
}

func TestLaunchReturnsTheHostsExitCode(t *testing.T) {
	var out bytes.Buffer
	code, err := launch(Options{
		Root: t.TempDir(), Stdout: &out, Stderr: &out, Env: []string{"PATH=/usr/bin:/bin"},
	}, "/bin/sh", []string{"-c", "exit 7"})
	if err != nil {
		t.Fatal(err)
	}
	if code != 7 {
		t.Fatalf("code = %d, want 7", code)
	}
}

func TestLaunchTeesTheHeadlessJSONToTheHostsUsageEventsFile(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh on this machine")
	}
	saved := mount.Snapshot()
	defer mount.Restore(saved)
	mount.Register(mount.Host{
		Name:      "codex",
		Installed: func(string) bool { return true },
		Headless: func(skill, target string) (string, []string) {
			return "/bin/sh", []string{"-c", `echo '{"type":"turn.completed"}'`}
		},
		EventsPath: func(root, task string) string {
			return filepath.Join(root, line.StateDir, "codex", task+".jsonl")
		},
	})
	root := t.TempDir()
	var out, errOut bytes.Buffer
	code, err := Launch(Options{
		Root: root, Target: "TSK-01.1.1", Stdout: &out, Stderr: &errOut, Env: []string{"PATH=/usr/bin:/bin"},
	})
	if err != nil || code != 0 {
		t.Fatalf("code = %d, err = %v", code, err)
	}
	events, err := os.ReadFile(filepath.Join(root, line.StateDir, "codex", "TSK-01.1.1.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(events), "turn.completed") {
		t.Fatalf("events file = %q", events)
	}
	if !strings.Contains(out.String(), "turn.completed") {
		t.Fatalf("stdout lost the tee: %q", out.String())
	}
}

// runGit runs one git command in dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// writeHandoff writes a ship handoff file the way a scrubbed ship leaves it, under its group's run directory.
func writeHandoff(t *testing.T, root string, handoff line.ShipHandoff) {
	t.Helper()
	path := line.HandoffPath(root, handoff.Group)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(handoff)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFinishShipPushesOpensThePullRequestThenStampsAndClearsTheHandoff(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	runGit(t, root, "remote", "add", "origin", bare)
	runGit(t, root, "checkout", "-b", "feat/a-group")
	if err := os.WriteFile(filepath.Join(root, "one.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "seed")
	writeHandoff(t, root, line.ShipHandoff{
		Group: "TG-01.1", Branch: "feat/a-group", Base: "main", Title: "t", Body: "b", Labels: []string{"@agent"},
	})
	created := false
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		switch {
		case len(args) > 1 && args[0] == "pr" && args[1] == "create":
			created = true
			return "https://example.invalid/pr/1", nil
		case len(args) > 0 && args[0] == "label":
			return "[]", nil
		}
		t.Fatalf("gh must not run any other command: %v", args)
		return "", nil
	}}
	var stderr bytes.Buffer
	if _, err := finishShip(Options{Root: root, Target: "TG-01.1", PR: client, Stderr: &stderr}); err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("gh pr create never ran")
	}
	if !strings.Contains(stderr.String(), "the repo has no @agent label") {
		t.Fatalf("a handoff ship dropped the missing label silently: %q", stderr.String())
	}
	if _, err := os.Stat(line.HandoffPath(root, "TG-01.1")); !os.IsNotExist(err) {
		t.Fatalf("ship.json survived a finished ship: %v", err)
	}
	entries, err := line.Book(root).All()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if entry.Station == "ship" && entry.Outcome == "done" {
			found = true
		}
	}
	if !found {
		t.Fatal("the ledger never recorded ship done")
	}
	out, err := exec.Command("git", "ls-remote", "--heads", bare, "feat/a-group").Output()
	if err != nil || len(out) == 0 {
		t.Fatalf("the branch never reached origin: %v %q", err, out)
	}
}

func TestFinishShipRefusesARefspecOrCriticalBranchAnAgentWrote(t *testing.T) {
	for _, branch := range []string{"+HEAD:main", "--mirror", "main", "feat/x:main", "feat/x..y"} {
		bare := filepath.Join(t.TempDir(), "origin.git")
		runGit(t, "", "init", "--bare", bare)
		root := t.TempDir()
		runGit(t, root, "init", "-b", "main")
		runGit(t, root, "config", "user.email", "a@example.com")
		runGit(t, root, "config", "user.name", "a")
		runGit(t, root, "remote", "add", "origin", bare)
		runGit(t, root, "commit", "--allow-empty", "-m", "seed")
		writeHandoff(t, root, line.ShipHandoff{Group: "TG-01.1", Branch: branch, Base: "main", Title: "t", Body: "b"})
		client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
			t.Fatalf("gh ran for %q: %v", branch, args)
			return "", nil
		}}
		if _, err := finishShip(Options{Root: root, Target: "TG-01.1", PR: client}); err == nil {
			t.Fatalf("finishShip pushed the handoff branch %q", branch)
		}
		if out, _ := exec.Command("git", "ls-remote", bare).Output(); len(out) != 0 {
			t.Fatalf("origin received refs for %q: %s", branch, out)
		}
	}
}

func TestFinishShipReadsOnlyItsOwnGroupsHandoff(t *testing.T) {
	root := t.TempDir()
	writeHandoff(t, root, line.ShipHandoff{Group: "TG-02.1", Branch: "+HEAD:main", Base: "main", Title: "t", Body: "b"})
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh ran for another group's handoff: %v", args)
		return "", nil
	}}
	if url, err := finishShip(Options{Root: root, Target: "TG-01.1", PR: client}); err != nil || url != "" {
		t.Fatalf("url = %q, err = %v; TG-01.1 has no handoff of its own", url, err)
	}
	if _, err := os.Stat(line.HandoffPath(root, "TG-02.1")); err != nil {
		t.Fatalf("another group's handoff was touched: %v", err)
	}
}

func TestDrainOrderPutsEveryOpenRunFirst(t *testing.T) {
	root := drainRepo(t)
	started := time.Now().UTC()
	for index, group := range []string{"TG-07.2", "TG-07.1"} {
		state := line.RunState{Run: group + "-1", Group: group, Base: "main", Branch: "feat/" + group, Worktree: root,
			Started: started.Add(time.Duration(index) * time.Second)}
		if err := line.SaveRun(root, state); err != nil {
			t.Fatal(err)
		}
	}
	order, err := drainOrder(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(order, " "); got != "TG-07.2 TG-07.1" {
		t.Fatalf("order = %q; both open runs drain first, oldest start first", got)
	}
}

func TestFinishShipDoesNothingWithNoHandoff(t *testing.T) {
	if _, err := finishShip(Options{Root: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if _, err := finishShip(Options{Root: t.TempDir(), Target: "TG-01.1"}); err != nil {
		t.Fatal(err)
	}
}

const drainText = "### [TG-07.1] First\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
	"#### [TSK-07.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"true\"]\n```\n\n" +
	"### [TG-07.2] Second\n```yaml\ntype: feat\nversion: 1.1.0\n```\n\n" +
	"#### [TSK-07.2.1] Two [P: C] [READY]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"true\"]\n```\n"

// fakeScript plays one group: it records the launch, then copies in the files staged for that group.
const fakeScript = `echo "$1" >> .komodo/fake/launched
cp ".komodo/fake/$1.md" BACKLOG.md 2>/dev/null
mkdir -p ".komodo/runs/$1" && cp ".komodo/fake/$1.json" ".komodo/runs/$1/ship.json" 2>/dev/null
exit 0`

// drainRepo builds a remoted repo holding drainText, one local branch per group, and a fake host that plays each group.
func drainRepo(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh on this machine")
	}
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	runGit(t, root, "remote", "add", "origin", bare)
	runGit(t, root, "commit", "--allow-empty", "-m", "seed")
	runGit(t, root, "push", "origin", "main")
	runGit(t, root, "branch", "feat/first")
	runGit(t, root, "branch", "feat/second")
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(drainText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, line.StateDir, "fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	saved := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(saved) })
	mount.Register(mount.Host{
		Name:      "fakehost-drain",
		Installed: func(string) bool { return true },
		Headless: func(skill, target string) (string, []string) {
			return "/bin/sh", []string{"-c", fakeScript, "sh", target}
		},
	})
	return root
}

// stageShip stages what the fake host leaves for a group: its tasks closed and its branch handed off.
func stageShip(t *testing.T, root, group, branch, backlogAfter string) {
	t.Helper()
	dir := filepath.Join(root, line.StateDir, "fake")
	if err := os.WriteFile(filepath.Join(dir, group+".md"), []byte(backlogAfter), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(line.ShipHandoff{Group: group, Branch: branch, Base: "main", Title: "t", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, group+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// fakeForge answers pr create with a numbered pull request and label with none.
func fakeForge(t *testing.T, root string) *pr.Client {
	created := 0
	return &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		switch {
		case len(args) > 1 && args[0] == "pr" && args[1] == "create":
			created++
			return "https://example.invalid/pr/" + strconv.Itoa(created), nil
		case len(args) > 0 && args[0] == "label":
			return "[]", nil
		}
		t.Fatalf("gh must not run any other command: %v", args)
		return "", nil
	}}
}

// launched reads which groups the fake host was given, in order.
func launched(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, line.StateDir, "fake", "launched"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

func TestDrainLaunchesAndShipsTwoReadyGroupsInOrder(t *testing.T) {
	root := drainRepo(t)
	firstDone := strings.Replace(drainText, "One [P: C] [READY]", "One [P: C] [DONE]", 1)
	stageShip(t, root, "TG-07.1", "feat/first", firstDone)
	stageShip(t, root, "TG-07.2", "feat/second", strings.Replace(firstDone, "Two [P: C] [READY]", "Two [P: C] [DONE]", 1))
	var out bytes.Buffer
	code, err := Launch(Options{
		Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out,
		Env: []string{"PATH=/usr/bin:/bin"}, PR: fakeForge(t, root),
	})
	if err != nil || code != 0 {
		t.Fatalf("code = %d, err = %v, out = %s", code, err, out.String())
	}
	if got := launched(t, root); got != "TG-07.1\nTG-07.2" {
		t.Fatalf("launched = %q; want both groups in order", got)
	}
	for _, want := range []string{
		"TG-07.1 shipped: https://example.invalid/pr/1",
		"TG-07.2 shipped: https://example.invalid/pr/2",
		"nothing is ready",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestDrainStopsAtAGroupThatEndsUnshipped(t *testing.T) {
	root := drainRepo(t)
	var out bytes.Buffer
	code, err := Launch(Options{
		Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out,
		Env: []string{"PATH=/usr/bin:/bin"}, PR: fakeForge(t, root),
	})
	if err != nil {
		t.Fatal(err)
	}
	if code == 0 {
		t.Fatalf("code = 0; a group that ends unshipped must stop the drain with a failure")
	}
	if got := launched(t, root); got != "TG-07.1" {
		t.Fatalf("launched = %q; the drain must stop before the next group", got)
	}
	if !strings.Contains(out.String(), "TG-07.1 stopped: it ended without shipping") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestDrainDryRunListsTheGroupsInOrderAndLaunchesNothing(t *testing.T) {
	root := drainRepo(t)
	stacked := drainText + "\n### [TG-07.3] Third\n```yaml\ntype: feat\nversion: 1.2.0\nbase: feat/second\n```\n\n" +
		"#### [TSK-07.3.1] Three [P: C] [READY]\n```yaml\nfiles: [c/three.go]\ndone_when: [\"true\"]\n```\n\n" +
		"### [TG-07.4] Fourth\n```yaml\ntype: feat\nversion: 1.3.0\nbase: feat/missing\n```\n\n" +
		"#### [TSK-07.4.1] Four [P: C] [READY]\n```yaml\nfiles: [d/four.go]\ndone_when: [\"true\"]\n```\n"
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(stacked), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code, err := Launch(Options{Root: root, DryRun: true, Stdout: &out, Stderr: &out})
	if err != nil || code != 0 {
		t.Fatalf("code = %d, err = %v", code, err)
	}
	if got := strings.TrimSpace(out.String()); got != "1. TG-07.1\n2. TG-07.2\n3. TG-07.3" {
		t.Fatalf("dry run = %q; want the ready groups in order, without the one whose base is missing", got)
	}
	if got := launched(t, root); got != "" {
		t.Fatalf("a dry run launched %q", got)
	}
}

func TestABareDrainBudgetsEveryGroupItPlans(t *testing.T) {
	root := drainRepo(t)
	total, err := drainBudget(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2*GroupBudget {
		t.Fatalf("total = %s; two planned groups get %s each", total, GroupBudget)
	}
	if given, _ := drainBudget(root, time.Minute); given != time.Minute {
		t.Fatalf("total = %s; a given --budget is the whole drain's budget", given)
	}
}

func TestDrainStopsWhenTheWholeBudgetIsSpent(t *testing.T) {
	root := drainRepo(t)
	var out bytes.Buffer
	code, err := Launch(Options{
		Root: root, Budget: time.Nanosecond, Stdout: &out, Stderr: &out,
		Env: []string{"PATH=/usr/bin:/bin"}, PR: fakeForge(t, root),
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 124 || !strings.Contains(out.String(), "TG-07.1 stopped: the whole 1ns budget is spent") {
		t.Fatalf("code = %d, out = %s; a spent budget must stop the drain with the budget code", code, out.String())
	}
	if got := launched(t, root); got != "" {
		t.Fatalf("launched = %q; nothing may launch once the budget is spent", got)
	}
}

func TestDrainStopsAGroupThatComesUpAgainAfterItShipped(t *testing.T) {
	root := drainRepo(t)
	stageShip(t, root, "TG-07.1", "feat/first", drainText)
	var out bytes.Buffer
	code, err := Launch(Options{
		Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out,
		Env: []string{"PATH=/usr/bin:/bin"}, PR: fakeForge(t, root),
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 || !strings.Contains(out.String(), "TG-07.1 stopped: it came up again after it shipped") {
		t.Fatalf("code = %d, out = %s; a shipped group that is still ready must stop the drain", code, out.String())
	}
	if got := launched(t, root); got != "TG-07.1" {
		t.Fatalf("launched = %q; the repeated group must not launch twice", got)
	}
}

func TestDrainReRendersTheRootWhenTheDoctorReportsDrift(t *testing.T) {
	root := drainRepo(t)
	rendered := filepath.Join(root, "rendered.md")
	if err := os.WriteFile(rendered, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	host, _ := mount.Get("fakehost-drain")
	host.Render = func(dir, binary string) (install.Plan, error) {
		plan := install.Plan{Host: host.Name, Root: dir}
		plan.AddProject(filepath.Join(dir, "rendered.md"), []byte("fresh\n"), "kept in sync")
		return plan, nil
	}
	mount.Register(host)
	var out bytes.Buffer
	if _, err := Launch(Options{
		Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out,
		Env: []string{"PATH=/usr/bin:/bin"}, PR: fakeForge(t, root),
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(rendered)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "fresh\n" {
		t.Fatalf("rendered.md = %q; a drifted root must be re-rendered before a group launches", data)
	}
}
