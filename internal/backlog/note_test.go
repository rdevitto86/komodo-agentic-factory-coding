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

func TestAddNoteWritesTheNoteUnderTheGroupAndBlocksItsOpenTasks(t *testing.T) {
	out, err := AddNote(notedBacklog, "TG-04.1", sampleNote)
	if err != nil {
		t.Fatalf("add note = %v", err)
	}
	want := "```\n\n> **Blocked** 2026-09-25 14:02, run r-0142, at Reviewing.\n" +
		"> - TSK-04.1.2: the reproducer needs a clock\n> - Needs: a decision on a clock parameter\n" +
		"> - Saved: WIP commit `3f2a9c1` on `feat/tg-04-1`\n\n* **Why:**"
	if !strings.Contains(out, want) {
		t.Fatalf("backlog =\n%s\nwant the note under the group's heading and yaml block", out)
	}
	parsed := Parse(out)
	group, _ := parsed.Group("TG-04.1")
	if len(parsed.Problems) > 0 || group.Type() != "feat" || len(group.Tasks) != 2 {
		t.Fatalf("problems %v, group %+v; the note must leave the grammar intact", parsed.Problems, group)
	}
	for id, status := range map[string]string{"TSK-04.1.1": "DONE", "TSK-04.1.2": "BLOCKED"} {
		if task, _ := parsed.Task(id); task.Status != status {
			t.Fatalf("%s = %s, want %s", id, task.Status, status)
		}
	}
}

func TestAddNoteReplacesAnEarlierNote(t *testing.T) {
	first, err := AddNote(notedBacklog, "TG-04.1", sampleNote)
	if err != nil {
		t.Fatal(err)
	}
	again := sampleNote
	again.Run = "r-0143"
	out, err := AddNote(first, "TG-04.1", again)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "> **Blocked**") != 1 || !strings.Contains(out, "run r-0143") {
		t.Fatalf("backlog =\n%s\nwant the one latest note", out)
	}
}

func TestRemoveNoteRestoresTheGroupAsItWas(t *testing.T) {
	noted, err := AddNote(notedBacklog, "TG-04.1", sampleNote)
	if err != nil {
		t.Fatal(err)
	}
	out, removed := RemoveNote(noted, "TG-04.1")
	want := strings.Replace(notedBacklog, "[P: H] [READY]", "[P: H] [BLOCKED]", 1)
	if !removed || out != want {
		t.Fatalf("remove = %v,\n%s\nwant the text before the note", removed, out)
	}
	if _, removed := RemoveNote(notedBacklog, "TG-04.1"); removed {
		t.Fatal("remove reported a note on a group that has none")
	}
}

func TestAddNoteRefusesAMissingGroup(t *testing.T) {
	if _, err := AddNote(notedBacklog, "TG-09.9", sampleNote); err == nil {
		t.Fatal("add note = nil, want an error for a group the backlog lacks")
	}
}

func TestGroupTextIsTheGroupsWholeSection(t *testing.T) {
	text, ok := GroupText(notedBacklog, "TG-04.1")
	if !ok || !strings.HasPrefix(text, "### [TG-04.1]") || !strings.Contains(text, "TSK-04.1.2") ||
		strings.Contains(text, "TG-04.2") {
		t.Fatalf("group text = %q, want TG-04.1's section alone", text)
	}
	if _, ok := GroupText(notedBacklog, "TG-09.9"); ok {
		t.Fatal("group text found for a group the backlog lacks")
	}
}

func TestAddNoteGoesUnderAHeadingWithNoYamlBlock(t *testing.T) {
	out, err := AddNote(notedBacklog, "TG-04.2", sampleNote)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out, "### [TG-04.2] Another group\n\n> **Blocked** 2026-09-25 14:02, run r-0142, at Reviewing.\n"+
		"> - TSK-04.1.2: the reproducer needs a clock\n> - Needs: a decision on a clock parameter\n"+
		"> - Saved: WIP commit `3f2a9c1` on `feat/tg-04-1`\n\n") {
		t.Fatalf("backlog =\n%s\nwant the note right under the bare heading", out)
	}
}
