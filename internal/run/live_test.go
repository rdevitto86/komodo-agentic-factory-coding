package run

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"komodo/internal/conductor"
	"komodo/internal/install"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/mount/claude"
	"komodo/internal/pr"
)

// liveEnv opts into the live smoke test, which spends real plan tokens.
const liveEnv = "KOMODO_LIVE"

// seededFinding is the one blocking finding the first review carries, so a repair always runs.
var seededFinding = map[string]any{
	"severity": "high", "class": "correctness", "file": "greet/greet.go", "line": 1,
	"title":  "Seeded finding",
	"detail": "The live smoke test seeds this finding so the conductor's repair path runs.",
	"fix":    "Append the line `// Seeded: repaired.` to the end of greet/greet.go.",
}

// liveHost runs every session on the light tier's machine and seeds one finding into the first review.
type liveHost struct {
	mount.Contract
	light mount.Machine

	mu        sync.Mutex
	reviewers map[mount.Handle]bool
	seeded    bool
	resumed   int
}

func (h *liveHost) Start(req mount.StartRequest) (mount.Handle, error) {
	req.Model, req.Effort = h.light.Model, h.light.Effort
	handle, err := h.Contract.Start(req)
	if err == nil && req.Role == "reviewer" {
		h.mu.Lock()
		h.reviewers[handle] = true
		h.mu.Unlock()
	}
	return handle, err
}

func (h *liveHost) Resume(handle mount.Handle, input string) (mount.Handle, error) {
	h.mu.Lock()
	h.resumed++
	h.mu.Unlock()
	return h.Contract.Resume(handle, input)
}

func (h *liveHost) Result(handle mount.Handle) (mount.Result, error) {
	result, err := h.Contract.Result(handle)
	h.mu.Lock()
	defer h.mu.Unlock()
	if err != nil || !h.reviewers[handle] || h.seeded || result.Value == nil {
		return result, err
	}
	h.seeded = true
	findings, _ := result.Value["findings"].([]any)
	result.Value["findings"] = append(findings, seededFinding)
	return result, nil
}

// liveRepo builds a scratch Go repo from testdata/live, remoted at a bare origin with main pushed,
// with the claude mount rendered against a freshly built komodo binary.
func liveRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	backlogText, err := os.ReadFile(filepath.Join("testdata", "live", "BACKLOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"BACKLOG.md": string(backlogText),
		"go.mod":     "module example.com/live\n\ngo 1.22\n",
		"README.md":  "# live\n\nA scratch repo for komodo's live smoke test.\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	binary := filepath.Join(t.TempDir(), "komodo")
	if out, err := exec.Command("go", "build", "-o", binary, "komodo/cmd/komodo").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v: %s", err, out)
	}
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.email", "live@example.com")
	runGit(t, root, "config", "user.name", "live")
	rendered, err := claude.Render(root, binary)
	if err != nil {
		t.Fatal(err)
	}
	// The line's state and the rendered project copies stay out of git, as komodo install leaves them.
	ignore := install.Plan{Host: "repo", Root: root}
	ignore.AddIgnore("/"+line.StateDir+"/", "the line's run state")
	for _, change := range rendered.Project().Changes {
		if rel, err := filepath.Rel(root, change.Path); err == nil && !strings.HasPrefix(rel, "..") {
			ignore.AddIgnore("/"+filepath.ToSlash(rel), "a rendered copy")
		}
	}
	for _, plan := range []install.Plan{ignore, rendered} {
		if _, err := plan.Apply(); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "seed")
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "-q", "--bare", bare)
	runGit(t, root, "remote", "add", "origin", bare)
	runGit(t, root, "push", "-q", "origin", "main")
	return root
}

// TestLiveDrivesATwoTaskGroupThroughRealSessions proves the conductor's wiring on the host itself:
// one build, a review of the whole diff, a repair by the builder, and a branch that fast-forwards.
func TestLiveDrivesATwoTaskGroupThroughRealSessions(t *testing.T) {
	if os.Getenv(liveEnv) != "1" {
		t.Skip("set KOMODO_LIVE=1 to run real sessions; this spends plan tokens")
	}
	root := liveRepo(t)
	var opened []string
	client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
		switch {
		case len(args) > 1 && args[0] == "pr" && args[1] == "create":
			opened = append(opened, strings.Join(args, " "))
			return "https://example.invalid/pr/1", nil
		case len(args) > 0 && args[0] == "label":
			return "[]", nil
		}
		return "", nil
	}}

	plan, err := line.PlanForGroup(root, "TG-01.1")
	if err != nil || plan == nil {
		t.Fatalf("plan = %v, %v", plan, err)
	}
	runState, err := cutIfNeeded(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	worktree := line.WorktreePath(root, plan.Worktree)
	contract, err := driverContract(root, worktree)
	if err != nil {
		t.Fatal(err)
	}
	host := &liveHost{Contract: contract, light: plan.Profile.Tiers.Light, reviewers: map[mount.Handle]bool{}}
	driver, err := newDriver(root, plan, runState.Run, host, client)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), GroupBudget)
	defer cancel()
	fresh := conductor.State{
		Group: plan.Group, Worktree: worktree, Branch: plan.Branch, Current: conductor.Ready, SlotFree: true,
	}
	final, err := driver.Drive(ctx, fresh)
	if err != nil {
		t.Fatalf("Drive = %v at %s", err, final.Current)
	}
	if final.Current != conductor.Shipped {
		t.Fatalf("final = %s, want Shipped", final.Current)
	}
	if len(opened) != 1 {
		t.Fatalf("pull requests opened = %q, want one", opened)
	}

	entries, err := line.Book(root).All()
	if err != nil {
		t.Fatal(err)
	}
	builds, reviews, repairs := 0, 0, 0
	for _, entry := range entries {
		switch entry.Station {
		case conductor.StationBuild:
			builds++
		case conductor.StationReview:
			reviews++
		case conductor.StationRepair:
			if entry.Role != "builder" {
				t.Fatalf("a repair ran as %q; only the builder repairs", entry.Role)
			}
			repairs++
		}
	}
	if builds != 1 || reviews < 2 || repairs < 1 {
		t.Fatalf("builds = %d, reviews = %d, repairs = %d; want one build, a repair, and a review after it",
			builds, reviews, repairs)
	}
	if !host.seeded {
		t.Fatal("no review result carried the seeded finding")
	}

	// The last review read the whole group's diff: both tasks' files.
	brief, err := os.ReadFile(filepath.Join(root, line.StateDir, "briefs", plan.Group+"-review.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"greet/greet.go", "greet/greet_test.go"} {
		if !strings.Contains(string(brief), file) {
			t.Fatalf("the review brief never showed %s; a review sees the whole diff", file)
		}
	}

	bare := bareRemote(t, root)
	head := "refs/heads/" + plan.Branch
	shipped, err := exec.Command("git", "-C", bare, "show", head+":greet/greet.go").CombinedOutput()
	if err != nil || !strings.Contains(string(shipped), "Seeded: repaired.") {
		t.Fatalf("pushed greet.go = %s, %v; the repair's fix must ship", shipped, err)
	}
	ancestor := exec.Command("git", "-C", bare, "merge-base", "--is-ancestor", "refs/heads/"+plan.Base, head)
	if out, err := ancestor.CombinedOutput(); err != nil {
		t.Fatalf("%s does not fast-forward from %s: %v: %s", plan.Branch, plan.Base, err, out)
	}
}
