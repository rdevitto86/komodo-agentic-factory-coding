package conductor

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"komodo/internal/backlog"
	"komodo/internal/backlog/backlogtest"
	"komodo/internal/line"
	"komodo/internal/pr"
)

// stackText is an epic holding a parent and its dependent child, plus a group depending across epics.
const stackText = "### [TG-01.1] Parent\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
	"#### [TSK-01.1.1] One [P: C] [DONE]\n```yaml\nfiles: [a.txt]\n```\n\n" +
	"### [TG-01.2] Child\n```yaml\ntype: feat\nversion: 1.0.0\ndepends_on: [TG-01.1]\n```\n\n" +
	"#### [TSK-01.2.1] Two [P: C] [READY]\n```yaml\nfiles: [b.txt]\n```\n\n" +
	"### [TG-02.1] Elsewhere\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-02.1.1] Three [P: C] [READY]\n```yaml\nfiles: [c.txt]\n```\n\n" +
	"### [TG-01.3] Across\n```yaml\ntype: feat\nversion: 1.0.0\ndepends_on: [TG-02.1]\n```\n\n" +
	"#### [TSK-01.3.1] Four [P: C] [READY]\n```yaml\nfiles: [d.txt]\n```\n"

// stackGroup returns one group of stackText, failing the test when it is missing.
func stackGroup(t *testing.T, parsed backlog.Backlog, id string) backlog.Group {
	t.Helper()
	group, ok := parsed.Group(id)
	if !ok {
		t.Fatalf("no %s in the stack backlog", id)
	}
	return group
}

func TestStackBaseTargetsAnUnmergedParentElseTheEpic(t *testing.T) {
	parsed := backlog.Parse(stackText)
	parent := stackGroup(t, parsed, "TG-01.1").Branch()
	cases := []struct {
		name   string
		group  string
		merged bool
		want   string
	}{
		{"a group with no parent targets its epic", "TG-01.1", false, "feat/1.0.0"},
		{"a child of an unmerged parent targets the parent's branch", "TG-01.2", false, parent},
		{"a child of a merged parent targets the epic", "TG-01.2", true, "feat/1.0.0"},
		{"a parent in another epic never stacks", "TG-01.3", false, "feat/1.0.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := StackBase(parsed, stackGroup(t, parsed, tc.group), func(string) bool { return tc.merged })
			if got != tc.want {
				t.Fatalf("base = %q, want %q", got, tc.want)
			}
		})
	}
}

// branchWith commits one file on a new branch cut from main in root, then returns to main.
func branchWith(t *testing.T, root, branch, name, content string) {
	t.Helper()
	gitIn(t, root, "checkout", "-q", "-b", branch, "main")
	writeIn(t, root, name, content)
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", branch)
	gitIn(t, root, "checkout", "-q", "main")
}

func TestTestMergeTurnsEachBreakageIntoAFixForTheGroupPreparing(t *testing.T) {
	root := checkRepo(t)
	writeIn(t, root, "shared.txt", "base\n")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "shared")
	branchWith(t, root, "feat/clash", "shared.txt", "theirs\n")
	branchWith(t, root, "feat/breaks", "broken.txt", "x\n")
	branchWith(t, root, "feat/clean", "clean.txt", "x\n")
	writeIn(t, root, "shared.txt", "ours\n")
	gitIn(t, root, "commit", "-q", "-am", "ours")
	const verify = "test ! -f broken.txt"
	cases := []struct {
		name  string
		ready []line.RunState
		want  string
	}{
		{"a clean group merges and passes", []line.RunState{{Group: "TG-3", Branch: "feat/clean"}}, ""},
		{"a conflicting group is a fix", []line.RunState{{Group: "TG-1", Branch: "feat/clash"}},
			"no longer merges with TG-1's branch feat/clash; it conflicts in shared.txt"},
		{"a group that breaks the build is a fix", []line.RunState{
			{Group: "TG-3", Branch: "feat/clean"}, {Group: "TG-2", Branch: "feat/breaks"},
		}, "`" + verify + "`"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixes, err := TestMerge(context.Background(), root, tc.ready, []string{verify})
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" && len(fixes) > 0 {
				t.Fatalf("fixes = %q, want none", fixes)
			}
			if tc.want != "" && (len(fixes) != 1 || !strings.Contains(fixes[0], tc.want)) {
				t.Fatalf("fixes = %q, want one naming %q", fixes, tc.want)
			}
			worktrees, err := exec.Command("git", "-C", root, "worktree", "list").Output()
			if err != nil || strings.Count(strings.TrimSpace(string(worktrees)), "\n") != 0 {
				t.Fatalf("worktrees = %q (%v); the scratch worktree must be removed", worktrees, err)
			}
			if status, _ := exec.Command("git", "-C", root, "status", "--porcelain").Output(); len(status) != 0 {
				t.Fatalf("status = %q; a test-merge must leave the group's worktree alone", status)
			}
		})
	}
}

// stackRepo builds root with stackText on main, the epic branch, the parent's branch, and the child's worktree
// cut from the parent with one commit, recorded as a run stacked on the parent.
func stackRepo(t *testing.T) (root, child string, parsed backlog.Backlog) {
	t.Helper()
	root = checkRepo(t)
	backlogtest.SeedText(t, root, stackText)
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "backlog")
	gitIn(t, root, "branch", "feat/1.0.0")
	parsed = backlog.Parse(stackText)
	parent := stackGroup(t, parsed, "TG-01.1").Branch()
	childBranch := stackGroup(t, parsed, "TG-01.2").Branch()
	branchWith(t, root, parent, "a.txt", "a\n")
	child = filepath.Join(t.TempDir(), "child")
	gitIn(t, root, "worktree", "add", "-q", "-b", childBranch, child, parent)
	writeIn(t, child, "b.txt", "b\n")
	gitIn(t, child, "add", "-A")
	gitIn(t, child, "commit", "-q", "-m", "child")
	state := line.RunState{Run: "r1", Group: "TG-01.2", Base: parent, Branch: childBranch, Worktree: child}
	if err := line.SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	return root, child, parsed
}

func TestRestackLeavesAChildOnItsUnmergedParent(t *testing.T) {
	root, _, parsed := stackRepo(t)
	moved, err := Restack(root, nil)
	if err != nil || len(moved) > 0 {
		t.Fatalf("restack = %v, %v; a parent still open keeps its child", moved, err)
	}
	if state, _ := line.LoadRunFor(root, "TG-01.2"); state.Base != stackGroup(t, parsed, "TG-01.1").Branch() {
		t.Fatalf("base = %q, want the parent's branch", state.Base)
	}
}

func TestRestackRebasesAChildOntoTheEpicOnceItsParentMerges(t *testing.T) {
	root, child, parsed := stackRepo(t)
	gitIn(t, root, "checkout", "-q", "feat/1.0.0")
	gitIn(t, root, "merge", "-q", "--no-ff", "--no-edit", stackGroup(t, parsed, "TG-01.1").Branch())
	gitIn(t, root, "checkout", "-q", "main")
	var calls []string
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "", nil
	}}
	moved, err := Restack(root, client)
	if err != nil || !slices.Equal(moved, []string{"TG-01.2 onto feat/1.0.0"}) {
		t.Fatalf("restack = %v, %v; want the child moved onto the epic", moved, err)
	}
	if state, _ := line.LoadRunFor(root, "TG-01.2"); state.Base != "feat/1.0.0" {
		t.Fatalf("base = %q, want the epic's branch recorded", state.Base)
	}
	if err := exec.Command("git", "-C", child, "merge-base", "--is-ancestor", "feat/1.0.0", "HEAD").Run(); err != nil {
		t.Fatal("the child does not sit on the epic after its restack")
	}
	if len(calls) > 0 {
		t.Fatalf("gh ran %q; a branch never pushed has no PR to retarget", calls)
	}
}

func TestRestackMergesTheEpicIntoAPushedChildPushesItAndRetargetsItsPR(t *testing.T) {
	root, child, parsed := stackRepo(t)
	bare := filepath.Join(t.TempDir(), "origin.git")
	gitIn(t, root, "init", "-q", "--bare", bare)
	gitIn(t, root, "remote", "add", "origin", bare)
	childBranch := stackGroup(t, parsed, "TG-01.2").Branch()
	gitIn(t, child, "push", "-q", "origin", childBranch)
	gitIn(t, root, "checkout", "-q", "feat/1.0.0")
	gitIn(t, root, "merge", "-q", "--no-ff", "--no-edit", stackGroup(t, parsed, "TG-01.1").Branch())
	gitIn(t, root, "checkout", "-q", "main")
	gitIn(t, root, "push", "-q", "origin", "feat/1.0.0")
	var calls []string
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "", nil
	}}
	moved, err := Restack(root, client)
	if err != nil || !slices.Equal(moved, []string{"TG-01.2 onto feat/1.0.0"}) {
		t.Fatalf("restack = %v, %v; want the pushed child moved onto the epic", moved, err)
	}
	if !slices.Equal(calls, []string{"pr edit " + childBranch + " --base feat/1.0.0"}) {
		t.Fatalf("gh ran %q, want the child's PR retargeted to the epic", calls)
	}
	cmd := exec.Command("git", "-C", bare, "merge-base", "--is-ancestor", "refs/heads/feat/1.0.0", "refs/heads/"+childBranch)
	if err := cmd.Run(); err != nil {
		t.Fatal("origin's child branch does not hold the epic after its restack")
	}
}

func TestRestackReturnsAFailedRetarget(t *testing.T) {
	root, child, parsed := stackRepo(t)
	bare := filepath.Join(t.TempDir(), "origin.git")
	gitIn(t, root, "init", "-q", "--bare", bare)
	gitIn(t, root, "remote", "add", "origin", bare)
	gitIn(t, child, "push", "-q", "origin", stackGroup(t, parsed, "TG-01.2").Branch())
	gitIn(t, root, "checkout", "-q", "feat/1.0.0")
	gitIn(t, root, "merge", "-q", "--no-ff", "--no-edit", stackGroup(t, parsed, "TG-01.1").Branch())
	gitIn(t, root, "checkout", "-q", "main")
	gitIn(t, root, "push", "-q", "origin", "feat/1.0.0")
	client := &pr.Client{Dir: root, Run: func(string, ...string) (string, error) {
		return "", errors.New("gh is down")
	}}
	if _, err := Restack(root, client); err == nil || !strings.Contains(err.Error(), "retargeting TG-01.2") {
		t.Fatalf("restack = %v, want the failed retarget named", err)
	}
	if state, _ := line.LoadRunFor(root, "TG-01.2"); state.Base == "feat/1.0.0" {
		t.Fatal("a failed retarget must not record the new base")
	}
}

func TestRestackReturnsAChildThatNoLongerRebases(t *testing.T) {
	root, child, parsed := stackRepo(t)
	gitIn(t, root, "checkout", "-q", "feat/1.0.0")
	gitIn(t, root, "merge", "-q", "--no-ff", "--no-edit", stackGroup(t, parsed, "TG-01.1").Branch())
	writeIn(t, root, "b.txt", "the epic's own b\n")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "epic edits b")
	gitIn(t, root, "checkout", "-q", "main")
	if _, err := Restack(root, nil); err == nil || !strings.Contains(err.Error(), "restacking TG-01.2") {
		t.Fatalf("restack = %v, want the conflicting child named", err)
	}
	if out, _ := exec.Command("git", "-C", child, "status", "--porcelain").Output(); len(out) != 0 {
		t.Fatalf("status = %q; a failed restack must abort its rebase", out)
	}
}

func TestMergedIntoCountsAParentWithNoBranchAsMerged(t *testing.T) {
	root, _, _ := stackRepo(t)
	if !mergedInto(root, "feat/gone", "feat/1.0.0") {
		t.Fatal("a parent with no branch left must count as merged")
	}
}

func TestTestMergeFailsWithNoHeadAndStopsWithItsContext(t *testing.T) {
	empty := t.TempDir()
	gitIn(t, empty, "init", "-q")
	ready := []line.RunState{{Group: "TG-2", Branch: "main"}}
	if _, err := TestMerge(context.Background(), empty, ready, nil); err == nil {
		t.Fatal("test-merge = nil, want the scratch worktree's failure in a repo with no commit")
	}
	root := checkRepo(t)
	branchWith(t, root, "feat/clean", "clean.txt", "x\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ready = []line.RunState{{Group: "TG-3", Branch: "feat/clean"}}
	if _, err := TestMerge(ctx, root, ready, []string{"", "true"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("test-merge = %v, want the stop", err)
	}
	if fixes, err := TestMerge(context.Background(), root, nil, []string{"false"}); err != nil || len(fixes) > 0 {
		t.Fatalf("test-merge = %q, %v; with no ready group nothing runs", fixes, err)
	}
}

func TestRestackSkipsARepoWithNoBacklog(t *testing.T) {
	if moved, err := Restack(t.TempDir(), nil); err != nil || len(moved) > 0 {
		t.Fatalf("restack = %v, %v; want nothing to restack", moved, err)
	}
}

func TestPrepareTestMergesEveryGroupReadyInTheSameRun(t *testing.T) {
	root := checkRepo(t)
	writeIn(t, root, ".komodo/commands.json", `{"compile": "true", "verify": "test ! -f broken.txt"}`)
	branchWith(t, root, "feat/breaks", "broken.txt", "x\n")
	branchWith(t, root, "feat/building", "other.txt", "x\n")
	runs := []struct {
		state line.RunState
		at    GroupState
	}{
		{line.RunState{Run: "r1", Group: "TG-1", Branch: "main"}, Preparing},
		{line.RunState{Run: "r1", Group: "TG-2", Branch: "feat/breaks"}, Shipping},
		{line.RunState{Run: "r1", Group: "TG-3", Branch: "feat/building"}, Building},
		{line.RunState{Run: "r0", Group: "TG-4", Branch: "feat/breaks"}, Shipped},
		{line.RunState{Run: "r1", Group: "TG-5", Branch: "feat/breaks"}, ""},
	}
	for _, each := range runs {
		if err := line.SaveRun(root, each.state); err != nil {
			t.Fatal(err)
		}
		// A group the conductor never saved a state for is never test-merged.
		if each.at == "" {
			continue
		}
		if err := SaveState(StatePath(root, each.state.Group), State{Group: each.state.Group, Current: each.at}); err != nil {
			t.Fatal(err)
		}
	}
	stations := &Line{Root: root, Plan: &line.Plan{
		Group: "TG-1", Title: "A group", Type: "feat", Base: "main", Branch: "main",
		Tasks: []line.PlanTask{{ID: "TSK-1", Files: []string{"sneaky.txt"}}},
	}}
	writeIn(t, root, "sneaky.txt", "x\n")
	fixes, err := stations.Prepare(context.Background())
	if err != nil || len(fixes) != 1 || !strings.Contains(fixes[0], "`test ! -f broken.txt`") {
		t.Fatalf("prepare = %q, %v; want the breakage TG-2 brings in as TG-1's one fix", fixes, err)
	}
}
