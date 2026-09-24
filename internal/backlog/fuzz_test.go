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
	f.Fuzz(func(t *testing.T, text string) {
		parsed := Parse(text)
		_ = Lint(parsed)
		for _, group := range parsed.Groups {
			_ = NextTaskID(group)
		}
		_, _ = ParseFields(text)
	})
}
