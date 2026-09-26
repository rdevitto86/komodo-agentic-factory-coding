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
	"komodo/internal/changelog"
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
	saveReview(t, root, "TG-09.1", `{"findings":[]}`)
	return root, group
}

// saveReview saves a review result for the group, as the reviewer would after the last commit.
func saveReview(t *testing.T, root, groupID, result string) {
	t.Helper()
	review := ResultPath(root, groupID+"-review")
	if err := os.MkdirAll(filepath.Dir(review), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(review, []byte(result), 0o644); err != nil {
		t.Fatal(err)
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
	if err := os.MkdirAll(filepath.Join(root, StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	commands := `{"after_publish":"touch published.txt"}`
	if err := os.WriteFile(filepath.Join(root, StateDir, "commands.json"), []byte(commands), 0o644); err != nil {
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
	root, _ := shipRepo(t)
	if err := os.MkdirAll(filepath.Join(root, StateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	commands := `{"after_publish":"exit 3"}`
	if err := os.WriteFile(filepath.Join(root, StateDir, "commands.json"), []byte(commands), 0o644); err != nil {
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
	data, err := os.ReadFile(filepath.Join(root, StateDir, "runs", "TG-09.1", "ship.json"))
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
	saveReview(t, root, "TG-11.1", `{"findings":[]}`)
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

func TestShipRefusesAGroupWithNoReviewResult(t *testing.T) {
	root, _ := shipRepo(t)
	if err := os.Remove(ResultPath(root, "TG-09.1-review")); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "no review result") {
		t.Fatalf("err = %v; a group with no review result must not ship", err)
	}
}

func TestShipRefusesAReviewOlderThanTheBranch(t *testing.T) {
	root, group := shipRepo(t)
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(ResultPath(root, "TG-09.1-review"), past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(group, "two.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "add", "-A")
	runGit(t, group, "commit", "-m", "a repair after the review")
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "changed after its review") {
		t.Fatalf("err = %v; a branch that moved after its review must not ship", err)
	}
}

func TestFileFindingsBeforePushAndCommit(t *testing.T) {
	root, group := shipRepo(t)
	review := ResultPath(root, "TG-09.1-review")
	if err := os.MkdirAll(filepath.Dir(review), 0o755); err != nil {
		t.Fatal(err)
	}
	findings := `{"findings":[{"class":"simplify","severity":"low","file":"a/one.go","line":1,"title":"t","detail":"d","fix":"f"}]}`
	if err := os.WriteFile(review, []byte(findings), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	plan.Profile.SeverityFloor = "high"
	result, err := ShipGroup(root, plan, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Filed) != 1 {
		t.Fatalf("filed = %v, want exactly one finding filed", result.Filed)
	}
	status, err := git.Run(group, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(status) != "" {
		t.Fatalf("status = %q; findings filed before push must be in the commit, leaving worktree clean", status)
	}
}

func TestShipRetriedAroundAFailedPushDoesNotFileAFindingTwice(t *testing.T) {
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
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
	}
	plan.Profile.SeverityFloor = "high"
	unreachableOrigin(t, root)
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "git push to origin feat/a-group") {
		t.Fatalf("err = %v; the origin is unreachable, so the first ship must fail at the push", err)
	}
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "git push to origin feat/a-group") {
		t.Fatalf("err = %v; a retried ship must fail the same way", err)
	}
	shipped, err := os.ReadFile(filepath.Join(worktree, "BACKLOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(shipped), "a/one.go:1 t") != 1 {
		t.Fatalf("a retry must not file the same finding a second time:\n%s", shipped)
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

func TestShipWritesTheRunsStatusIntoItsCommitAndClearsIt(t *testing.T) {
	for _, status := range []string{"DONE", "BLOCKED"} {
		worktree := gitRepo(t)
		commit(t, worktree, "BACKLOG.md", flipBacklog, "the backlog")
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(flipBacklog), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := RecordStatus(root, "TSK-11.1.1", status); err != nil {
			t.Fatal(err)
		}
		plan := &Plan{
			Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
			Base: "main", Branch: "feat/a-group", Worktree: worktree,
			Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
		}
		unreachableOrigin(t, root)
		saveReview(t, root, "TG-11.1", `{"findings":[]}`)
		_, _ = ShipGroup(root, plan, nil, nil)
		committed, err := git.Run(worktree, "show", "HEAD:BACKLOG.md")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(committed, "[TSK-11.1.1] One [P: C] ["+status+"]") {
			t.Fatalf("the ship commit does not carry %s:\n%s", status, committed)
		}
		if len(LoadStatus(root)) != 0 {
			t.Fatalf("%s: status.json survived the ship commit", status)
		}
		if data, _ := os.ReadFile(filepath.Join(root, "BACKLOG.md")); string(data) != flipBacklog {
			t.Fatalf("%s: ship rewrote the root's BACKLOG.md", status)
		}
	}
}

func TestShipLeavesAnotherGroupsLiveStatusAlone(t *testing.T) {
	text := flipBacklog + "\n### [TG-11.2] Another group\n```yaml\ntype: feat\nversion: 2.1.0\n```\n\n" +
		"#### [TSK-11.2.1] Other [P: C] [READY]\n```yaml\nfiles: [b/other.go]\ndone_when: [\"go test ./b/...\"]\n```\n"
	worktree := gitRepo(t)
	commit(t, worktree, "BACKLOG.md", text, "the backlog")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, taskID := range []string{"TSK-11.1.1", "TSK-11.2.1"} {
		if err := RecordStatus(root, taskID, "DONE"); err != nil {
			t.Fatal(err)
		}
	}
	plan := &Plan{
		Group: "TG-11.1", Title: "A group", Type: "feat", Version: "2.0.0",
		Base: "main", Branch: "feat/a-group", Worktree: worktree,
		Tasks: []PlanTask{{ID: "TSK-11.1.1", Title: "One", Status: "READY"}},
	}
	unreachableOrigin(t, root)
	saveReview(t, root, "TG-11.1", `{"findings":[]}`)
	_, _ = ShipGroup(root, plan, nil, nil)
	committed, err := git.Run(worktree, "show", "HEAD:BACKLOG.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(committed, "[TSK-11.1.1] One [P: C] [DONE]") {
		t.Fatalf("the ship commit does not carry its own task's DONE:\n%s", committed)
	}
	if !strings.Contains(committed, "[TSK-11.2.1] Other [P: C] [READY]") {
		t.Fatalf("the ship commit wrote another group's status:\n%s", committed)
	}
	if got := LoadStatus(root); len(got) != 1 || got["TSK-11.2.1"].Status != "DONE" {
		t.Fatalf("status = %+v; ship must clear only its own group's tasks", got)
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
	saveReview(t, root, "TG-12.1", `{"findings":[]}`)
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
	saveReview(t, root, "TG-12.1", `{"findings":[]}`)
	if _, err := ShipGroup(root, plan, nil, client); err != nil {
		t.Fatal(err)
	}
	if len(calls) == 0 || !strings.Contains(calls[0], "- **Unproven** TSK-12.1.1 One is blocked") {
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
	saveReview(t, root, "TG-09.1", `{"findings":[]}`)
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

// shipWithBase ships the shipRepo group against base and returns the gh pr create call and result.
func shipWithBase(t *testing.T, base string, pushBase bool) (string, *ShipResult) {
	t.Helper()
	root, group := shipRepo(t)
	runGit(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
	if pushBase {
		runGit(t, group, "push", "origin", "HEAD:refs/heads/"+base)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: base, Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	var create string
	client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
		if len(args) > 1 && args[0] == "pr" && args[1] == "create" {
			create = strings.Join(args, "\x00")
		}
		return "https://example.com/pull/1", nil
	}}
	result, err := ShipGroup(root, plan, nil, client)
	if err != nil {
		t.Fatal(err)
	}
	return create, result
}

func TestShipKeepsABaseOriginStillHas(t *testing.T) {
	create, result := shipWithBase(t, "feat/stack", true)
	if result.Base != "feat/stack" || result.StaleBase != "" {
		t.Fatalf("base = %q, stale = %q; a present base is kept", result.Base, result.StaleBase)
	}
	if !strings.Contains(create, "--base\x00feat/stack\x00") || strings.Contains(create, "is gone from origin") {
		t.Fatalf("create = %q", create)
	}
}

func TestShipTargetsTheDefaultBranchWhenTheBaseIsDeleted(t *testing.T) {
	create, result := shipWithBase(t, "feat/gone", false)
	if result.Base != "trunk" || result.StaleBase != "feat/gone" {
		t.Fatalf("base = %q, stale = %q; a deleted base becomes the default branch", result.Base, result.StaleBase)
	}
	if !strings.Contains(create, "--base\x00trunk\x00") {
		t.Fatalf("create = %q; the pull request must target the default branch", create)
	}
	if !strings.Contains(create, "Base feat/gone is gone from origin, so this targets trunk.") {
		t.Fatalf("create = %q; the body must name the switch", create)
	}
}

// twoTaskPlan is a stacked two-task group whose second task declared two files.
func twoTaskPlan() *Plan {
	return &Plan{
		Group: "TG-09.1", Title: "A group", Base: "feat/stack",
		Tasks: []PlanTask{
			{ID: "TSK-09.1.1", Title: "Do one", Files: []string{"a/one.go"}},
			{ID: "TSK-09.1.2", Title: "Do two", Files: []string{"a/two.go", "a/two_test.go"}},
		},
	}
}

func TestReportBodyRendersTheFourSectionsInOrder(t *testing.T) {
	plan := twoTaskPlan()
	result := &ShipResult{Base: "feat/stack", Done: []string{"TSK-09.1.1", "TSK-09.1.2"}}
	waves := []*WaveResult{{Wave: 1, Gates: []CommandResult{{Command: "go vet ./..."}}, Verify: &CommandResult{Command: "go test ./...", ExitCode: 1}}}
	context := BodyContext{Why: "The line needs it.", DefaultBase: "main", BlastRadius: "low", BlastRadiusWhy: "one package"}
	body := ReportBody(plan, result, waves, context)
	last := -1
	for _, heading := range []string{"## Summary", "## Changes", "## Validation", "## Dependencies"} {
		at := strings.Index(body, heading)
		if at <= last {
			t.Fatalf("%s is missing or out of order:\n%s", heading, body)
		}
		last = at
	}
	for _, want := range []string{
		"The line needs it.",
		"- **TSK-09.1.1** — Do one (`a/one.go`)",
		"- **TSK-09.1.2** — Do two (`a/two.go`, `a/two_test.go`)",
		"- `go vet ./...` passed", "- `go test ./...` exited 1", "- **Blast radius** low: one package",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %q:\n%s", want, body)
		}
	}
	for _, banned := range []string{"Co-authored-by", "Generated", "session"} {
		if strings.Contains(body, banned) {
			t.Errorf("body carries %q:\n%s", banned, body)
		}
	}
}

func TestReportBodyNamesAStackedBaseUnderDependencies(t *testing.T) {
	body := ReportBody(twoTaskPlan(), &ShipResult{Base: "feat/stack"}, nil, BodyContext{DefaultBase: "main"})
	deps := body[strings.Index(body, "## Dependencies"):]
	if !strings.Contains(deps, "`feat/stack`") {
		t.Fatalf("dependencies do not name the base:\n%s", body)
	}
	plain := ReportBody(twoTaskPlan(), &ShipResult{Base: "main"}, nil, BodyContext{DefaultBase: "main"})
	if strings.Contains(plain, "## Dependencies") {
		t.Fatalf("a group on the default branch has no dependencies:\n%s", plain)
	}
}

func TestReportBodyKeepsABlockedTaskUnderValidation(t *testing.T) {
	body := ReportBody(twoTaskPlan(), &ShipResult{Blocked: []string{"TSK-09.1.2"}}, nil, BodyContext{})
	validation := body[strings.Index(body, "## Validation"):]
	if !strings.Contains(validation, "TSK-09.1.2 Do two is blocked") {
		t.Fatalf("the blocked task is not under Validation:\n%s", body)
	}
}

func TestTemplateSectionsFollowTheRepoTemplate(t *testing.T) {
	dir := t.TempDir()
	if got := templateSections(dir); strings.Join(got, ",") != "Summary,Changes,Validation,Dependencies" {
		t.Fatalf("no template: sections = %v", got)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".github"), 0o755); err != nil {
		t.Fatal(err)
	}
	template := "<!-- note -->\n\n## Validation\n\n## Summary\n\n## Notes\n\n## Changes\n"
	if err := os.WriteFile(filepath.Join(dir, templatePath), []byte(template), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := templateSections(dir); strings.Join(got, ",") != "Validation,Summary,Changes,Dependencies" {
		t.Fatalf("template: sections = %v", got)
	}
}

func TestGroupWhyReadsOnlyItsOwnGroup(t *testing.T) {
	text := "### [TG-01.1] One\n* **Why:** first reason\n\n### [TG-01.2] Two\n* **Why:** second reason\n"
	if got := groupWhy(text, "TG-01.2"); got != "second reason" {
		t.Fatalf("why = %q", got)
	}
}

func TestShipStampsTheChangedLinesOnItsRow(t *testing.T) {
	root, group := shipRepo(t)
	runGit(t, group, "branch", "main")
	if err := os.WriteFile(filepath.Join(group, "one.go"), []byte("package b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(group, "two.go"), []byte("package b\n\nvar x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "add", "-A")
	runGit(t, group, "commit", "-m", "work")
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	if err := SaveRun(root, RunState{Run: "TG-09.1-1", Group: "TG-09.1", Base: "main", Branch: "feat/a-group"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ShipGroup(root, plan, nil, nil); err != nil {
		t.Fatal(err)
	}
	entries, err := Book(root).All()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Station == "ship" {
			if entry.Lines != 5 {
				t.Fatalf("lines = %d; one line replaced and three added is five changed", entry.Lines)
			}
			return
		}
	}
	t.Fatal("ship stamped no row")
}

func TestChangedLinesIsZeroWhenTheBaseIsUnknown(t *testing.T) {
	_, group := shipRepo(t)
	if got := ChangedLines(group, "no-such-base", "feat/a-group"); got != 0 {
		t.Fatalf("lines = %d; an unknown base is no count, never a guess", got)
	}
}

func TestScopeLabelRules(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  string
	}{
		{"guard", []string{"internal/guard/guard.go"}, "scope/guard"},
		{"mount", []string{"internal/mount/mount.go"}, "scope/mount"},
		{"skills", []string{"komodo/skills/build.md"}, "scope/skills"},
		{"roles", []string{"komodo/roles/build.md"}, "scope/skills"},
		{"agents", []string{"internal/profile/tier.go"}, "scope/agents"},
		{"machine", []string{"internal/profile/machine.go"}, "scope/agents"},
		{"harness", []string{"internal/line/ship.go"}, "scope/harness"},
		{"profile without tier or machine", []string{"internal/profile/profile.go"}, "scope/harness"},
		{"none", nil, "scope/harness"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := scopeLabel(c.files); got != c.want {
				t.Fatalf("scopeLabel(%v) = %q, want %q", c.files, got, c.want)
			}
		})
	}
}

func TestShipWarnsWhenTheRepoLacksAWantedLabel(t *testing.T) {
	root, group := shipRepo(t)
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "label" {
			return `[{"name":"@agent 🤖"}]`, nil
		}
		return "https://example.com/pull/1", nil
	}}
	result, err := ShipGroup(root, plan, nil, client)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Labels) != 1 || result.Labels[0] != "@agent 🤖" {
		t.Fatalf("labels = %v", result.Labels)
	}
	found := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "scope/harness") {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %v; a wanted label the repo lacks must be a warning", result.Warnings)
	}
}

func TestShipRefusesWhenTaskIsRefinedOnlyAtRoot(t *testing.T) {
	root, _ := shipRepo(t)
	refined := "### [TG-09.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
		"#### [TSK-09.1.1] Do it [P: C] [DONE]\n```yaml\nfiles: [a/one.go, a/two.go]\ndone_when:\n  - true\n  - echo more\ncontext: [docs/guide.md]\n```\n"
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(refined), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it"}},
	}
	if _, err := ShipGroup(root, plan, nil, nil); err == nil || !strings.Contains(err.Error(), "TSK-09.1.1") {
		t.Fatalf("err = %v; ship must refuse when a task differs between root and worktree, naming the task", err)
	}
}

func TestShipWritesAChangelogFragmentAndLeavesTheChangelogAlone(t *testing.T) {
	worktree := gitRepo(t)
	commit(t, worktree, "BACKLOG.md", staleWorktreeBacklog, "the backlog")
	commit(t, worktree, "CHANGELOG.md", "# Changelog\n", "the changelog")
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
	client := &pr.Client{Dir: worktree, Run: func(string, ...string) (string, error) {
		return "https://example.com/pull/1", nil
	}}
	saveReview(t, root, "TG-12.1", `{"findings":[]}`)
	if _, err := ShipGroup(root, plan, nil, client); err != nil {
		t.Fatal(err)
	}
	fragment, err := os.ReadFile(changelog.FragmentPath(worktree, "2.0.0", "TG-12.1"))
	if err != nil || !strings.HasPrefix(string(fragment), "- **TG-12.1** A group") {
		t.Fatalf("fragment = %q, %v", fragment, err)
	}
	if data, _ := os.ReadFile(filepath.Join(worktree, "CHANGELOG.md")); string(data) != "# Changelog\n" {
		t.Fatalf("ship edited CHANGELOG.md:\n%s", data)
	}
	if tracked, _ := git.Run(worktree, "ls-files", changelog.Dir); tracked == "" {
		t.Fatal("the fragment is not in the ship commit")
	}
}

func TestCatchUpRebasesOntoAMovedBaseKeepingEdits(t *testing.T) {
	_, group := shipRepo(t)
	runGit(t, group, "branch", "main", "HEAD~0")
	runGit(t, group, "checkout", "-q", "main")
	commitDated(t, group, "two.go", "package a\n", "base moved", time.Now())
	runGit(t, group, "checkout", "-q", "feat/a-group")
	commitDated(t, group, "three.go", "package a\n", "group work", time.Now())
	if err := os.WriteFile(filepath.Join(group, "CHANGELOG.md"), []byte("# Changelog\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, group, "add", "CHANGELOG.md")
	if err := catchUp(group, "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(group, "two.go")); err != nil {
		t.Fatal("the base's commit is not under the group branch")
	}
	if _, err := os.Stat(filepath.Join(group, "CHANGELOG.md")); err != nil {
		t.Fatal("the uncommitted ship edit was lost")
	}
}

func TestCatchUpStopsOnAConflictNamingTheFile(t *testing.T) {
	_, group := shipRepo(t)
	runGit(t, group, "branch", "main", "HEAD~0")
	runGit(t, group, "checkout", "-q", "main")
	commitDated(t, group, "one.go", "package base\n", "base edits one.go", time.Now())
	runGit(t, group, "checkout", "-q", "feat/a-group")
	commitDated(t, group, "one.go", "package group\n", "group edits one.go", time.Now())
	err := catchUp(group, "main")
	if err == nil || !strings.Contains(err.Error(), "one.go") {
		t.Fatalf("want a conflict naming one.go, got %v", err)
	}
	if status, _ := exec.Command("git", "-C", group, "status", "--porcelain").Output(); strings.Contains(string(status), "UU") {
		t.Fatal("the rebase was left half done")
	}
}
