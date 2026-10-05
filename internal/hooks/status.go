package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"komodo/internal/conductor"
	"komodo/internal/gate"
	"komodo/internal/line"
	"komodo/internal/mount"
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
			status.State, status.TimeUsed = string(s.Current), s.TimeUsed+liveStage(root, run.Group, s)
			if s.Current == conductor.Escalated || s.Current == conductor.Blocked {
				status.Blocker = conductor.Next(s).Why
			}
		}
		groups = append(groups, status)
	}
	return groups
}

// liveStage is a live run's time in the group's current state; a settled or unheld group adds none.
func liveStage(root, group string, s conductor.State) time.Duration {
	if s.Current == conductor.Shipped || s.Current == conductor.Blocked || line.CheckLock(root, group) == nil {
		return 0
	}
	// The conductor saves state.json as it enters a state, so its modification time is when the stage began.
	info, err := os.Stat(conductor.StatePath(root, group))
	if err != nil {
		return 0
	}
	return max(time.Since(info.ModTime()), 0)
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

// StaleCheck names each file of a host's installed global layer whose bytes differ from what checkout's toolkit renders.
type StaleCheck func(checkout string) []string

// staleChecks are the registered hosts' global-layer checks, by mount name, under staleLock.
var (
	staleLock   sync.Mutex
	staleChecks = map[string]StaleCheck{}
)

// RegisterStaleCheck records how one mount finds the stale files of its installed global layer.
func RegisterStaleCheck(host string, check StaleCheck) {
	staleLock.Lock()
	defer staleLock.Unlock()
	staleChecks[host] = check
}

// binaryBuild reads a binary's version and commit; tests swap it.
var binaryBuild = mount.BinaryBuild

// Stale names what the machine layer holds that differs from checkout: the installed binary, when its commit is not
// the toolkit build's stamp, then each registered host's stale global files.
func Stale(checkout string) []string {
	var stale []string
	if stamp, err := os.ReadFile(filepath.Join(checkout, "bin", gate.BuiltFrom)); err == nil {
		if installed, err := mount.HookPath(); err == nil {
			_, commit := binaryBuild(installed)
			if commit == "" || !strings.HasPrefix(strings.TrimSpace(string(stamp)), commit) {
				stale = append(stale, "the installed binary "+installed)
			}
		}
	}
	staleLock.Lock()
	checks := maps.Clone(staleChecks)
	staleLock.Unlock()
	for _, host := range slices.Sorted(maps.Keys(checks)) {
		stale = append(stale, checks[host](checkout)...)
	}
	return stale
}

// StaleText is the one line naming everything stale, or empty when nothing is.
func StaleText(stale []string) string {
	if len(stale) == 0 {
		return ""
	}
	return "stale: " + strings.Join(stale, ", ") + "; run komodo sync\n"
}

// addStatus is the status hook: it adds the run's status, blocked groups, and a stale machine layer to a session.
func addStatus(_ context.Context, in Input) (Outcome, error) {
	text := StatusText(RunStatus(in.Root)) + StaleText(Stale(mount.MainCheckout(in.Root)))
	if text == "" {
		return Outcome{Verdict: Allow}, nil
	}
	return Outcome{Verdict: Inform, Message: text}, nil
}
