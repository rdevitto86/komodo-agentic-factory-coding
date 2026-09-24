package line

import (
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
	"komodo/internal/profile"
)

// snapBacklog is a two-wave group: one and two run first, three depends on one.
const snapBacklog = "### [TG-20.1] A snapshot group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-20.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\n```\n\n" +
	"#### [TSK-20.1.2] Two [P: C] [READY]\n```yaml\nfiles: [b/two.go]\n```\n\n" +
	"#### [TSK-20.1.3] Three [P: C] [READY]\n```yaml\nfiles: [c/three.go]\ndepends_on: [TSK-20.1.1]\n```\n"

// snap builds a cut snapshot over snapBacklog with every task open and briefed, and no result.
func snap(t *testing.T) Snapshot {
	t.Helper()
	group, ok := backlog.Parse(snapBacklog).Group("TG-20.1")
	if !ok {
		t.Fatal("snapBacklog has no group")
	}
	plan := &Plan{
		Group: "TG-20.1", Base: "main", Branch: "feat/a-snapshot-group",
		Worktree: filepath.Join(StateDir, "wt", "TG-20.1"),
		Waves:    [][]string{{"TSK-20.1.1", "TSK-20.1.2"}, {"TSK-20.1.3"}},
		Profile:  profile.Profile{Repairs: 1, SeverityFloor: "high"},
	}
	tasks := map[string]TaskState{}
	for _, task := range group.Tasks {
		tasks[task.ID] = TaskState{Open: true}
	}
	return Snapshot{Plan: plan, Cut: true, Group: group, Tasks: tasks, Waves: make([]WaveState, 2)}
}

// closed marks every task closed and every wave merged, which puts the walk at the review.
func closed(s Snapshot) Snapshot {
	for id := range s.Tasks {
		s.Tasks[id] = TaskState{HasResult: true}
	}
	s.Waves = []WaveState{{Merged: true}, {Merged: true}}
	return s
}

func TestNextDecidesFromTheSnapshotAlone(t *testing.T) {
	cases := []struct {
		name    string
		edit    func(Snapshot) Snapshot
		action  string
		command string
		task    string
	}{
		{"nothing ready", func(s Snapshot) Snapshot { return Snapshot{} }, "done", "", ""},
		{"paused", func(s Snapshot) Snapshot {
			s.Paused = &Action{Action: "done", Until: "2026-09-24T00:00:00Z"}
			return s
		}, "done", "", ""},
		{"not cut", func(s Snapshot) Snapshot { s.Cut = false; return s }, "run", "komodo next --start TG-20.1 --base main", ""},
		{"no brief", func(s Snapshot) Snapshot {
			s.Tasks["TSK-20.1.1"] = TaskState{Open: true, StaleBrief: true}
			return s
		}, "run", "komodo brief TSK-20.1.1", "TSK-20.1.1"},
		{"briefed", func(s Snapshot) Snapshot {
			s.Tasks["TSK-20.1.2"] = TaskState{Open: true, HasResult: true}
			return s
		}, "spawn", "", "TSK-20.1.1"},
		{"a wave briefed together spawns together", func(s Snapshot) Snapshot { return s }, "spawn", "", ""},
		{"failed with a stale result spawns its repair", func(s Snapshot) Snapshot {
			s.Tasks["TSK-20.1.1"] = TaskState{Open: true, HasResult: true, Attempts: 1}
			s.Tasks["TSK-20.1.2"] = TaskState{Open: true, HasResult: true}
			return s
		}, "spawn", "", "TSK-20.1.1"},
		{"result in, still open", func(s Snapshot) Snapshot {
			s.Tasks["TSK-20.1.1"] = TaskState{Open: true, HasResult: true}
			s.Tasks["TSK-20.1.2"] = TaskState{HasResult: true}
			return s
		}, "run", "komodo close TSK-20.1.1 --gate", "TSK-20.1.1"},
		{"wave closed, not merged", func(s Snapshot) Snapshot {
			s = closed(s)
			s.Waves[0].Merged = false
			return s
		}, "run", "komodo close --wave 1", ""},
		{"blocked task skips its dependent", func(s Snapshot) Snapshot {
			s.Tasks["TSK-20.1.1"] = TaskState{HasResult: true, Attempts: 2}
			s.Tasks["TSK-20.1.2"] = TaskState{HasResult: true}
			return s
		}, "run", "komodo close --wave 1", ""},
		{"unreviewed", closed, "spawn", "", "TG-20.1-review"},
		{"review blocks", func(s Snapshot) Snapshot {
			s = closed(s)
			s.Reviewed = true
			s.Blocking = []Finding{{Severity: "high"}}
			return s
		}, "done", "", ""},
		{"handed off", func(s Snapshot) Snapshot {
			s = closed(s)
			s.ReviewSkippable, s.Handoff = true, true
			return s
		}, "done", "", ""},
		{"not shipped", func(s Snapshot) Snapshot {
			s = closed(s)
			s.Reviewed = true
			return s
		}, "run", "komodo close --group", ""},
		{"shipped", func(s Snapshot) Snapshot {
			s = closed(s)
			s.Reviewed, s.Shipped = true, true
			return s
		}, "done", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next := Next(tc.edit(snap(t)))
			if next.Action != tc.action || next.Command != tc.command || next.Task != tc.task {
				t.Fatalf("Next = %+v, want %s %q %s", next, tc.action, tc.command, tc.task)
			}
		})
	}
}

func TestNextSpawnsASingleModeTaskInTheGroupWorktree(t *testing.T) {
	s := snap(t)
	s.Group.Fields = singleFields()
	next := Next(s)
	if next.Action != "spawn" || next.Worktree != filepath.Join(StateDir, "wt", "TG-20.1") {
		t.Fatalf("Next = %+v", next)
	}
}

// FuzzNext proves Next never panics, answers run, spawn, or done, and never respawns a closeable task.
func FuzzNext(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	f.Add([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		bit := func(index int) bool { return index < len(data) && data[index]&1 == 1 }
		count := func(index int) int {
			if index < len(data) {
				return int(data[index] % 4)
			}
			return 0
		}
		s := snap(t)
		s.Cut = !bit(0)
		for index, id := range []string{"TSK-20.1.1", "TSK-20.1.2", "TSK-20.1.3"} {
			at := 1 + index*6
			s.Tasks[id] = TaskState{
				HasResult: bit(at), Attempts: count(at + 1), RepairResultReady: bit(at + 2),
				StaleBrief: bit(at + 3), Open: bit(at + 4),
			}
		}
		s.Waves = make([]WaveState, count(19)%3)
		for index := range s.Waves {
			s.Waves[index].Merged = bit(20 + index)
		}
		s.Reviewed, s.ReviewSkippable, s.Handoff, s.Shipped = bit(22), bit(23), bit(24), bit(25)
		if bit(26) {
			s.Blocking = []Finding{{Severity: "high"}}
		}
		if bit(27) {
			s.Group.Fields = singleFields()
		}
		next := Next(s)
		switch next.Action {
		case "run", "done":
		case "spawn":
			for _, spawn := range append([]Action{next}, next.Spawns...) {
				if spawn.Role == "builder" && s.Tasks[spawn.Task].Closeable() {
					t.Fatalf("Next spawned %s, whose result is ready to close", spawn.Task)
				}
			}
		default:
			t.Fatalf("Next answered %q", next.Action)
		}
	})
}

// singleFields is snapBacklog's group block with mode single.
func singleFields() backlog.Fields {
	return backlog.Parse(strings.Replace(snapBacklog, "version: 2.0.0\n", "version: 2.0.0\nmode: single\n", 1)).Groups[0].Fields
}
