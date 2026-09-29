package run

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/guard"
	"komodo/internal/install"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/mount/claude"
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

func TestLaunchTargetSkipsFinishShipWithNoShip(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh on this machine")
	}
	saved := mount.Snapshot()
	defer mount.Restore(saved)
	mount.Register(mount.Host{
		Name:      "codex",
		Installed: func(string) bool { return true },
		Headless: func(skill, target string) (string, []string) {
			return "/bin/sh", []string{"-c", "exit 0"}
		},
	})
	root := t.TempDir()
	writeHandoff(t, root, line.ShipHandoff{Group: "TG-01.1", Branch: "feat/a-group", Base: "main", Title: "t", Body: "b"})
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh must not run with --no-ship: %v", args)
		return "", nil
	}}
	code, url, err := launchTarget(Options{
		Root: root, Target: "TG-01.1", NoShip: true, PR: client, Env: []string{"PATH=/usr/bin:/bin"},
	})
	if err != nil || code != 0 {
		t.Fatalf("code = %d, err = %v", code, err)
	}
	if url != "" {
		t.Fatalf("url = %q, want none with --no-ship", url)
	}
	if _, err := os.Stat(line.HandoffPath(root, "TG-01.1")); err != nil {
		t.Fatalf("ship.json was consumed with --no-ship: %v", err)
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

func TestFinishShipRunsAfterPublishWithoutThePushCredentials(t *testing.T) {
	t.Setenv("GH_TOKEN", "secret-token")
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	runGit(t, root, "remote", "add", "origin", bare)
	runGit(t, root, "checkout", "-b", "feat/a-group")
	runGit(t, root, "commit", "--allow-empty", "-m", "seed")
	writeHandoff(t, root, line.ShipHandoff{
		Group: "TG-01.1", Branch: "feat/a-group", Base: "main", Title: "t", Body: "b", AfterPublish: "env > after.env",
	})
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		if len(args) > 1 && args[0] == "pr" && args[1] == "create" {
			return "https://example.invalid/pr/1", nil
		}
		return "[]", nil
	}}
	if _, err := finishShip(Options{Root: root, Target: "TG-01.1", PR: client}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "after.env"))
	if err != nil {
		t.Fatalf("after_publish never ran: %v", err)
	}
	env := string(data)
	if strings.Contains(env, "secret-token") || !strings.Contains(env, "GIT_TERMINAL_PROMPT=0") {
		t.Fatalf("after_publish, which an agent can write, ran with the push credentials:\n%s", env)
	}
}

func TestFinishShipPushesFromTheGroupsWorktreeSoThePrePushGateJudgesTheBranch(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", "-b", "main", bare)
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	runGit(t, root, "remote", "add", "origin", bare)
	runGit(t, root, "commit", "--allow-empty", "-m", "seed")
	worktree := filepath.Join(root, ".komodo", "wt", "TG-01.1")
	runGit(t, root, "worktree", "add", "-b", "feat/a-group", worktree)
	runGit(t, worktree, "commit", "--allow-empty", "-m", "group")
	marker := filepath.Join(t.TempDir(), "pushed-from")
	hook := "#!/bin/sh\ngit rev-parse --show-toplevel > " + marker + "\n"
	if err := os.WriteFile(filepath.Join(root, ".git", "hooks", "pre-push"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	writeHandoff(t, root, line.ShipHandoff{
		Group: "TG-01.1", Worktree: worktree, Branch: "feat/a-group", Base: "main", Title: "t", Body: "b",
	})
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		if len(args) > 1 && args[0] == "pr" && args[1] == "create" {
			return "https://example.invalid/pr/1", nil
		}
		return "[]", nil
	}}
	if _, err := finishShip(Options{Root: root, Target: "TG-01.1", PR: client}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("the pre-push hook never ran: %v", err)
	}
	got, _ := filepath.EvalSymlinks(strings.TrimSpace(string(data)))
	want, _ := filepath.EvalSymlinks(worktree)
	if got != want {
		t.Fatalf("the push ran from %s, want the group's worktree %s", got, want)
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

// driveDrainText is two ready groups, each declaring the file its own fake claude session writes.
const driveDrainText = "### [TG-07.1] First\n```yaml\ntype: feat\nversion: 1.0.0\nmode: single\n```\n\n" +
	"#### [TSK-07.1.1] One [P: C] [READY]\n```yaml\nfiles: [one.txt]\ndone_when: [\"true\"]\n```\n\n" +
	"### [TG-07.2] Second\n```yaml\ntype: feat\nversion: 1.1.0\nmode: single\n```\n\n" +
	"#### [TSK-07.2.1] Two [P: C] [READY]\n```yaml\nfiles: [two.txt]\ndone_when: [\"true\"]\n```\n"

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

// drainDriveFakeClaude plays every lane's session by its KOMODO_ROLE, except FAKE_BLOCK_GROUP's own.
const drainDriveFakeClaude = `#!/bin/sh
if [ "$1" != "-p" ]; then echo "2.0.0"; exit 0; fi
group=$(basename "$PWD")
if [ "$group" = "$FAKE_BLOCK_GROUP" ]; then
  if [ "$KOMODO_ROLE" = "orchestrator" ]; then cat "$FAKE_STOP_FIXTURE"; else cat "$FAKE_BLOCK_FIXTURE"; fi
  exit 0
fi
if [ "$KOMODO_ROLE" = "builder" ]; then
  case "$group" in
    TG-07.1) echo built > one.txt ;;
    TG-07.2) echo built > two.txt ;;
    *) echo built > change.txt ;;
  esac
  cat "$FAKE_BUILD_FIXTURE"
else
  cat "$FAKE_REVIEW_FIXTURE"
fi
`

// fakeBlockFixture is a builder result that stops a group for the orchestrator.
const fakeBlockFixture = `{"type":"result","subtype":"success","is_error":false,"num_turns":1,` +
	`"session_id":"build-blocked","total_cost_usd":0,"usage":{"input_tokens":1,"output_tokens":1},` +
	`"structured_output":{"result":"BLOCKED","question":"stuck"}}` + "\n"

// fakeStopFixture is an orchestrator's answer that gives up on a group, leaving it blocked.
const fakeStopFixture = `{"type":"result","subtype":"success","is_error":false,"num_turns":1,` +
	`"session_id":"escalate-stop","total_cost_usd":0,"usage":{"input_tokens":1,"output_tokens":1},` +
	`"structured_output":{"action":"stop","needs":"a person's call"}}` + "\n"

// setupDrainDriveFakeClaude drops the fake claude script on a fresh PATH entry, with its fixtures at
// absolute paths, so a drain's lanes each build then review clean through the real conductor.
func setupDrainDriveFakeClaude(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	if err := os.WriteFile(script, []byte(drainDriveFakeClaude), 0o755); err != nil {
		t.Fatal(err)
	}
	fixtures := t.TempDir()
	for name, text := range map[string]string{
		"build.jsonl": buildFixture, "review.jsonl": reviewFixture,
		"block.jsonl": fakeBlockFixture, "stop.jsonl": fakeStopFixture,
	} {
		if err := os.WriteFile(filepath.Join(fixtures, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_BUILD_FIXTURE", filepath.Join(fixtures, "build.jsonl"))
	t.Setenv("FAKE_REVIEW_FIXTURE", filepath.Join(fixtures, "review.jsonl"))
	t.Setenv("FAKE_BLOCK_FIXTURE", filepath.Join(fixtures, "block.jsonl"))
	t.Setenv("FAKE_STOP_FIXTURE", filepath.Join(fixtures, "stop.jsonl"))
}

// driveDrainRepo builds a root remoted at a bare origin, with main pushed and the claude mount installed,
// holding backlogText, so a drain's lanes each cut a real worktree and branch through the conductor.
func driveDrainRepo(t *testing.T, backlogText string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(backlogText), 0o644); err != nil {
		t.Fatal(err)
	}
	ignore := "/" + line.StateDir + "/\n/" + claude.Dir + "/\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(ignore), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "seed")
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	runGit(t, root, "remote", "add", "origin", bare)
	runGit(t, root, "push", "origin", "main")
	if err := os.MkdirAll(filepath.Join(root, claude.Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, claude.Dir, "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
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

// fakeForge answers pr create with a numbered pull request, pr view with the URL its own head branch
// got, and label with none, so a lane can look its own pull request back up once Drive ships it.
func fakeForge(t *testing.T, root string) *pr.Client {
	var lock sync.Mutex
	created := 0
	urls := map[string]string{}
	return &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		lock.Lock()
		defer lock.Unlock()
		switch {
		case len(args) > 1 && args[0] == "pr" && args[1] == "create":
			created++
			url := "https://example.invalid/pr/" + strconv.Itoa(created)
			urls[argAfter(args, "--head")] = url
			return url, nil
		case len(args) > 2 && args[0] == "pr" && args[1] == "view":
			if url, ok := urls[args[2]]; ok {
				return fmt.Sprintf(`{"number":%d,"url":%q,"state":"OPEN"}`, created, url), nil
			}
			return "", fmt.Errorf("no pull request for %s", args[2])
		case len(args) > 0 && args[0] == "label":
			return "[]", nil
		case len(args) > 1 && args[0] == "pr" && args[1] == "ready":
			return "", nil
		}
		t.Errorf("gh must not run any other command: %v", args)
		return "", fmt.Errorf("gh must not run any other command: %v", args)
	}}
}

// argAfter is the value gh args holds right after flag, or empty when flag is absent.
func argAfter(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
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

// TestDrainDrivesEachLaneThroughTheConductorAndShipsInOrder proves a drain runs each of its groups
// to Shipped through Drive, no relay involved, and prints the pull request each one opened.
func TestDrainDrivesEachLaneThroughTheConductorAndShipsInOrder(t *testing.T) {
	root := driveDrainRepo(t, driveDrainText)
	setupDrainDriveFakeClaude(t)
	var out bytes.Buffer
	code, err := Launch(Options{Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out, PR: fakeForge(t, root)})
	if err != nil || code != 0 {
		t.Fatalf("code = %d, err = %v, out = %s", code, err, out.String())
	}
	for _, want := range []string{
		"TG-07.1 shipped: https://example.invalid/pr/",
		"TG-07.2 shipped: https://example.invalid/pr/",
		"nothing is ready",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output lacks %q:\n%s", want, out.String())
		}
	}
}

// TestDrainParksAGroupThatEndsUnshippedAndRunsTheNext proves a group the conductor cannot settle
// parks without a URL, and the drain still drives the next ready group to Shipped.
func TestDrainParksAGroupThatEndsUnshippedAndRunsTheNext(t *testing.T) {
	root := driveDrainRepo(t, driveDrainText)
	setupDrainDriveFakeClaude(t)
	t.Setenv("FAKE_BLOCK_GROUP", "TG-07.1")
	var out bytes.Buffer
	code, err := Launch(Options{Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out, PR: fakeForge(t, root)})
	if err != nil {
		t.Fatal(err)
	}
	if code == 0 {
		t.Fatalf("code = 0; a drain that parked a group must end with a failure")
	}
	for _, want := range []string{
		"TG-07.1 parked:",
		"stopped at Blocked, not Shipped",
		"TG-07.2 shipped: https://example.invalid/pr/",
		"drain done: nothing is ready; 1 shipped, 1 parked (TG-07.1)",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestDrainDryRunListsTheGroupsInOrderAndLaunchesNothing(t *testing.T) {
	root := drainRepo(t)
	stacked := drainText + "\n### [TG-07.3] Third\n```yaml\ntype: feat\nversion: 1.2.0\nbase: feat/TG-07.2-second\n```\n\n" +
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

// TestDrainSkipsAGroupThatComesUpAgainAfterItShipped proves a group still READY on the root's own
// BACKLOG.md, since shipping never rewrites it, does not launch twice within the one drain.
func TestDrainSkipsAGroupThatComesUpAgainAfterItShipped(t *testing.T) {
	root := driveDrainRepo(t, driveDrainText)
	setupDrainDriveFakeClaude(t)
	var out bytes.Buffer
	code, err := Launch(Options{Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out, PR: fakeForge(t, root)})
	if err != nil || code != 0 {
		t.Fatalf("code = %d, err = %v, out = %s", code, err, out.String())
	}
	if !strings.Contains(out.String(), "drain done: nothing is ready; 2 shipped, 0 parked") {
		t.Fatalf("output = %s", out.String())
	}
}

// laneScript plays one group, recording itself when the group named second runs at the same time.
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

// TestDrainSyncsOnceBeforeAndOnceAfterTheWholeRunNeverBetweenGroups proves the drain's own sync runs
// exactly twice around the whole run, never per lane, while every lane still ships through Drive.
func TestDrainSyncsOnceBeforeAndOnceAfterTheWholeRunNeverBetweenGroups(t *testing.T) {
	root := driveDrainRepo(t, driveDrainText)
	setupDrainDriveFakeClaude(t)
	var out bytes.Buffer
	code, err := Launch(Options{Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out, PR: fakeForge(t, root)})
	if code != 0 || err != nil {
		t.Fatalf("launch failed: code %d, err %v", code, err)
	}
	output := out.String()
	if got := strings.Count(output, "root: already current"); got != 2 {
		t.Fatalf("root sync ran %d times, want exactly 2: once before the run and once after; output:\n%s", got, output)
	}
	if !strings.Contains(output, "TG-07.1 shipped") || !strings.Contains(output, "TG-07.2 shipped") {
		t.Fatalf("both groups should have shipped; output:\n%s", output)
	}
}

func TestADrainRestacksNothingInARepoWithNoBacklog(t *testing.T) {
	var out bytes.Buffer
	restack(Options{Root: t.TempDir()}, &out)
	if out.Len() != 0 {
		t.Fatalf("out = %q, want nothing restacked", out.String())
	}
}

// restackText is an epic whose child group depends on its parent.
const restackText = "### [TG-01.1] Parent\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
	"#### [TSK-01.1.1] One [P: C] [DONE]\n```yaml\nfiles: [a.txt]\n```\n\n" +
	"### [TG-01.2] Child\n```yaml\ntype: feat\nversion: 1.0.0\ndepends_on: [TG-01.1]\n```\n\n" +
	"#### [TSK-01.2.1] Two [P: C] [READY]\n```yaml\nfiles: [b.txt]\n```\n"

func TestADrainRestacksAChildOntoTheEpicOnceItsParentMerged(t *testing.T) {
	cases := []struct {
		name string
		// clash is whether the epic also adds the child's file, so the child's rebase onto it conflicts.
		clash bool
		want  string
	}{
		{"the child rebases onto the epic", false, "restacked TG-01.2 onto feat/1.0.0\n"},
		{"a child that conflicts is printed and left", true, "restack: restacking TG-01.2 onto feat/1.0.0: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, root := restackDrain(t, tc.clash)
			if !strings.HasPrefix(out, tc.want) {
				t.Fatalf("out = %q, want it to start %q", out, tc.want)
			}
			moved := !tc.clash
			if saved, _ := line.LoadRunFor(root, "TG-01.2"); (saved.Base == "feat/1.0.0") != moved {
				t.Fatalf("base = %q; the epic's branch recorded must be %v", saved.Base, moved)
			}
		})
	}
}

// restackDrain builds an epic whose parent merged and whose child is stacked on the parent, the epic adding the
// child's file too when clash is set, then runs the drain's restack and returns what it printed and the root.
func restackDrain(t *testing.T, clash bool) (string, string) {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	for name, text := range map[string]string{"BACKLOG.md": restackText, ".gitignore": "/.komodo/\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "backlog")
	runGit(t, root, "branch", "feat/1.0.0")
	parsed := backlog.Parse(restackText)
	parent, _ := parsed.Group("TG-01.1")
	child, _ := parsed.Group("TG-01.2")
	runGit(t, root, "checkout", "-q", "-b", parent.Branch())
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "parent")
	runGit(t, root, "checkout", "-q", "feat/1.0.0")
	runGit(t, root, "merge", "-q", "--no-ff", "--no-edit", parent.Branch())
	if clash {
		if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("the epic's own b\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, root, "add", "-A")
		runGit(t, root, "commit", "-q", "-m", "epic edits b")
	}
	runGit(t, root, "checkout", "-q", "main")
	worktree := filepath.Join(t.TempDir(), "child")
	runGit(t, root, "worktree", "add", "-q", "-b", child.Branch(), worktree, parent.Branch())
	if err := os.WriteFile(filepath.Join(worktree, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, worktree, "add", "-A")
	runGit(t, worktree, "commit", "-q", "-m", "child")
	state := line.RunState{Run: "r1", Group: "TG-01.2", Base: parent.Branch(), Branch: child.Branch(), Worktree: worktree}
	if err := line.SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	restack(Options{Root: root, PR: &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		t.Errorf("gh must not run for a branch never pushed: %v", args)
		return "", nil
	}}}, &out)
	return out.String(), root
}

// TestLaunchAndDriveMarkEverySessionAsLine proves both entries set the guard's line marker, so no
// session komodo run starts falls to the orchestrator's global tier only.
func TestLaunchAndDriveMarkEverySessionAsLine(t *testing.T) {
	for name, entry := range map[string]func(Options) (int, error){"Launch": Launch, "Drive": Drive} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(guard.RoleEnv, "")
			_, _ = entry(Options{Root: t.TempDir(), Target: "TG-01.1", DryRun: true, Stdout: io.Discard, Stderr: io.Discard})
			if got := os.Getenv(guard.RoleEnv); got != LineRole {
				t.Fatalf("%s left %s = %q, want %q", name, guard.RoleEnv, got, LineRole)
			}
		})
	}
}
