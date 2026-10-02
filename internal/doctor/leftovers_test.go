package doctor

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog/backlogtest"
	"komodo/internal/line"
)

// epicGroupFile renders one docs/backlog group file of epic whose one task is ticked when done.
func epicGroupFile(id, epic string, done bool) string {
	box := " "
	if done {
		box = "x"
	}
	return "## [" + id + "] A group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 0.1.0\nepic: " + epic + "\n```\n\n" +
		"- [" + box + "] **TSK-" + strings.TrimPrefix(id, "TG-") + ".1** Do it\n  - files: `a.go`\n"
}

// leftoverRepo builds a root on main whose backlog holds one open group, and returns a git runner.
func leftoverRepo(t *testing.T) (string, func(dir string, args ...string)) {
	t.Helper()
	root := gitRepo(t)
	backlogtest.SeedText(t, root, openBacklog)
	write(t, root, ".gitignore", "/.komodo/\n")
	commitAll(t, root, "init")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return root, run
}

func TestDoctorNamesAnEndedEpicsFiles(t *testing.T) {
	root, _ := leftoverRepo(t)
	write(t, root, "docs/backlog/TG-02.1-a.md", epicGroupFile("TG-02.1", "EPIC-02", true))
	write(t, root, "docs/backlog/TG-02.2-b.md", epicGroupFile("TG-02.2", "EPIC-02", true))
	write(t, root, "docs/backlog/TG-03.1-c.md", epicGroupFile("TG-03.1", "EPIC-03", false))
	var notes []string
	if _, err := Run(root, Options{Warn: func(note string) { notes = append(notes, note) }}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(notes, "\n")
	for _, name := range []string{"TG-02.1-a.md", "TG-02.2-b.md"} {
		if !strings.Contains(joined, filepath.Join("docs", "backlog", name)+": EPIC-02 has ended") {
			t.Fatalf("notes = %v, want %s named as an ended epic's file", notes, name)
		}
	}
	if strings.Contains(joined, "EPIC-03") {
		t.Fatalf("notes = %v; an epic with an open task has not ended", notes)
	}
}

func TestDoctorNamesAStaleLocalJSONBaseKey(t *testing.T) {
	root, _ := leftoverRepo(t)
	write(t, root, ".komodo/local.json", `{"base":"main"}`)
	notes := Leftovers(root)
	if len(notes) != 1 || !strings.Contains(notes[0], ".komodo/local.json") || !strings.Contains(notes[0], "base key") {
		t.Fatalf("notes = %v, want one note naming the stale base key", notes)
	}
}

func TestDoctorIgnoresALocalJSONWithNoBaseKey(t *testing.T) {
	root, _ := leftoverRepo(t)
	write(t, root, ".komodo/local.json", `{"sandbox":true}`)
	if notes := Leftovers(root); len(notes) != 0 {
		t.Fatalf("notes = %v, want none for a local.json with no base key", notes)
	}
}

func TestDoctorNamesAWorktreeNoGroupOwns(t *testing.T) {
	root, run := leftoverRepo(t)
	write(t, root, "docs/backlog/TG-03.1-c.md", epicGroupFile("TG-03.1", "EPIC-03", false))
	owned := filepath.Join(root, ".komodo", "wt", "TG-01.1")
	byFile := filepath.Join(root, ".komodo", "wt", "TG-03.1")
	orphan := filepath.Join(root, ".komodo", "wt", "TG-09.9")
	run(root, "worktree", "add", "-q", "-b", "feat/g", owned, "main")
	run(root, "worktree", "add", "-q", "-b", "feat/c", byFile, "main")
	run(root, "worktree", "add", "-q", "-b", "feat/old", orphan, "main")

	notes := Leftovers(root)
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "TG-09.9") || !strings.Contains(joined, "feat/old") {
		t.Fatalf("notes = %v, want a note naming the TG-09.9 worktree and its branch", notes)
	}
}

func TestDoctorNamesARunsBranchWhoseGroupIsNoLongerOpen(t *testing.T) {
	root, run := leftoverRepo(t)
	run(root, "branch", "feat/g")
	run(root, "branch", "feat/old")
	for _, state := range []line.RunState{
		{Run: "r1", Group: "TG-01.1", Base: "main", Branch: "feat/g"},
		{Run: "r2", Group: "TG-09.9", Base: "main", Branch: "feat/old"},
		{Run: "r3", Group: "TG-09.8", Base: "main", Branch: "feat/deleted"},
	} {
		if err := line.SaveRun(root, state); err != nil {
			t.Fatal(err)
		}
	}

	notes := Leftovers(root)
	if len(notes) != 1 || !strings.Contains(notes[0], "branch feat/old") || !strings.Contains(notes[0], "TG-09.9") {
		t.Fatalf("notes = %v, want one note naming feat/old and its closed group", notes)
	}
}

func TestDoctorNamesADetachedWorktreeNoGroupOwnsByItsTrackedBranch(t *testing.T) {
	root, _ := leftoverRepo(t)
	orphan := filepath.Join(root, ".komodo", "wt", "TG-09.9")
	if err := line.AddDetached(root, "feat/old", "main", orphan); err != nil {
		t.Fatal(err)
	}
	notes := Leftovers(root)
	if len(notes) != 1 || !strings.Contains(notes[0], "TG-09.9") || !strings.Contains(notes[0], "on branch feat/old") {
		t.Fatalf("notes = %v, want one note naming the detached worktree's tracked branch", notes)
	}
}

func TestDoctorNamesARunsTipRefWhoseGroupIsNoLongerOpen(t *testing.T) {
	root, run := leftoverRepo(t)
	run(root, "update-ref", line.TipRef("feat/old"), "main")
	if err := line.SaveRun(root, line.RunState{Run: "r2", Group: "TG-09.9", Base: "main", Branch: "feat/old"}); err != nil {
		t.Fatal(err)
	}
	notes := Leftovers(root)
	if len(notes) != 1 || !strings.Contains(notes[0], "branch feat/old") {
		t.Fatalf("notes = %v, want one note naming the tip ref's branch", notes)
	}
}

func TestStrayWorktreesNamesADetachedOneOutsideButNotInside(t *testing.T) {
	root, _ := leftoverRepo(t)
	outside := filepath.Join(t.TempDir(), "elsewhere")
	if err := line.AddDetached(root, "feat/out", "main", outside); err != nil {
		t.Fatal(err)
	}
	if err := line.AddDetached(root, "feat/in", "main", filepath.Join(root, ".komodo", "wt", "TG-01.1")); err != nil {
		t.Fatal(err)
	}
	notes := StrayWorktrees(root)
	if len(notes) != 1 || !strings.Contains(notes[0], "on branch feat/out") {
		t.Fatalf("notes = %v, want one note for the detached worktree outside", notes)
	}
}
