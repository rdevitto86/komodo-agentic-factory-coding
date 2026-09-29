package conductor

import (
	"fmt"
	"strings"
	"testing"

	"komodo/internal/backlog"
	"komodo/internal/mount"
)

// scheduleGroup is one group of a test backlog: its id, its one task's files, and the groups it depends on.
type scheduleGroup struct {
	id, files, depends string
}

// scheduleGroups parses the groups into a backlog and returns them in order.
func scheduleGroups(t *testing.T, groups ...scheduleGroup) []backlog.Group {
	t.Helper()
	var text strings.Builder
	for index, group := range groups {
		fmt.Fprintf(&text, "### [%s] Group %d\n```yaml\ntype: feat\nversion: 1.0.0\n", group.id, index)
		if group.depends != "" {
			fmt.Fprintf(&text, "depends_on: [%s]\n", group.depends)
		}
		fmt.Fprintf(&text, "```\n\n#### [TSK-%d.1] Task [P: C] [READY]\n```yaml\nfiles: [%s]\n```\n\n", index+1, group.files)
	}
	parsed := backlog.Parse(text.String()).Groups
	if len(parsed) != len(groups) {
		t.Fatalf("parsed %d groups, want %d", len(parsed), len(groups))
	}
	return parsed
}

// ids names the groups in order.
func ids(groups []backlog.Group) string {
	var out []string
	for _, group := range groups {
		out = append(out, group.ID)
	}
	return strings.Join(out, " ")
}

func TestConcurrencyReadsTheActiveMountsOwnNumbers(t *testing.T) {
	snapshot := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(snapshot) })
	mount.Restore(map[string]mount.Host{})
	mount.Register(mount.Host{Name: "fake", Concurrency: func(lane string) int {
		if lane == "big" {
			return 6
		}
		return 1
	}})
	if got := Concurrency("big"); got != 6 {
		t.Errorf("Concurrency(%q) = %d, want 6", "big", got)
	}
	if got := Concurrency("small"); got != 1 {
		t.Errorf("Concurrency(%q) = %d, want 1", "small", got)
	}
}

func TestConcurrencyWithNoActiveMountRunsOneLane(t *testing.T) {
	snapshot := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(snapshot) })
	mount.Restore(map[string]mount.Host{})
	if got := Concurrency("anything"); got != 1 {
		t.Errorf("Concurrency(%q) = %d, want 1", "anything", got)
	}
}

func TestStartableRunsGroupsSharingNoFileTogetherAndQueuesOnesThatDo(t *testing.T) {
	cases := []struct {
		name     string
		pending  []scheduleGroup
		running  []scheduleGroup
		capacity int
		want     string
	}{
		{
			name:     "groups sharing no file overlap",
			pending:  []scheduleGroup{{"TG-1.1", "a/one.go", ""}, {"TG-1.2", "b/two.go", ""}},
			capacity: 4,
			want:     "TG-1.1 TG-1.2",
		},
		{
			name:     "groups sharing a file run one after another",
			pending:  []scheduleGroup{{"TG-1.1", "a/one.go", ""}, {"TG-1.2", "a/one.go", ""}},
			capacity: 4,
			want:     "TG-1.1",
		},
		{
			name:     "a group sharing a running group's file waits",
			pending:  []scheduleGroup{{"TG-1.2", "a/one_test.go", ""}, {"TG-1.3", "c/three.go", ""}},
			running:  []scheduleGroup{{"TG-1.1", "a/one.go", ""}},
			capacity: 4,
			want:     "TG-1.3",
		},
		{
			name: "a later group never jumps an earlier one's file",
			pending: []scheduleGroup{
				{"TG-1.2", "a/one.go, b/two.go", ""}, {"TG-1.3", "b/two.go", ""}, {"TG-1.4", "d/four.go", ""},
			},
			running:  []scheduleGroup{{"TG-1.1", "a/one.go", ""}},
			capacity: 4,
			want:     "TG-1.4",
		},
		{
			name:     "the plan's concurrency caps the lanes",
			pending:  []scheduleGroup{{"TG-1.2", "b/two.go", ""}, {"TG-1.3", "c/three.go", ""}},
			running:  []scheduleGroup{{"TG-1.1", "a/one.go", ""}},
			capacity: 2,
			want:     "TG-1.2",
		},
		{
			name:     "a full plan starts nothing",
			pending:  []scheduleGroup{{"TG-1.2", "b/two.go", ""}},
			running:  []scheduleGroup{{"TG-1.1", "a/one.go", ""}},
			capacity: 1,
			want:     "",
		},
		{
			name:     "a group waits for its running parent",
			pending:  []scheduleGroup{{"TG-1.2", "b/two.go", "TG-1.1"}},
			running:  []scheduleGroup{{"TG-1.1", "a/one.go", ""}},
			capacity: 4,
			want:     "",
		},
		{
			name:     "a group waits for its pending parent",
			pending:  []scheduleGroup{{"TG-1.1", "a/one.go", ""}, {"TG-1.2", "b/two.go", "TG-1.1"}},
			capacity: 4,
			want:     "TG-1.1",
		},
		{
			name:     "a group whose parent is done starts",
			pending:  []scheduleGroup{{"TG-1.2", "b/two.go", "TG-1.1"}},
			capacity: 4,
			want:     "TG-1.2",
		},
		{
			name:     "no capacity still runs one lane",
			pending:  []scheduleGroup{{"TG-1.1", "a/one.go", ""}, {"TG-1.2", "b/two.go", ""}},
			capacity: 0,
			want:     "TG-1.1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			all := scheduleGroups(t, append(append([]scheduleGroup{}, tc.running...), tc.pending...)...)
			running, pending := all[:len(tc.running)], all[len(tc.running):]
			if got := ids(Startable(pending, running, tc.capacity)); got != tc.want {
				t.Fatalf("startable = %q, want %q", got, tc.want)
			}
		})
	}
}
