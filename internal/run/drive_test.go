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
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(driveBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
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
	shipped, err := exec.Command("git", "-C", bareRemote(t, root), "show", "feat/TG-40.1-a-fake-group:BACKLOG.md").CombinedOutput()
	if err != nil || strings.Contains(string(shipped), "[READY]") || !strings.Contains(string(shipped), "[DONE]") {
		t.Fatalf("shipped BACKLOG.md = %s, %v; Prepare must mark every task DONE", shipped, err)
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

// TestRunMergesAShippedGroupIntoItsEpicBranch checks a group cut from its epic's branch ends with its
// PR merged there by a merge commit, and state.json's merged flag set.
func TestRunMergesAShippedGroupIntoItsEpicBranch(t *testing.T) {
	root := driveRepo(t)
	setupDriveFakeClaude(t)
	epicBacklog := "## [EPIC-40] The fake epic. Ships as `1.0.0`\n\n" + driveBacklog
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(epicBacklog), 0o644); err != nil {
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
	driver, err := newDriver(root, plan, "run-1", nil, nil)
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

func (h *lensHost) Preflight() error { return nil }

func (h *lensHost) Start(req mount.StartRequest) (mount.Handle, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.starts = append(h.starts, req)
	return mount.Handle(fmt.Sprintf("session-%d", len(h.starts))), nil
}

func (h *lensHost) Resume(mount.Handle, string) (mount.Handle, error) {
	return "", errors.New("no session resumes here")
}

func (h *lensHost) Stream(mount.Handle) (<-chan mount.Event, error) {
	out := make(chan mount.Event)
	close(out)
	return out, nil
}

func (h *lensHost) Result(mount.Handle) (mount.Result, error) {
	return mount.Result{Value: map[string]any{"findings": []any{}}}, nil
}

func (h *lensHost) Stop(mount.Handle) error { return nil }

func (h *lensHost) Capabilities() mount.Capabilities { return mount.Capabilities{Structured: true} }

// passingStations is every model-free station passing at once, so a drive reaches Shipped from review.
type passingStations struct{}

func (passingStations) Snapshot() error             { return nil }
func (passingStations) Check() ([]string, error)    { return nil, nil }
func (passingStations) Prepare() ([]string, error)  { return nil, nil }
func (passingStations) Ship() error                 { return nil }
func (passingStations) Head() (string, error)       { return "reviewed", nil }
func (passingStations) Diff(string) (string, error) { return "", nil }
func (passingStations) Merge() (bool, error)        { return false, nil }

// TestRunStartsOneReviewerSessionPerLens is REQ-19: the ledger shows three lens sessions in full mode
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
			driver, err := newDriver(root, plan, "run-1", host, nil)
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
