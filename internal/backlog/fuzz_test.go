package backlog

import (
	"os"
	"testing"
)

// FuzzParse proves the task grammar parser and its lint never panic on any input.
func FuzzParse(f *testing.F) {
	if data, err := os.ReadFile("../../BACKLOG.md"); err == nil {
		f.Add(string(data))
	}
	f.Add("## TG-01.1 A group\n\n```yaml\nstatus: READY\n```\n\n### TSK-01.1.1 A task\n\n```yaml\nfiles: [a.go]\n```\n")
	f.Add("```yaml\nkey: [unterminated\n")
	f.Add(`the "guard" can't see it`)
	f.Fuzz(func(t *testing.T, text string) {
		parsed := Parse(text)
		_ = Lint(parsed)
		for _, group := range parsed.Groups {
			_ = NextTaskID(group)
		}
		_, _ = ParseFields(text)
		roundTrip(t, text)
	})
}

// FuzzParseGroupFile proves the group file grammar parser never panics on any input.
func FuzzParseGroupFile(f *testing.F) {
	f.Add(groupFileSample)
	f.Add("## [TG-01.1] A group [P: H] [READY]\n\n```yaml\nkey: [unterminated\n")
	f.Add("- [ ] **TSK-01.1.1** No group heading\n  - files: `a.go`\n")
	f.Add(`the "guard" can't see it`)
	f.Fuzz(func(t *testing.T, text string) {
		_ = ParseGroupFile(text)
	})
}

// roundTrip checks a value dumped as a scalar and a list item parses back unchanged, newlines collapsed.
func roundTrip(t *testing.T, text string) {
	t.Helper()
	want := newlineRe.ReplaceAllString(text, " ")
	var fields Fields
	fields.Set("value", text)
	fields.Set("items", []any{text})
	dumped := DumpFields(fields)
	again, err := ParseFields(dumped)
	if err != nil {
		t.Fatalf("dumped %q does not parse back: %v", dumped, err)
	}
	if got := again.String("value"); got != want {
		t.Fatalf("value = %q, want %q (dumped: %q)", got, want, dumped)
	}
	if got := again.List("items"); len(got) != 1 || got[0] != want {
		t.Fatalf("items = %q, want [%q] (dumped: %q)", got, want, dumped)
	}
}
