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
	"komodo/internal/backlog/backlogtest"
	"komodo/internal/conductor"
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

// TestLaunchDryRunPrintsTheTargetWithNoMountAndNoModel proves a targeted dry run needs no mount and
// starts no model: it only names the group Drive would run.
func TestLaunchDryRunPrintsTheTargetWithNoMountAndNoModel(t *testing.T) {
	var out bytes.Buffer
	code, err := Launch(Options{Root: t.TempDir(), Target: "TG-01.1", DryRun: true, Stdout: &out})
	if err != nil || code != 0 {
		t.Fatalf("code = %d, err = %v", code, err)
	}
	if got := out.String(); got != "1. TG-01.1\n" {
		t.Fatalf("out = %q, want the one target named, no model started", got)
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

const drainText = "### [TG-07.1] First\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
	"#### [TSK-07.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"true\"]\n```\n\n" +
	"### [TG-07.2] Second\n```yaml\ntype: feat\nversion: 1.1.0\n```\n\n" +
	"#### [TSK-07.2.1] Two [P: C] [READY]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"true\"]\n```\n"

// driveDrainText is two ready groups, each declaring the file its own fake claude session writes.
const driveDrainText = "### [TG-07.1] First\n```yaml\ntype: feat\nversion: 1.0.0\nmode: single\n```\n\n" +
	"#### [TSK-07.1.1] One [P: C] [READY]\n```yaml\nfiles: [one.txt]\ndone_when: [\"true\"]\n```\n\n" +
	"### [TG-07.2] Second\n```yaml\ntype: feat\nversion: 1.1.0\nmode: single\n```\n\n" +
	"#### [TSK-07.2.1] Two [P: C] [READY]\n```yaml\nfiles: [two.txt]\ndone_when: [\"true\"]\n```\n"

// driveDrainSharedFileText is driveDrainText's two groups, both declaring the same file.
const driveDrainSharedFileText = "### [TG-07.1] First\n```yaml\ntype: feat\nversion: 1.0.0\nmode: single\n```\n\n" +
	"#### [TSK-07.1.1] One [P: C] [READY]\n```yaml\nfiles: [one.txt]\ndone_when: [\"true\"]\n```\n\n" +
	"### [TG-07.2] Second\n```yaml\ntype: feat\nversion: 1.1.0\nmode: single\n```\n\n" +
	"#### [TSK-07.2.1] Two [P: C] [READY]\n```yaml\nfiles: [one.txt]\ndone_when: [\"true\"]\n```\n"

// fakeConcurrency is the fake host's own lanes: one by default, four once a test names a busier plan.
func fakeConcurrency(plan string) int {
	if plan == "max_20x" {
		return 4
	}
	return 1
}

// drainRepo builds a remoted repo holding drainText, one local branch per group, and a fake host that plays each group.
func drainRepo(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh on this machine")
	}
	// A real drain pins its own binary onto PATH for the whole process; restore it once the test ends.
	t.Setenv("PATH", os.Getenv("PATH"))
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
	backlogtest.SeedText(t, root, drainText)
	if err := os.MkdirAll(filepath.Join(root, line.StateDir, "fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	saved := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(saved) })
	mount.Register(mount.Host{
		Name:        "fakehost-drain",
		Installed:   func(string) bool { return true },
		Concurrency: fakeConcurrency,
	})
	return root
}

// drainDriveFakeClaude plays every lane's session by its KOMODO_ROLE, except FAKE_BLOCK_GROUP's own.
const drainDriveFakeClaude = `#!/bin/sh
if [ "$1" != "-p" ]; then echo "2.0.0"; exit 0; fi
group=$(basename "$PWD")
if [ "$KOMODO_ROLE" = "escalation" ] && [ -n "$FAKE_DEBUG" ]; then cat > "$FAKE_DEBUG" ; fi
if [ "$group" = "$FAKE_BLOCK_GROUP" ]; then
  if [ "$KOMODO_ROLE" = "escalation" ]; then cat "$FAKE_STOP_FIXTURE"; else cat "$FAKE_BLOCK_FIXTURE"; fi
  exit 0
fi
if [ "$KOMODO_ROLE" = "builder" ]; then
  if [ "$group" = "$FAKE_WAIT_GROUP" ] && [ -n "$FAKE_WAIT_FILE" ]; then
    i=0
    while [ ! -f "$FAKE_WAIT_FILE" ] && [ $i -lt 200 ]; do sleep 0.1; i=$((i+1)); done
  fi
  if [ -n "$FAKE_PATH_FILE" ] && [ ! -f "$FAKE_PATH_FILE" ]; then echo "$PATH" > "$FAKE_PATH_FILE"; fi
  if [ -n "$FAKE_OVERLAP_DIR" ]; then
    touch "$FAKE_OVERLAP_DIR/$group.started"
    i=0
    while [ -z "$(ls "$FAKE_OVERLAP_DIR"/*.started 2>/dev/null | grep -v "/$group.started\$")" ] && [ $i -lt 30 ]; do
      sleep 0.1
      i=$((i+1))
    done
    other=$(ls "$FAKE_OVERLAP_DIR"/*.started 2>/dev/null | grep -v "/$group.started\$")
    if [ -n "$other" ]; then
      ended=$(ls "$FAKE_OVERLAP_DIR"/*.ended 2>/dev/null | grep -v "/$group.ended\$")
      if [ -z "$ended" ]; then echo "$group" >> "$FAKE_OVERLAP_DIR/overlapped"; fi
    fi
  fi
  if [ -n "$FAKE_SHARED_FILE" ]; then
    echo built > one.txt
  else
    case "$group" in
      TG-07.1) echo built > one.txt ;;
      TG-07.2) echo built > two.txt ;;
      *) echo built > change.txt ;;
    esac
  fi
  cat "$FAKE_BUILD_FIXTURE"
  if [ -n "$FAKE_OVERLAP_DIR" ]; then touch "$FAKE_OVERLAP_DIR/$group.ended"; fi
  if [ "$group" = "$FAKE_STALE_GROUP" ] && [ -n "$FAKE_ROOT" ]; then echo stale > "$FAKE_ROOT/bin/.built-from"; fi
else
  cat "$FAKE_REVIEW_FIXTURE"
fi
`

// fakeBlockFixture is a builder result that stops a group for its escalation.
const fakeBlockFixture = `{"type":"result","subtype":"success","is_error":false,"num_turns":1,` +
	`"session_id":"build-blocked","total_cost_usd":0,"usage":{"input_tokens":1,"output_tokens":1},` +
	`"structured_output":{"result":"BLOCKED","question":"stuck"}}` + "\n"

// fakeStopFixture is an escalation's answer that gives up on a group, leaving it blocked.
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
	// A real drain pins its own binary onto PATH for the whole process; restore it once the test ends.
	t.Setenv("PATH", os.Getenv("PATH"))
	root := t.TempDir()
	backlogtest.SeedText(t, root, backlogText)
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
	if err := os.WriteFile(filepath.Join(root, claude.Dir, claude.LineSettings), []byte("{}"), 0o644); err != nil {
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

// TestDrainRunsTwoLanesAtOnceWhenTheySharesNoFile proves two ready groups whose tasks touch no
// common file run their Drive calls concurrently, and both still ship.
func TestDrainRunsTwoLanesAtOnceWhenTheySharesNoFile(t *testing.T) {
	root := driveDrainRepo(t, driveDrainText)
	setupDrainDriveFakeClaude(t)
	overlap := t.TempDir()
	t.Setenv("FAKE_OVERLAP_DIR", overlap)
	saved := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(saved) })
	host, _ := mount.Get("claude")
	host.Probe = func() (mount.Usage, bool) { return mount.Usage{Plan: "max_20x"}, true }
	mount.Register(host)
	var out bytes.Buffer
	code, err := Launch(Options{Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out, PR: fakeForge(t, root)})
	if err != nil || code != 0 {
		t.Fatalf("code = %d, err = %v, out = %s", code, err, out.String())
	}
	if !strings.Contains(out.String(), "TG-07.1 shipped:") || !strings.Contains(out.String(), "TG-07.2 shipped:") {
		t.Fatalf("both groups should have shipped; out = %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(overlap, "overlapped")); err != nil {
		t.Fatalf("no lane overlapped; want two Drive calls running at once: %v", err)
	}
}

// TestDrainRunsTwoLanesSharingAFileInTurn proves two ready groups whose tasks share one file never
// run their Drive calls at once, so the second waits for the first to ship.
func TestDrainRunsTwoLanesSharingAFileInTurn(t *testing.T) {
	root := driveDrainRepo(t, driveDrainSharedFileText)
	setupDrainDriveFakeClaude(t)
	t.Setenv("FAKE_SHARED_FILE", "1")
	overlap := t.TempDir()
	t.Setenv("FAKE_OVERLAP_DIR", overlap)
	var out bytes.Buffer
	code, err := Launch(Options{Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out, PR: fakeForge(t, root)})
	if err != nil || code != 0 {
		t.Fatalf("code = %d, err = %v, out = %s", code, err, out.String())
	}
	if !strings.Contains(out.String(), "TG-07.1 shipped:") || !strings.Contains(out.String(), "TG-07.2 shipped:") {
		t.Fatalf("both groups should have shipped; out = %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(overlap, "overlapped")); err == nil {
		t.Fatal("groups sharing a file overlapped; want them run one after another")
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

// TestDrainFinishesEveryRunningLaneAfterADrainGroupsError proves a scheduling error mid-drain still
// drains each already-running lane to shipped or parked before the drain returns that error.
func TestDrainFinishesEveryRunningLaneAfterADrainGroupsError(t *testing.T) {
	root := driveDrainRepo(t, driveDrainText)
	setupDrainDriveFakeClaude(t)
	saved := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(saved) })
	host, _ := mount.Get("claude")
	host.Probe = func() (mount.Usage, bool) { return mount.Usage{Plan: "max_20x"}, true }
	mount.Register(host)
	waitFile := filepath.Join(t.TempDir(), "go")
	t.Setenv("FAKE_WAIT_GROUP", "TG-07.2")
	t.Setenv("FAKE_WAIT_FILE", waitFile)
	backlogDir := filepath.Join(root, "docs", "backlog")
	t.Cleanup(func() { _ = os.Chmod(backlogDir, 0o755) })

	type outcome struct {
		code int
		err  error
	}
	done := make(chan outcome, 1)
	var out bytes.Buffer
	client := fakeForge(t, root)
	go func() {
		code, err := Launch(Options{Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out, PR: client})
		done <- outcome{code, err}
	}()

	// The second lane waits on waitFile, so it is still running when the first lane's finish reconsiders scheduling.
	for i := 0; !shipped(root, "TG-07.1", time.Time{}); i++ {
		if i >= 300 {
			t.Fatal("TG-07.1 never shipped")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := os.Chmod(backlogDir, 0o000); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(waitFile, []byte("go\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := <-done
	if result.err == nil {
		t.Fatalf("drain = %v; a backlog it cannot read must be returned", result.err)
	}
	if result.code != 1 {
		t.Fatalf("code = %d, want 1; out = %s", result.code, out.String())
	}
	// The still-running lane's own Preparing reads the same corrupted backlog and parks, but the drain must print it.
	if !strings.Contains(out.String(), "TG-07.2 parked:") {
		t.Fatalf("out = %q; a lane still running when the scheduler failed must still reach parked, not be abandoned", out.String())
	}
}

func TestDrainDryRunListsTheGroupsInOrderAndLaunchesNothing(t *testing.T) {
	root := drainRepo(t)
	stacked := drainText + "\n### [TG-07.3] Third\n```yaml\ntype: feat\nversion: 1.2.0\nbase: feat/TG-07.2-second\n```\n\n" +
		"#### [TSK-07.3.1] Three [P: C] [READY]\n```yaml\nfiles: [c/three.go]\ndone_when: [\"true\"]\n```\n\n" +
		"### [TG-07.4] Fourth\n```yaml\ntype: feat\nversion: 1.3.0\nbase: feat/missing\n```\n\n" +
		"#### [TSK-07.4.1] Four [P: C] [READY]\n```yaml\nfiles: [d/four.go]\ndone_when: [\"true\"]\n```\n"
	if err := os.RemoveAll(filepath.Join(root, "docs", "backlog")); err != nil {
		t.Fatal(err)
	}
	backlogtest.SeedText(t, root, stacked)
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

// TestDrainSkipsAGroupThatComesUpAgainAfterItShipped proves a group still READY in the root's own
// backlog, since shipping never rewrites it, does not launch twice within the one drain.
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

// TestDrainPinsTheRunsBinaryOnPathForEveryLane proves a rebuild sync does before a drain starts
// lands on the run's own link, and every lane's builder session inherits it first on PATH.
func TestDrainPinsTheRunsBinaryOnPathForEveryLane(t *testing.T) {
	root := driveDrainRepo(t, driveDrainText)
	setupDrainDriveFakeClaude(t)
	toolkitCheckout(t, root, "stale-commit")
	fakeBuild(t)
	pathFile := filepath.Join(t.TempDir(), "path.txt")
	t.Setenv("FAKE_PATH_FILE", pathFile)
	var out bytes.Buffer
	if _, err := Launch(Options{Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out, PR: fakeForge(t, root)}); err != nil {
		t.Fatalf("launch failed: %v, out = %s", err, out.String())
	}
	link := filepath.Join(root, line.StateDir, "bin", "komodo")
	data, err := os.ReadFile(link)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "bin" {
		t.Fatalf("komodo on PATH = %q, want the rebuilt binary's own bytes", data)
	}
	seen, err := os.ReadFile(pathFile)
	if err != nil {
		t.Fatalf("no builder session recorded its PATH: %v", err)
	}
	first := strings.SplitN(strings.TrimSpace(string(seen)), string(os.PathListSeparator), 2)[0]
	if first != filepath.Dir(link) {
		t.Fatalf("a builder session's PATH started %q, want the run's own link dir %q", first, filepath.Dir(link))
	}
}

// TestAStaleBuildMarkerDuringADriveRunChangesNothingUntilItEnds proves a marker a lane's session
// leaves stale mid-drain rebuilds once, only after the whole drain ends, never moving a running lane.
func TestAStaleBuildMarkerDuringADriveRunChangesNothingUntilItEnds(t *testing.T) {
	root := driveDrainRepo(t, driveDrainText)
	setupDrainDriveFakeClaude(t)
	head := gitOut(t, root, "rev-parse", "HEAD")
	toolkitCheckout(t, root, head)
	builds, installs := fakeBuild(t)
	// The first group's builder leaves the build marker stale, as a session's own rebuild might mid-run.
	t.Setenv("FAKE_STALE_GROUP", "TG-07.1")
	t.Setenv("FAKE_ROOT", root)
	var out bytes.Buffer
	if _, err := Launch(Options{Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out, PR: fakeForge(t, root)}); err != nil {
		t.Fatalf("launch failed: %v, out = %s", err, out.String())
	}
	if *builds != 1 || *installs != 1 {
		t.Fatalf("builds = %d, installs = %d; a marker gone stale mid-run rebuilds once, only after the run ends", *builds, *installs)
	}
	recorded, err := os.ReadFile(filepath.Join(root, "bin", BuiltFrom))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(recorded)) != head {
		t.Fatalf("marker = %q, want %q; the run's own end-of-run sync must fix it", recorded, head)
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
	backlogtest.SeedText(t, root, restackText)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/.komodo/\n"), 0o644); err != nil {
		t.Fatal(err)
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

// noShipArgsFakeClaude plays a builder then a reviewer session, logging its argv to FAKE_ARGS_LOG.
const noShipArgsFakeClaude = `#!/bin/sh
echo "$@" >> "$FAKE_ARGS_LOG"
if [ "$1" != "-p" ]; then echo "2.0.0"; exit 0; fi
if [ "$KOMODO_ROLE" = "builder" ]; then
  echo built > one.txt
  cat "$FAKE_BUILD_FIXTURE"
else
  cat "$FAKE_REVIEW_FIXTURE"
fi
`

// TestDriveWithNoShipRunsNoRelayAndStopsBeforeShip proves a single-target --no-ship run drives the
// conductor directly, no relay prompt, and stops the group ready to ship, without ever calling gh.
func TestDriveWithNoShipRunsNoRelayAndStopsBeforeShip(t *testing.T) {
	root := driveDrainRepo(t, driveDrainText)
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	if err := os.WriteFile(script, []byte(noShipArgsFakeClaude), 0o755); err != nil {
		t.Fatal(err)
	}
	fixtures := t.TempDir()
	for name, text := range map[string]string{"build.jsonl": buildFixture, "review.jsonl": reviewFixture} {
		if err := os.WriteFile(filepath.Join(fixtures, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_BUILD_FIXTURE", filepath.Join(fixtures, "build.jsonl"))
	t.Setenv("FAKE_REVIEW_FIXTURE", filepath.Join(fixtures, "review.jsonl"))
	argsLog := filepath.Join(t.TempDir(), "args.log")
	t.Setenv("FAKE_ARGS_LOG", argsLog)
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh must not run with --no-ship: %v", args)
		return "", nil
	}}
	code, err := Launch(Options{Root: root, Target: "TG-07.1", NoShip: true, Budget: time.Minute, PR: client})
	if err != nil || code != 0 {
		t.Fatalf("code = %d, err = %v", code, err)
	}
	logged, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(logged), "/run TG-07.1") {
		t.Fatalf("a session received the run skill's relay prompt: %q", logged)
	}
	state, err := conductor.LoadState(conductor.StatePath(root, "TG-07.1"))
	if err != nil {
		t.Fatal(err)
	}
	if state.Current != conductor.Shipping {
		t.Fatalf("state = %s, want the group stopped ready to ship, at Shipping", state.Current)
	}
}

// TestDriveResumedWithNoShipNeverShipsFromShipping proves a group whose saved state is already at
// Shipping, not yet shipped, stays there and never calls gh when a --no-ship run resumes it.
func TestDriveResumedWithNoShipNeverShipsFromShipping(t *testing.T) {
	root := driveDrainRepo(t, driveDrainText)
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	if err := os.WriteFile(script, []byte(noShipArgsFakeClaude), 0o755); err != nil {
		t.Fatal(err)
	}
	fixtures := t.TempDir()
	for name, text := range map[string]string{"build.jsonl": buildFixture, "review.jsonl": reviewFixture} {
		if err := os.WriteFile(filepath.Join(fixtures, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_BUILD_FIXTURE", filepath.Join(fixtures, "build.jsonl"))
	t.Setenv("FAKE_REVIEW_FIXTURE", filepath.Join(fixtures, "review.jsonl"))
	t.Setenv("FAKE_ARGS_LOG", filepath.Join(t.TempDir(), "args.log"))
	refusing := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh must not run on a --no-ship drive, resumed or not: %v", args)
		return "", nil
	}}
	if code, err := Launch(Options{Root: root, Target: "TG-07.1", NoShip: true, Budget: time.Minute, PR: refusing}); err != nil || code != 0 {
		t.Fatalf("first pass: code = %d, err = %v", code, err)
	}
	statePath := conductor.StatePath(root, "TG-07.1")
	before, err := conductor.LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if before.Current != conductor.Shipping || before.ShipDone {
		t.Fatalf("state = %+v, want it saved at Shipping with ShipDone false", before)
	}
	if code, err := Launch(Options{Root: root, Target: "TG-07.1", NoShip: true, Budget: time.Minute, PR: refusing}); err != nil || code != 0 {
		t.Fatalf("resumed pass: code = %d, err = %v", code, err)
	}
	after, err := conductor.LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if after.Current != conductor.Shipping {
		t.Fatalf("state = %s after a resumed --no-ship run, want it to stay at Shipping", after.Current)
	}
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
