package line

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/backlog/backlogtest"
	"komodo/internal/pr"
)

// blockedBacklog is shipBacklog with its one task still open, as a stopped group's is.
var blockedBacklog = strings.Replace(shipBacklog, "[P: C] [DONE]", "[P: C] [READY]", 1)

// blockedPlan is the plan of the group shipRepo builds.
func blockedPlan() *Plan {
	return &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it", Files: []string{"a/one.go"}}},
	}
}

// blockedNote is the note a test group stops with.
var blockedNote = backlog.BlockerNote{
	At: time.Date(2026, 9, 25, 14, 2, 0, 0, time.UTC), Run: "run-1", State: "Building",
	Items: []string{"TSK-09.1.1: which clock?"}, Needs: "a decision on the clock",
}

// stopGroup writes the open backlog and a change the builder left, as a group stops with.
func stopGroup(t *testing.T, group string) {
	t.Helper()
	reseed(t, group, blockedBacklog)
	if err := os.MkdirAll(filepath.Join(group, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(group, "a", "one.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAddBlockerNoteWritesIntoAGroupFile proves the note lands in the group's own the group index file, blocking its
// open task.
func TestAddBlockerNoteWritesIntoAGroupFile(t *testing.T) {
	worktree := t.TempDir()
	runGit(t, worktree, "init")
	runGit(t, worktree, "config", "user.email", "a@example.com")
	runGit(t, worktree, "config", "user.name", "a")
	backlogtest.Seed(t, worktree, backlog.GroupFile{
		ID: "TG-09.1", Title: "A group", Priority: "C", Status: "READY", Type: "feat", Version: "2.0.0",
		Tasks: []backlog.GroupTask{{ID: "TSK-09.1.1", Title: "Do it", Files: []string{"a/one.go"}}},
	})
	path := groupPath(worktree, "TG-09.1")
	runGit(t, worktree, "add", "-A")
	runGit(t, worktree, "commit", "-m", "seed")
	blocked, err := addBlockerNote(worktree, "TG-09.1", blockedNote)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocked) != 1 || blocked[0] != "TSK-09.1.1" {
		t.Fatalf("blocked = %v", blocked)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[BLOCKED]") || !strings.Contains(string(data), "which clock?") {
		t.Fatalf("group file = %s", data)
	}
	if err := dropCredentialNote(worktree, ShipHandoff{Group: "TG-09.1"}); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "which clock?") {
		t.Fatalf("group file still carries the note: %s", data)
	}
}

func TestShipBlockedCommitsTheWorkAndNoteThenOpensABlockedDraft(t *testing.T) {
	root, group := shipRepo(t)
	stopGroup(t, group)
	var calls []string
	client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		switch {
		case args[0] == "label":
			return `[{"name":"@agent 🤖"},{"name":"scope/harness ⚙️"},{"name":"status/blocked ⛔"}]`, nil
		case args[1] == "create":
			return "https://example.com/pull/7", nil
		}
		return "", nil
	}}
	result, err := ShipBlocked(root, blockedPlan(), blockedNote, client)
	if err != nil {
		t.Fatal(err)
	}
	if result.URL != "https://example.com/pull/7" || !result.Draft || len(result.Blocked) != 1 {
		t.Fatalf("result = %+v, want a draft with its open task blocked", result)
	}
	// A blocked PR carries the labels every shipped PR earns, plus the blocked one.
	if want := []string{"@agent 🤖", "scope/harness ⚙️", "status/blocked ⛔"}; !slices.Equal(result.Labels, want) {
		t.Fatalf("labels = %v, want %v", result.Labels, want)
	}
	joined := strings.Join(calls, "\n")
	if !strings.Contains(joined, "--draft") || !strings.Contains(joined, "Needs: a decision on the clock") {
		t.Fatalf("gh calls = %s, want a draft whose body is the note", joined)
	}
	out, err := exec.Command("git", "-C", group, "log", "--format=%s", "-2").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if subjects := strings.Split(strings.TrimSpace(string(out)), "\n"); len(subjects) != 2 ||
		!strings.HasPrefix(subjects[0], "docs: A group is blocked") || !strings.HasPrefix(subjects[1], "wip: A group") {
		t.Fatalf("commits = %q, want the WIP commit then the note", subjects)
	}
	data, err := os.ReadFile(groupPath(group, "TG-09.1"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "> - Saved: WIP commit `") || !strings.Contains(string(data), "[P: C] [BLOCKED]") {
		t.Fatalf("backlog =\n%s\nwant the note naming the WIP commit and the task BLOCKED", data)
	}
}

func TestShipBlockedKeepsItsCommitsLocalWhenScrubbed(t *testing.T) {
	root, group := shipRepo(t)
	stopGroup(t, group)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_CONFIG_KEY_0", "credential.helper")
	t.Setenv("GIT_CONFIG_VALUE_0", "")
	client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
		t.Fatalf("gh must not run while the environment is scrubbed: %v", args)
		return "", nil
	}}
	result, err := ShipBlocked(root, blockedPlan(), blockedNote, client)
	if err != nil || result.URL != "" || len(result.Warnings) != 1 {
		t.Fatalf("result = %+v, %v; want local commits and a warning", result, err)
	}
}

// TestBlockLocalCommitsTheWorkAndNoteWithoutPushing proves a no-ship block leaves both commits on the branch and origin untouched.
func TestBlockLocalCommitsTheWorkAndNoteWithoutPushing(t *testing.T) {
	root, group := shipRepo(t)
	stopGroup(t, group)
	result, err := BlockLocal(root, blockedPlan(), blockedNote)
	if err != nil || result.URL != "" || len(result.Blocked) != 1 || len(result.Warnings) != 1 {
		t.Fatalf("result = %+v, %v; want local commits, the open task blocked and one warning", result, err)
	}
	out, err := exec.Command("git", "-C", group, "log", "--format=%s", "-2").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if subjects := strings.Split(strings.TrimSpace(string(out)), "\n"); len(subjects) != 2 ||
		!strings.HasPrefix(subjects[0], "docs: A group is blocked") || !strings.HasPrefix(subjects[1], "wip: A group") {
		t.Fatalf("commits = %q, want the WIP commit then the note", subjects)
	}
	out, err = exec.Command("git", "-C", group, "ls-remote", "origin", "refs/heads/feat/a-group").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "" {
		t.Fatalf("ls-remote = %s, %v; want nothing pushed", out, err)
	}
}

func TestShipBlockedPushesWithoutAPullRequestClient(t *testing.T) {
	root, group := shipRepo(t)
	stopGroup(t, group)
	result, err := ShipBlocked(root, blockedPlan(), blockedNote, nil)
	if err != nil || result.URL != "" {
		t.Fatalf("result = %+v, %v; want the branch pushed and no pull request", result, err)
	}
	out, err := exec.Command("git", "-C", group, "ls-remote", "origin", "refs/heads/feat/a-group").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		t.Fatalf("ls-remote = %s, %v; want the blocked branch on origin", out, err)
	}
}

func TestShipBlockedRefreshesThePullRequestAlreadyOpen(t *testing.T) {
	root, group := shipRepo(t)
	stopGroup(t, group)
	var edited bool
	client := &pr.Client{Dir: group, Run: func(_ string, args ...string) (string, error) {
		switch {
		case args[0] == "label":
			return `[]`, nil
		case args[1] == "create":
			return "", errors.New("a pull request already exists")
		case args[1] == "view":
			return `{"number":7,"url":"https://example.com/pull/7","state":"OPEN"}`, nil
		case args[1] == "edit" && args[2] == "https://example.com/pull/7":
			edited = true
		}
		return "", nil
	}}
	result, err := ShipBlocked(root, blockedPlan(), blockedNote, client)
	if err != nil || result.URL != "https://example.com/pull/7" || !edited {
		t.Fatalf("result = %+v, %v, edited %v; want the open pull request refreshed", result, err, edited)
	}
}

func TestShipBlockedRefusesAWorktreeWithNoBacklog(t *testing.T) {
	root, group := shipRepo(t)
	if err := os.RemoveAll(filepath.Join(group, "docs", "backlog")); err != nil {
		t.Fatal(err)
	}
	if _, err := ShipBlocked(root, blockedPlan(), blockedNote, nil); err == nil {
		t.Fatal("ship blocked = nil, want the missing backlog named")
	}
}

func TestShipBlockedRefusesAWorktreeThatIsNoRepo(t *testing.T) {
	plan := blockedPlan()
	plan.Worktree = t.TempDir()
	if _, err := ShipBlocked(t.TempDir(), plan, blockedNote, nil); err == nil {
		t.Fatal("ship blocked = nil, want the failed WIP commit")
	}
}

func TestLabelBlockedAddsOnlyTheLabelTheRepoDefines(t *testing.T) {
	for _, tc := range []struct {
		name   string
		labels string
		fail   error
		want   int
	}{
		{"the repo defines it", `[{"name":"bug"},{"name":"status/blocked"}]`, nil, 1},
		{"the repo lacks it", `[{"name":"bug"}]`, nil, 0},
		{"the labels cannot be listed", "", errors.New("gh: offline"), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var added []string
			client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
				if args[0] == "label" {
					return tc.labels, tc.fail
				}
				added = append(added, args...)
				return "", nil
			}}
			labels, warnings := labelBlocked(client, "7")
			if len(labels) != tc.want || (tc.want == 0) != (len(warnings) == 1) {
				t.Fatalf("labels %v, warnings %v; want %d label(s)", labels, warnings, tc.want)
			}
			if tc.want == 1 && !strings.Contains(strings.Join(added, " "), "--add-label status/blocked") {
				t.Fatalf("gh edit = %v, want the status/blocked label added", added)
			}
		})
	}
}
