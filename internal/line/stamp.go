package line

import (
	"path/filepath"
	"time"

	"komodo/internal/ledger"
)

// Book returns the ledger for one repo.
func Book(root string) *ledger.Ledger { return ledger.New(filepath.Join(root, StateDir)) }

// Stamp records one station event, filling the run and group from the run state.
func Stamp(root string, entry ledger.Entry) {
	if entry.Run == "" {
		if state, err := LoadRun(root); err == nil {
			entry.Run, entry.Group = state.Run, state.Group
		}
	}
	_ = Book(root).Stamp(entry)
}

// Since is the seconds elapsed from a station's start, for the ledger.
func Since(started time.Time) float64 { return time.Since(started).Seconds() }

// FailureClass names which check rejected a task, for the metrics.
func FailureClass(problems []string) string {
	for _, problem := range problems {
		switch {
		case len(problem) >= 6 && problem[:6] == "result":
			return "schema"
		case len(problem) >= 6 && problem[:6] == "schema":
			return "schema"
		case len(problem) >= 9 && problem[:9] == "done_when":
			return "done_when"
		case len(problem) >= 4 && problem[:4] == "gate":
			return "gate"
		}
	}
	if len(problems) > 0 {
		return "comments"
	}
	return ""
}
