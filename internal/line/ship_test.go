package line

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/git"
	"komodo/internal/pr"
)

const shipBacklog = "### [TG-09.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-09.1.1] Do it [P: C] [DONE]\n```yaml\nfiles: [a/one.go]\ndone_when:\n  - true\n```\n"

// shipRepo builds a group worktree and a main checkout, both remoted at a bare origin, so
// ShipGroup can read the push URL from the root and push from the group.
func shipRepo(t *testing.T) (root, group string) {
	t.Helper()
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(shipBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	runGit(t, root, "init")
	runGit(t, root, "remote", "add", "origin", bare)
	group = filepath.Join(root, "group")
	if err := os.MkdirAll(group, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "init")
	runGit(t, group, "config", "user.email", "a@example.com")
	runGit(t, group, "config", "user.name", "a")
	runGit(t, group, "remote", "add", "origin", bare)
	runGit(t, group, "checkout", "-b", "feat/a-group")
	if err := os.WriteFile(filepath.Join(group, "one.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(group, "BACKLOG.md"), []byte(shipBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "add", "-A")
	runGit(t, group, "commit", "-m", "seed")
	return root, group
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

// commitDated writes a file and commits it with an explicit author and committer date.
func commitDated(t *testing.T, root, name, body, message string, when time.Time) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	stamp := when.Format(time.RFC3339)
	cmd := exec.Command("git", "commit", "-m", message, "--date", stamp)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_COMMITTER_DATE="+stamp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
}

func TestShipGroupRunsTheAfterPublishCommand(t *testing.T) {
	root, group := shipRepo(t)
	if err := os.MkdirAll(filepath.Join(group, StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	commands := `{"after_publish":"touch published.txt"}`
	if err := os.WriteFile(filepath.Join(group, StateDir, "commands.json"), []byte(commands), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	result, err := ShipGroup(root, plan, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Published == nil || !result.Published.OK() {
		t.Fatalf("published = %+v", result.Published)
	}
	if _, err := os.Stat(filepath.Join(group, "published.txt")); err != nil {
		t.Fatalf("after_publish did not run: %v", err)
	}
}

func TestShipGroupPushesThroughTheRootsExplicitURL(t *testing.T) {
	root, group := shipRepo(t)
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	if _, err := ShipGroup(root, plan, nil, nil); err != nil {
		t.Fatal(err)
	}
	bare, err := git.Run(root, "remote", "get-url", "--push", "origin")
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", bare, "branch", "--list", "feat/a-group").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "feat/a-group") {
		t.Fatalf("branch = %s; ShipGroup must push the branch to the URL the root names", out)
	}
	if _, err := git.Run(group, "config", "--get", "remote.origin.pushurl"); err == nil {
		t.Fatal("the group worktree's own config must never gain a pushurl from a ship push")
	}
}

func TestShipGroupWithNoAfterPublishLeavesPublishedUnset(t *testing.T) {
	root, _ := shipRepo(t)
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	result, err := ShipGroup(root, plan, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Published != nil {
		t.Fatalf("published = %+v, want nil", result.Published)
	}
}

func TestAfterPublishFailureFailsTheShip(t *testing.T) {
	root, group := shipRepo(t)
	if err := os.MkdirAll(filepath.Join(group, StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	commands := `{"after_publish":"exit 3"}`
	if err := os.WriteFile(filepath.Join(group, StateDir, "commands.json"), []byte(commands), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	result, err := ShipGroup(root, plan, nil, nil)
	if err == nil {
		t.Fatal("a failed after_publish command must fail the ship, not exit clean")
	}
	if result == nil || result.Published == nil || result.Published.ExitCode != 3 {
		t.Fatalf("published = %+v", result.Published)
	}
}

func TestShipRefusesAPlanWhosePauseBlankedItsWaves(t *testing.T) {
	root, _ := shipRepo(t)
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks:     []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
		WaitUntil: time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "paused") {
		t.Fatalf("err = %v; a plan whose waves a pause blanked must not ship", err)
	}
}

func TestShipWritesAHandoffInsteadOfPushingWhenScrubbed(t *testing.T) {
	root, group := shipRepo(t)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_CONFIG_KEY_0", "credential.helper")
	t.Setenv("GIT_CONFIG_VALUE_0", "")
	client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh must not run while the environment is scrubbed: %v", args)
		return "", nil
	}}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	result, err := ShipGroup(root, plan, nil, client)
	if err != nil {
		t.Fatal(err)
	}
	if result.URL != "" {
		t.Fatalf("url = %q; a scrubbed ship never reaches the pull request client", result.URL)
	}
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	cmd.Dir = group
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("upstream = %s; a scrubbed ship must not push", out)
	}
	data, err := os.ReadFile(filepath.Join(root, StateDir, "ship.json"))
	if err != nil {
		t.Fatal(err)
	}
	var handoff ShipHandoff
	if err := json.Unmarshal(data, &handoff); err != nil {
		t.Fatal(err)
	}
	if handoff.Branch != "feat/a-group" || handoff.Base != "main" || handoff.Title == "" {
		t.Fatalf("handoff = %+v", handoff)
	}
}

const flipBacklog = "### [TG-11.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-11.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"

func TestShipFlipsTheStatusOnTheBranchItPushes(t *testing.T) {
	worktree := gitRepo(t)
	commit(t, worktree, "BACKLOG.md", flipBacklog, "the backlog")
	root := t.TempDir()
	closed := strings.Replace(flipBacklog, "[READY]", "[DONE]", 1)
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(closed), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
	}
	unreachableOrigin(t, root)
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "git push to origin feat/a-group") {
		t.Fatalf("err = %v; the origin is unreachable, so ship must fail at the push itself and not before", err)
	}
	shipped, err := os.ReadFile(filepath.Join(worktree, "BACKLOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(shipped), "[TSK-11.1.1] One [P: C] [DONE]") {
		t.Fatalf("the branch being pushed still says READY:\n%s", shipped)
	}
	stale, err := os.ReadFile(filepath.Join(root, "BACKLOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(stale) != closed {
		t.Fatal("ship must write the worktree it commits, not the root it was invoked from")
	}
}

func TestShipNeverMarksATaskItSkippedAsDone(t *testing.T) {
	worktree := gitRepo(t)
	commit(t, worktree, "BACKLOG.md", flipBacklog, "the backlog")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(flipBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
	}
	_, _ = ShipGroup(root, plan, nil, nil)
	shipped, err := os.ReadFile(filepath.Join(worktree, "BACKLOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(shipped), "[DONE]") {
		t.Fatalf("a task close never marked DONE shipped as DONE:\n%s", shipped)
	}
}

func TestFileFindingsOnlyAfterASuccessfulPush(t *testing.T) {
	worktree := gitRepo(t)
	commit(t, worktree, "BACKLOG.md", flipBacklog, "the backlog")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(flipBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
	review := ResultPath(root, "TG-11.1-review")
	if err := os.MkdirAll(filepath.Dir(review), 0o755); err != nil {
		t.Fatal(err)
	}
	findings := `{"findings":[{"class":"simplify","severity":"low","file":"a/one.go","line":1,"title":"t","detail":"d","fix":"f"}]}`
	if err := os.WriteFile(review, []byte(findings), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "main", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
	}
	plan.Profile.SeverityFloor = "high"
	unreachableOrigin(t, root)
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "git push to origin main") {
		t.Fatalf("err = %v; the origin is unreachable, so ship must fail at the push itself", err)
	}
	before, err := os.ReadFile(filepath.Join(worktree, "BACKLOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(before), "#### [") != 1 {
		t.Fatalf("a failed push must not file a finding:\n%s", before)
	}
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	runGit(t, root, "remote", "set-url", "origin", bare)
	result, err := ShipGroup(root, plan, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Filed) != 1 {
		t.Fatalf("filed = %v, want exactly one finding filed once the push succeeds", result.Filed)
	}
}

func TestAFailedGateRecordsShipAsFailedNotDone(t *testing.T) {
	root, _ := shipRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "cmd", "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "komodo", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	state := RunState{Run: "TG-09.1-1", Group: "TG-09.1", Base: "main", Branch: "feat/a-group"}
	if err := SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "gate") {
		t.Fatalf("err = %v; the group worktree has no cmd/komodo, so the gate must fail", err)
	}
	entries, err := Book(root).All()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Station == "ship" && entry.Group == "TG-09.1" && entry.Outcome == "done" {
			t.Fatal("a failed gate must not stamp the ship entry as done")
		}
	}
	if shipped(root, plan, backlog.Backlog{}) {
		t.Fatal("a failed gate must not release the line to the next group")
	}
}

func TestShipCommitsTheStatusItWrote(t *testing.T) {
	worktree := gitRepo(t)
	commit(t, worktree, "BACKLOG.md", flipBacklog, "the backlog")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(flipBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
	}
	_, _ = ShipGroup(root, plan, nil, nil)
	status, err := git.Run(worktree, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(status) != "" {
		t.Fatalf("status = %q; the status flip and the changelog must land in the commit, not after it", status)
	}
}

const closedRootBacklog = "### [TG-12.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-12.1.1] One [P: C] [BLOCKED]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"

const staleWorktreeBacklog = "### [TG-12.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-12.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```\n"

func TestShipReadsStatusFromTheRootCloseWrites(t *testing.T) {
	worktree := gitRepo(t)
	commit(t, worktree, "BACKLOG.md", staleWorktreeBacklog, "the backlog")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(closedRootBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	runGit(t, root, "init")
	runGit(t, root, "remote", "add", "origin", bare)
	plan := &Plan{
		Group: "TG-12.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "main", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-12.1.1", Title: "One", Status: "READY"}},
	}
	result, err := ShipGroup(root, plan, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Blocked) != 1 || result.Blocked[0] != "TSK-12.1.1" {
		t.Fatalf("blocked = %v; ship must read status from the root close writes, not the stale worktree copy", result.Blocked)
	}
	if !result.Draft {
		t.Fatal("a blocked task must ship as a draft")
	}
}

func TestShipRendersTheBodyAfterBlockedIsKnown(t *testing.T) {
	worktree := gitRepo(t)
	commit(t, worktree, "BACKLOG.md", staleWorktreeBacklog, "the backlog")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(closedRootBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	runGit(t, root, "init")
	runGit(t, root, "remote", "add", "origin", bare)
	plan := &Plan{
		Group: "TG-12.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "main", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-12.1.1", Title: "One", Status: "READY"}},
	}
	var calls []string
	client := &pr.Client{Dir: worktree, Run: func(_ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "https://example.com/pull/1", nil
	}}
	if _, err := ShipGroup(root, plan, nil, client); err != nil {
		t.Fatal(err)
	}
	if len(calls) == 0 || !strings.Contains(calls[0], "[ ] **TSK-12.1.1**") {
		t.Fatalf("calls = %v; the body must render after Blocked is known, so a blocked task ships unticked", calls)
	}
}

func TestShipsOwnCommitDoesNotRestaleTheReview(t *testing.T) {
	worktree := gitRepo(t)
	now := time.Now()
	commitDated(t, worktree, "a/one.go", "package a\n", "seed", now.Add(-2*time.Hour))
	root := t.TempDir()
	plan := &Plan{Group: "TG-13.1", Title: "A group", Type: "feat", Worktree: worktree}
	review := ResultPath(root, "TG-13.1-review")
	if err := os.MkdirAll(filepath.Dir(review), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(review, []byte(`{"findings":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	reviewTime := now.Add(-time.Hour)
	if err := os.Chtimes(review, reviewTime, reviewTime); err != nil {
		t.Fatal(err)
	}
	message := fmt.Sprintf("%s: %s (%s)", plan.Type, plan.Title, plan.Group)
	commitDated(t, worktree, "CHANGELOG.md", "# Changelog\n", message, now)
	if !reviewed(root, plan) {
		t.Fatal("ship's own commit must not restale a review that already passed it")
	}
}

// TestShipGroupPushesFromAWorktreeThatRefusesPush cuts the group the way the line does, with its
// refused pushurl, and proves ship still lands the branch on origin.
func TestShipGroupPushesFromAWorktreeThatRefusesPush(t *testing.T) {
	root := t.TempDir()
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	runGit(t, root, "remote", "add", "origin", bare)
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(shipBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "seed")
	runGit(t, root, "push", "origin", "main")
	runGit(t, root, "fetch", "origin")
	worktree := filepath.Join(root, StateDir, "wt", "TG-09.1")
	if err := AddWorktree(root, "feat/a-group", "main", worktree); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", worktree, "push", "origin", "feat/a-group").CombinedOutput(); err == nil {
		t.Fatalf("the cut worktree must refuse a plain push, out = %s", out)
	}
	if err := os.WriteFile(filepath.Join(worktree, "one.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, worktree, "add", "-A")
	runGit(t, worktree, "commit", "-m", "work")
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group",
		Worktree: filepath.Join(StateDir, "wt", "TG-09.1"),
		Tasks:    []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	if _, err := ShipGroup(root, plan, nil, nil); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", bare, "branch", "--list", "feat/a-group").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "feat/a-group") {
		t.Fatalf("branch = %s, err = %v; ship must land the branch on origin", out, err)
	}
	if merge, err := git.Run(worktree, "config", "--get", "branch.feat/a-group.merge"); err != nil || merge != "refs/heads/feat/a-group" {
		t.Fatalf("upstream merge = %q, err = %v; ship must set the branch's upstream the way push -u does", merge, err)
	}
	if refused, err := git.Run(worktree, "config", "--get", "remote.origin.pushurl"); err != nil || refused != RefusedPushURL {
		t.Fatalf("pushurl = %q; ship must leave the worktree's refusal in place", refused)
	}
}

// unreachableOrigin makes root a repo whose origin names a path that does not exist, so a ship
// resolves the URL and then fails at the push itself.
func unreachableOrigin(t *testing.T, root string) {
	t.Helper()
	runGit(t, root, "init")
	runGit(t, root, "remote", "add", "origin", filepath.Join(t.TempDir(), "missing.git"))
}

// TestPushErrorsNeverCarryACredential checks a failed push redacts a token the origin URL holds.
func TestPushErrorsNeverCarryACredential(t *testing.T) {
	text := redactURL("fatal: unable to access 'https://x-access-token:SECRET@github.com/o/r.git/': 403", "https://x-access-token:SECRET@github.com/o/r.git")
	if strings.Contains(text, "SECRET") {
		t.Fatalf("text = %q; a push error must never carry a token", text)
	}
	if other := redactURL("remote: https://u:p@example.com/x denied", ""); strings.Contains(other, "u:p") {
		t.Fatalf("other = %q; any URL credential must be redacted", other)
	}
}
