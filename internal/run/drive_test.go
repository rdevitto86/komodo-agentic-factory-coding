package run

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"komodo/internal/backlog/backlogtest"
	"komodo/internal/conductor"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/mount/claude"
	"komodo/internal/pr"
	"komodo/internal/review"
)

const driveBacklog = "### [TG-40.1] A fake group\n```yaml\ntype: feat\nversion: 1.0.0\nmode: single\n```\n\n" +
	"#### [TSK-40.1.1] Do the work [P: C] [READY]\n```yaml\nfiles: [change.txt]\ndone_when:\n  - true\n```\n"

// driveFakeClaude leaves a fake change uncommitted, as a real builder does, and replays the builder result, then the reviewer result.
const driveFakeClaude = `#!/bin/sh
if [ -n "$FAKE_HANG_MARKER" ]; then
  touch "$FAKE_HANG_MARKER"
  sleep "$FAKE_HANG_SLEEP"
fi
n=0
if [ -f "$FAKE_COUNTER" ]; then n=$(cat "$FAKE_COUNTER"); fi
n=$((n+1))
echo "$n" > "$FAKE_COUNTER"
if [ "$n" = "1" ] && [ -n "$FAKE_LAND_BRANCH" ]; then
  git -C "$FAKE_LAND_REPO" push --quiet origin "$FAKE_LAND_BRANCH":main
fi
if [ "$n" = "1" ]; then
  echo "built" > change.txt
  cat "$FAKE_BUILD_FIXTURE"
else
  cat "$FAKE_REVIEW_FIXTURE"
fi
`

const buildFixture = `{"type":"result","subtype":"success","is_error":false,"num_turns":1,` +
	`"session_id":"build-1","total_cost_usd":0,"usage":{"input_tokens":1,"output_tokens":1},` +
	`"structured_output":{"result":"DONE"}}` + "\n"

const reviewFixture = `{"type":"result","subtype":"success","is_error":false,"num_turns":1,` +
	`"session_id":"review-1","total_cost_usd":0,"usage":{"input_tokens":1,"output_tokens":1},` +
	`"structured_output":{"findings":[]}}` + "\n"

// driveRepo builds a root remoted at a bare origin, with main pushed so a fresh cut can fetch it,
// and installs the claude mount at root so the profile resolves to a host with a session contract.
func driveRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	backlogtest.SeedText(t, root, driveBacklog)
	// The install ignores the state dir and the mount's rendered copies, so Check's scope never sees them.
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

// setupDriveFakeClaude drops the fake claude script on a fresh PATH entry, with its fixtures at
// absolute paths and a fresh counter file, so it replays the builder then the reviewer in order.
func setupDriveFakeClaude(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	if err := os.WriteFile(script, []byte(driveFakeClaude), 0o755); err != nil {
		t.Fatal(err)
	}
	fixtures := t.TempDir()
	build := filepath.Join(fixtures, "build.jsonl")
	review := filepath.Join(fixtures, "review.jsonl")
	if err := os.WriteFile(build, []byte(buildFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(review, []byte(reviewFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_COUNTER", filepath.Join(fixtures, "counter"))
	t.Setenv("FAKE_BUILD_FIXTURE", build)
	t.Setenv("FAKE_REVIEW_FIXTURE", review)
}

// escalateOnTimeoutClaude hangs a builder, then answers its escalation session by its plugin-dir argument.
const escalateOnTimeoutClaude = `#!/bin/sh
case "$*" in
  *"/plugins/escalation"*) cat "$FAKE_ESCALATE_FIXTURE"; exit 0 ;;
esac
sleep "$FAKE_HANG_SLEEP"
`

// escalateFixture is the orchestrator's one settled action: stop the group for a person.
const escalateFixture = `{"type":"result","subtype":"success","is_error":false,"num_turns":1,` +
	`"session_id":"escalate-1","total_cost_usd":0,"usage":{"input_tokens":1,"output_tokens":1},` +
	`"structured_output":{"action":"stop","needs":"a person's call"}}` + "\n"

// TestAGroupPastItsBudgetSavesAndEscalatesInsteadOfOnlyDying hangs a builder under a short budget,
// and proves the conductor saves it as a settled escalation, never only the budget's own error.
func TestAGroupPastItsBudgetSavesAndEscalatesInsteadOfOnlyDying(t *testing.T) {
	root := driveRepo(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	if err := os.WriteFile(script, []byte(escalateOnTimeoutClaude), 0o755); err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(t.TempDir(), "escalate.jsonl")
	if err := os.WriteFile(fixture, []byte(escalateFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_ESCALATE_FIXTURE", fixture)
	t.Setenv("FAKE_HANG_SLEEP", fmt.Sprintf("61.%d", os.Getpid()))
	client := &pr.Client{Run: func(string, ...string) (string, error) { return "[]", nil }}

	code, err := Drive(Options{Root: root, Target: "TG-40.1", Budget: 50 * time.Millisecond, PR: client})
	if code != 1 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("drive = %d, %v; want the spent budget once the escalation settles", code, err)
	}

	state, err := conductor.LoadState(conductor.StatePath(root, "TG-40.1"))
	if err != nil {
		t.Fatal(err)
	}
	if state.Current != conductor.Escalated || !state.Answered || !state.Stop {
		t.Fatalf("state = %+v, want an answered, stopped escalation saved", state)
	}
	if !strings.Contains(state.Reason, "ran out of its budget") {
		t.Fatalf("reason = %q, want it naming the spent budget", state.Reason)
	}
}

// TestRunDrivesAGroupEndToEnd checks a group reaches Shipped, pushes its branch, and stamps one
// build and one review session, driven by a fake claude on PATH and a fake forge client.
func TestRunDrivesAGroupEndToEnd(t *testing.T) {
	root := driveRepo(t)
	setupDriveFakeClaude(t)
	var created []string
	client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
		if len(args) > 1 && args[0] == "pr" && args[1] == "create" {
			created = append(created, "opened")
			return "https://example.invalid/pr/1", nil
		}
		if len(args) > 0 && args[0] == "label" {
			return "[]", nil
		}
		return "", nil
	}}

	code, err := Drive(Options{Root: root, Target: "TG-40.1", PR: client})
	if err != nil {
		t.Fatalf("Drive = %v", err)
	}
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}

	state, err := conductor.LoadState(conductor.StatePath(root, "TG-40.1"))
	if err != nil {
		t.Fatal(err)
	}
	if state.Current != conductor.Shipped {
		t.Fatalf("state.Current = %s, want Shipped", state.Current)
	}
	if len(created) != 1 {
		t.Fatalf("pull requests opened = %d, want 1", len(created))
	}

	out, err := exec.Command("git", "-C", bareRemote(t, root), "branch", "--list", "feat/TG-40.1-a-fake-group").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "feat/TG-40.1-a-fake-group") {
		t.Fatalf("branch = %s; the group's branch must be pushed", out)
	}

	entries, err := line.Book(root).All()
	if err != nil {
		t.Fatal(err)
	}
	builds, reviews := 0, 0
	for _, entry := range entries {
		switch entry.Station {
		case "build":
			builds++
		case "review":
			reviews++
		}
	}
	if lenses := len(review.ForMode("full")); builds != 1 || reviews != lenses {
		t.Fatalf("builds = %d, reviews = %d; want one build and one review per full-mode lens", builds, reviews)
	}

	// The reviewer read the build's diff, committed before review, never the empty branch the run was cut with.
	brief, err := os.ReadFile(filepath.Join(root, line.StateDir, "briefs", "TG-40.1-review.md"))
	if err != nil || !strings.Contains(string(brief), "+built") {
		t.Fatalf("review brief = %q, %v; the reviewer must see the builder's change", brief, err)
	}

	// The reviewer's own result is what ship read, not a copy rebuilt from state.
	review, err := os.ReadFile(line.ResultPath(root, "TG-40.1-review"))
	if err != nil || !strings.Contains(string(review), `"findings"`) {
		t.Fatalf("review result = %q, %v; the reviewer's result must be saved for ship", review, err)
	}
	// A no-epic group deletes its own file once it ships, its tasks' ticks the record.
	if _, err := exec.Command("git", "-C", bareRemote(t, root), "show",
		"feat/TG-40.1-a-fake-group:docs/backlog/TG-40.1-a-fake-group.md").CombinedOutput(); err == nil {
		t.Fatal("the shipped group's own file must be removed once it has no epic")
	}

	// A group sent back to review after Prepare closed its tasks still finds its plan, and ships again.
	state.Current = conductor.Reviewing
	if err := conductor.SaveState(conductor.StatePath(root, "TG-40.1"), state); err != nil {
		t.Fatal(err)
	}
	if code, err := Drive(Options{Root: root, Target: "TG-40.1", PR: client}); err != nil || code != 0 {
		t.Fatalf("Drive after a rewind = %d, %v; a resumed group must find its closed tasks' plan", code, err)
	}
}

// TestRunClearsAMergedGroupsWorktreeBeforeItCuts checks a run removes an earlier group's worktree once origin
// holds its pushed branch in main, keeps the person's branch, and leaves its own shipped group's worktree for the merge.
func TestRunClearsAMergedGroupsWorktreeBeforeItCuts(t *testing.T) {
	root := driveRepo(t)
	setupDriveFakeClaude(t)
	stale := filepath.Join(root, line.StateDir, "wt", "TG-39.1")
	runGit(t, root, "worktree", "add", "-b", "feat/old", stale, "main")
	if err := os.WriteFile(filepath.Join(stale, "old.txt"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, stale, "add", "old.txt")
	runGit(t, stale, "commit", "-m", "old")
	runGit(t, stale, "push", "origin", "feat/old", "feat/old:main")
	client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
		if len(args) > 1 && args[0] == "pr" && args[1] == "create" {
			return "https://example.invalid/pr/1", nil
		}
		if len(args) > 0 && args[0] == "label" {
			return "[]", nil
		}
		return "", nil
	}}

	var out strings.Builder
	if code, err := Drive(Options{Root: root, Target: "TG-40.1", PR: client, Stdout: &out}); err != nil || code != 0 {
		t.Fatalf("Drive = %d, %v", code, err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Fatalf("the merged group's worktree survived the run; out = %s", out.String())
	}
	if branches := gitOut(t, root, "branch", "--list", "feat/old"); branches == "" {
		t.Fatalf("feat/old, a person's branch, was deleted by the run; out = %s", out.String())
	}
	if !strings.Contains(out.String(), "removed worktree") {
		t.Fatalf("out = %q; the run must print what it removed", out.String())
	}
	state, err := line.LoadRunFor(root, "TG-40.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(line.WorktreePath(root, state.Worktree)); err != nil {
		t.Fatalf("the shipped group's own worktree was removed before its PR merged: %v", err)
	}
}

// TestRunClearsALeftoverThatLandsOnlyAfterTheCut checks a worktree landed into main mid-run is gone.
// The prune after Ship removes it, proving it is not only the one before the cut.
func TestRunClearsALeftoverThatLandsOnlyAfterTheCut(t *testing.T) {
	root := driveRepo(t)
	setupDriveFakeClaude(t)
	stale := filepath.Join(root, line.StateDir, "wt", "TG-39.1")
	runGit(t, root, "worktree", "add", "-b", "feat/old", stale, "main")
	if err := os.WriteFile(filepath.Join(stale, "old.txt"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, stale, "add", "old.txt")
	runGit(t, stale, "commit", "-m", "old")
	runGit(t, stale, "push", "origin", "feat/old")
	// feat/old is pushed but unmerged, so only the fake builder landing it mid-run triggers the post-Ship prune.
	t.Setenv("FAKE_LAND_BRANCH", "feat/old")
	t.Setenv("FAKE_LAND_REPO", stale)
	client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
		if len(args) > 1 && args[0] == "pr" && args[1] == "create" {
			return "https://example.invalid/pr/1", nil
		}
		if len(args) > 0 && args[0] == "label" {
			return "[]", nil
		}
		return "", nil
	}}

	var out strings.Builder
	if code, err := Drive(Options{Root: root, Target: "TG-40.1", PR: client, Stdout: &out}); err != nil || code != 0 {
		t.Fatalf("Drive = %d, %v", code, err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Fatalf("a worktree that landed during the run survived; out = %s", out.String())
	}
	if !strings.Contains(out.String(), "removed worktree") {
		t.Fatalf("out = %q; the prune after Ship must print what it removed", out.String())
	}
}

// TestRunRefusesToCutWhereLeftoversCannotBeListed checks a cut stops when the root is no git repo to prune.
func TestRunRefusesToCutWhereLeftoversCannotBeListed(t *testing.T) {
	root := t.TempDir()
	// A .git naming no repo stops git's walk up to any repo holding the temp dir.
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+filepath.Join(root, "missing")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if _, err := cutIfNeeded(root, &line.Plan{Group: "TG-40.1"}, &out); err == nil {
		t.Fatalf("cutIfNeeded cut a group where no worktree can be listed; out = %s", out.String())
	}
	if out.Len() != 0 {
		t.Fatalf("out = %q; nothing was removed", out.String())
	}
}

// TestRunMergesAShippedGroupIntoItsEpicBranch checks a group cut from its epic's branch ends with its
// PR merged there by a merge commit, and state.json's merged flag set.
func TestRunMergesAShippedGroupIntoItsEpicBranch(t *testing.T) {
	root := driveRepo(t)
	setupDriveFakeClaude(t)
	groupPath := filepath.Join(root, "docs", "backlog", "TG-40.1-a-fake-group.md")
	data, err := os.ReadFile(groupPath)
	if err != nil {
		t.Fatal(err)
	}
	withEpic := strings.Replace(string(data), "type: feat\n", "type: feat\nepic: EPIC-40\n", 1)
	if withEpic == string(data) {
		t.Fatal("the group's yaml block was not found to add an epic to")
	}
	if err := os.WriteFile(groupPath, []byte(withEpic), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "commit", "-am", "an epic")
	runGit(t, root, "push", "origin", "main", "main:refs/heads/feat/1.0.0")
	var merged []string
	client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
		switch {
		case len(args) > 1 && args[0] == "pr" && args[1] == "create":
			return "https://example.invalid/pr/7", nil
		case len(args) > 1 && args[0] == "pr" && args[1] == "view":
			return `{"number":7,"url":"https://example.invalid/pr/7","state":"OPEN"}`, nil
		case len(args) > 1 && args[0] == "pr" && args[1] == "merge":
			merged = append(merged, strings.Join(args[2:], " "))
		case len(args) > 0 && args[0] == "label":
			return "[]", nil
		}
		return "", nil
	}}

	if code, err := Drive(Options{Root: root, Target: "TG-40.1", PR: client}); err != nil || code != 0 {
		t.Fatalf("Drive = %d, %v", code, err)
	}
	state, err := conductor.LoadState(conductor.StatePath(root, "TG-40.1"))
	if err != nil {
		t.Fatal(err)
	}
	if state.Current != conductor.Shipped || !state.Merged {
		t.Fatalf("state = %s, merged %v; a shipped group on its epic branch is merged there", state.Current, state.Merged)
	}
	if len(merged) != 1 || merged[0] != "7 --merge" {
		t.Fatalf("merges = %q; want the group's PR merged once with a merge commit", merged)
	}
}

// bareRemote reads root's own push URL for origin, which is the bare repo the group's branch lands on.
func bareRemote(t *testing.T, root string) string {
	t.Helper()
	cmd := exec.Command("git", "remote", "get-url", "--push", "origin")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestNewDriverWiresTheReReviewToCommitTheRepairAndDiffSinceTheReviewedCommit(t *testing.T) {
	root := requestsRepo(t)
	plan := requestsPlan()
	driver, err := newDriver(root, plan, "run-1", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if driver.ReReview == nil {
		t.Fatal("newDriver left ReReview nil; a production run would resume the reviewer with the findings alone")
	}
	reviewed := strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))
	path := filepath.Join(root, "b", "two.go")
	if err := os.WriteFile(path, []byte("package b\n\n// Two does nothing.\nfunc Two() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := conductor.State{Reviewed: reviewed, Findings: map[review.Lens][]conductor.Finding{
		review.Quality: {{Severity: "high", Verified: true, File: "b/two.go", Line: 3, Title: "Two has no comment"}},
	}}
	input, err := driver.ReReview(review.Quality, s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(input, "Two does nothing") || strings.Contains(input, "One returns one") {
		t.Fatalf("re-review input = %q, want the uncommitted repair's diff alone", input)
	}
	if !strings.Contains(input, "`b/two.go:3` high: Two has no comment") {
		t.Fatalf("re-review input = %q, want the lens's open finding by file and line", input)
	}
	other, err := driver.ReReview(review.Security, s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(other, "Two has no comment") {
		t.Fatalf("security re-review input = %q, want no other lens's finding", other)
	}
}

// lensHost is a canned host whose every session returns no findings; it is safe for parallel lenses.
type lensHost struct {
	mu     sync.Mutex
	starts []mount.StartRequest
}

func (h *lensHost) Preflight(context.Context) error { return nil }

func (h *lensHost) Start(ctx context.Context, req mount.StartRequest) (mount.Handle, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.starts = append(h.starts, req)
	return mount.Handle(fmt.Sprintf("session-%d", len(h.starts))), nil
}

func (h *lensHost) Resume(context.Context, mount.Handle, string) (mount.Handle, error) {
	return "", errors.New("no session resumes here")
}

func (h *lensHost) Stream(context.Context, mount.Handle) (<-chan mount.Event, error) {
	out := make(chan mount.Event)
	close(out)
	return out, nil
}

func (h *lensHost) Result(mount.Handle) (mount.Result, error) {
	return mount.Result{Value: map[string]any{"findings": []any{}}}, nil
}

func (h *lensHost) Stop(context.Context, mount.Handle) error { return nil }

func (h *lensHost) Capabilities() mount.Capabilities { return mount.Capabilities{Structured: true} }

// passingStations is every model-free station passing at once, so a drive reaches Shipped from review.
type passingStations struct{}

func (passingStations) Snapshot() error                           { return nil }
func (passingStations) Check(context.Context) ([]string, error)   { return nil, nil }
func (passingStations) Prepare(context.Context) ([]string, error) { return nil, nil }
func (passingStations) Ship(context.Context) error                { return nil }
func (passingStations) Head() (string, error)                     { return "reviewed", nil }
func (passingStations) Diff(string) (string, error)               { return "", nil }
func (passingStations) Merge() (bool, error)                      { return false, nil }

// TestRunStartsOneReviewerSessionPerLens: the ledger shows three lens sessions in full mode
// and one in economy mode, each bound to its lens's skill on the reviewer tier.
func TestRunStartsOneReviewerSessionPerLens(t *testing.T) {
	cases := []struct {
		mode   string
		lenses []review.Lens
	}{
		{"full", []review.Lens{review.Correctness, review.Security, review.Quality}},
		{"economy", []review.Lens{review.Economy}},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			root := requestsRepo(t)
			plan := requestsPlan()
			plan.Profile.Mode = tc.mode
			host := &lensHost{}
			driver, err := newDriver(root, plan, "run-1", host, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			driver.Stations = passingStations{}
			start := conductor.State{Group: plan.Group, Current: conductor.Reviewing}
			final, err := driver.Resume(context.Background(), start)
			if err != nil || final.Current != conductor.Shipped {
				t.Fatalf("drive = %s, %v; want Shipped", final.Current, err)
			}
			entries, err := line.Book(root).All()
			if err != nil {
				t.Fatal(err)
			}
			reviews := 0
			for _, entry := range entries {
				if entry.Station == conductor.StationReview {
					reviews++
				}
			}
			if reviews != len(tc.lenses) {
				t.Fatalf("ledger review sessions = %d, want %d in %s mode", reviews, len(tc.lenses), tc.mode)
			}
			for i, lens := range tc.lenses {
				req := host.starts[i]
				if !strings.Contains(req.Brief, "`"+lens.Skill()+"`") || req.Model != "claude-opus-4" {
					t.Fatalf("%s request = %q on %q, want its skill bound on the reviewer tier", lens, req.Brief, req.Model)
				}
			}
		})
	}
}
