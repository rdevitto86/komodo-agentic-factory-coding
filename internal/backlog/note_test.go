package backlog

import (
	"strings"
	"testing"
	"time"
)

const notedBacklog = "# Backlog\n\n### [TG-04.1] Tokens report their expiry\n```yaml\ntype: feat\n```\n" +
	"* **Why:** tokens expire.\n\n" +
	"#### [TSK-04.1.1] Tokens know when they expire [P: H] [DONE]\n```yaml\nfiles: [a.go]\n```\n\n" +
	"#### [TSK-04.1.2] An expired token gets a 401 [P: H] [READY]\n```yaml\nfiles: [b.go]\n```\n\n" +
	"### [TG-04.2] Another group\n"

// sampleNote is the note the stopped group in notedBacklog carries.
var sampleNote = BlockerNote{
	At: time.Date(2026, 9, 25, 14, 2, 0, 0, time.UTC), Run: "r-0142", State: "Reviewing",
	Items: []string{"TSK-04.1.2: the reproducer needs a clock"}, Needs: "a decision on a clock parameter",
	Saved: "WIP commit `3f2a9c1` on `feat/tg-04-1`",
}

const notedGroupFile = "## [TG-04.1] Tokens report their expiry [P: H] [READY]\n\n" +
	"```yaml\ntype: feat\nversion: 1.4.0\nepic: EPIC-04\ndepends_on: []\n```\n\n" +
	"- [x] **TSK-04.1.1** Tokens know when they expire\n  - files: `internal/token/`\n" +
	"- [ ] **TSK-04.1.2** An expired token gets a 401\n  - files: `internal/api/auth.go`\n"

// TestAddGroupFileNoteWritesTheNoteAndBlocksTheHeading proves a group file's note lands under its
// heading and yaml block, and the heading's own status flips to BLOCKED.
func TestAddGroupFileNoteWritesTheNoteAndBlocksTheHeading(t *testing.T) {
	out, err := AddGroupFileNote(notedGroupFile, sampleNote)
	if err != nil {
		t.Fatalf("add note = %v", err)
	}
	want := "```\n\n> **Blocked** 2026-09-25 14:02, run r-0142, at Reviewing.\n" +
		"> - TSK-04.1.2: the reproducer needs a clock\n> - Needs: a decision on a clock parameter\n" +
		"> - Saved: WIP commit `3f2a9c1` on `feat/tg-04-1`\n\n\n- [x] **TSK-04.1.1**"
	if !strings.Contains(out, want) {
		t.Fatalf("group file =\n%s\nwant the note under the heading and yaml block", out)
	}
	if !strings.Contains(out, "[TG-04.1] Tokens report their expiry [P: H] [BLOCKED]") {
		t.Fatalf("group file =\n%s\nwant the heading status flipped to BLOCKED", out)
	}
	file := ParseGroupFile(out)
	if len(file.Problems) != 0 || len(file.Tasks) != 2 {
		t.Fatalf("problems %v, tasks %+v; the note must leave the grammar intact", file.Problems, file.Tasks)
	}
}

// TestAddGroupFileNoteReplacesAnEarlierNote proves a second note replaces the first, one note only.
func TestAddGroupFileNoteReplacesAnEarlierNote(t *testing.T) {
	first, err := AddGroupFileNote(notedGroupFile, sampleNote)
	if err != nil {
		t.Fatal(err)
	}
	again := sampleNote
	again.Run = "r-0143"
	out, err := AddGroupFileNote(first, again)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "> **Blocked**") != 1 || !strings.Contains(out, "run r-0143") {
		t.Fatalf("group file =\n%s\nwant the one latest note", out)
	}
}

// TestRemoveGroupFileNoteRestoresTheGroupAsItWas proves removing a note restores every byte but the
// heading status, which stays as the note left it.
func TestRemoveGroupFileNoteRestoresTheGroupAsItWas(t *testing.T) {
	noted, err := AddGroupFileNote(notedGroupFile, sampleNote)
	if err != nil {
		t.Fatal(err)
	}
	out, removed := RemoveGroupFileNote(noted)
	want := strings.Replace(notedGroupFile, "[P: H] [READY]", "[P: H] [BLOCKED]", 1)
	if !removed || out != want {
		t.Fatalf("remove = %v,\n%s\nwant the text before the note", removed, out)
	}
	if _, removed := RemoveGroupFileNote(notedGroupFile); removed {
		t.Fatal("remove reported a note on a group file that has none")
	}
}

// TestFindGroupFileLocatesTheFileNamingTheGroup proves FindGroupFile reads every file under
// docs/backlog until it finds the one whose heading names groupID.
func TestFindGroupFileLocatesTheFileNamingTheGroup(t *testing.T) {
	root := t.TempDir()
	dir := root + "/docs/backlog"
	writeFile(t, dir+"/TG-01.1-a.md", "## [TG-01.1] A [P: H] [READY]\n\n```yaml\ntype: fix\nversion: 1.0.0\n```\n")
	writeFile(t, dir+"/TG-01.2-b.md", "## [TG-01.2] B [P: H] [READY]\n\n```yaml\ntype: fix\nversion: 1.0.0\n```\n")
	path, text, found, err := FindGroupFile(root, "TG-01.2")
	if err != nil || !found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
	if !strings.HasSuffix(path, "TG-01.2-b.md") || !strings.Contains(text, "[TG-01.2]") {
		t.Fatalf("path = %q, text = %q", path, text)
	}
	if _, _, found, err := FindGroupFile(root, "TG-09.9"); err != nil || found {
		t.Fatalf("found = %v, err = %v, want none for an unknown group", found, err)
	}
}

