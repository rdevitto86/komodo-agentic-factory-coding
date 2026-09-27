package run

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/conductor"
	"komodo/internal/line"
	"komodo/internal/mount/claude"
	"komodo/internal/pr"
)

const driveBacklog = "### [TG-40.1] A fake group\n```yaml\ntype: feat\nversion: 1.0.0\nmode: single\n```\n\n" +
	"#### [TSK-40.1.1] Do the work [P: C] [READY]\n```yaml\nfiles: [change.txt]\ndone_when:\n  - true\n```\n"

// driveFakeClaude leaves a fake change uncommitted, as a real builder does, and replays the builder result, then the reviewer result.
const driveFakeClaude = `#!/bin/sh
if [ -n "$FAKE_HANG_MARKER" ]; then
  touch "$FAKE_HANG_MARKER"
  sleep 61.25
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

	out, err := exec.Command("git", "-C", bareRemote(t, root), "branch", "--list", "feat/a-fake-group").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "feat/a-fake-group") {
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
	if builds != 1 || reviews != 1 {
		t.Fatalf("builds = %d, reviews = %d; want exactly one each", builds, reviews)
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
	shipped, err := exec.Command("git", "-C", bareRemote(t, root), "show", "feat/a-fake-group:BACKLOG.md").CombinedOutput()
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
	s := conductor.State{Reviewed: reviewed, Findings: []conductor.Finding{
		{Severity: "high", Verified: true, File: "b/two.go", Line: 3, Title: "Two has no comment"},
	}}
	input, err := driver.ReReview(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(input, "Two does nothing") || strings.Contains(input, "One returns one") {
		t.Fatalf("re-review input = %q, want the uncommitted repair's diff alone", input)
	}
	if !strings.Contains(input, "`b/two.go:3` high: Two has no comment") {
		t.Fatalf("re-review input = %q, want the open finding by file and line", input)
	}
}
