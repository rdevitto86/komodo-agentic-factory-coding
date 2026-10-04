package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"komodo/internal/conductor"
	"komodo/internal/line"
)

// stateOpen names a recorded run whose group has no state.json, so no conductor has driven it yet.
const stateOpen = "open"

// GroupStatus is one group's line in the run's status: its state, the time it has used, and what blocks it.
type GroupStatus struct {
	Group    string        `json:"group"`
	State    string        `json:"state"`
	TimeUsed time.Duration `json:"-"`
	Blocker  string        `json:"blocker,omitempty"`
}

// MarshalJSON names TimeUsed in whole seconds under its own key, so a reader is never off by a
// nanosecond's billion reading it as a unit the key never named.
func (g GroupStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Group           string `json:"group"`
		State           string `json:"state"`
		TimeUsedSeconds int64  `json:"time_used_seconds"`
		Blocker         string `json:"blocker,omitempty"`
	}{g.Group, g.State, int64(g.TimeUsed.Seconds()), g.Blocker})
}

// RunStatus reads every group a run has recorded under root, oldest first, with its saved state.
func RunStatus(root string) []GroupStatus {
	runs := line.LoadRuns(root)
	groups := make([]GroupStatus, 0, len(runs))
	for _, run := range runs {
		status := GroupStatus{Group: run.Group, State: stateOpen}
		if s, err := conductor.LoadState(conductor.StatePath(root, run.Group)); err == nil {
			status.State, status.TimeUsed = string(s.Current), s.TimeUsed
			if s.Current == conductor.Escalated || s.Current == conductor.Blocked {
				status.Blocker = conductor.Next(s).Why
			}
		}
		groups = append(groups, status)
	}
	return groups
}

// StatusText renders the groups by state, one line each, then every blocker; empty when no run is recorded.
func StatusText(groups []GroupStatus) string {
	if len(groups) == 0 {
		return ""
	}
	var out strings.Builder
	var blockers []string
	out.WriteString("The line's run:\n")
	for _, group := range groups {
		fmt.Fprintf(&out, "- %s: %s, %s used\n", group.Group, group.State, group.TimeUsed.Round(time.Second))
		if group.Blocker != "" {
			blockers = append(blockers, fmt.Sprintf("- %s: %s", group.Group, group.Blocker))
		}
	}
	if len(blockers) > 0 {
		out.WriteString("Blocked, waiting on you:\n" + strings.Join(blockers, "\n") + "\n")
	}
	return out.String()
}

// addStatus is the status hook: it adds the run's status and any blocked groups to a session as it starts.
func addStatus(_ context.Context, in Input) (Outcome, error) {
	text := StatusText(RunStatus(in.Root))
	if text == "" {
		return Outcome{Verdict: Allow}, nil
	}
	return Outcome{Verdict: Inform, Message: text}, nil
}
