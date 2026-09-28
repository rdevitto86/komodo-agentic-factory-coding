package conductor

import (
	"fmt"
	"slices"

	"komodo/internal/backlog"
)

// stallLimit is how many stops without progress block a group, whatever the orchestrator would say.
const stallLimit = 2

// HeldGroup is a pending group held back because a group it depends on stopped.
type HeldGroup struct {
	Group backlog.Group
	On    string
}

// Hold splits pending groups, in order, into those free to take a lane and those that depend, directly
// or through another held group, on a stopped group.
func Hold(pending []backlog.Group, stopped []string) (free []backlog.Group, held []HeldGroup) {
	waitsOn := make(map[string]string, len(stopped))
	for _, id := range stopped {
		waitsOn[id] = id
	}
	// A group can depend on one listed after it, so the walk repeats until no group joins the held set.
	for changed := true; changed; {
		changed = false
		for _, group := range pending {
			if _, ok := waitsOn[group.ID]; ok {
				continue
			}
			for _, parent := range group.DependsOn() {
				if root, ok := waitsOn[parent]; ok && parent != group.ID {
					waitsOn[group.ID], changed = root, true
					break
				}
			}
		}
	}
	for _, group := range pending {
		root, ok := waitsOn[group.ID]
		switch {
		case slices.Contains(stopped, group.ID):
		case ok:
			held = append(held, HeldGroup{Group: group, On: root})
		default:
			free = append(free, group)
		}
	}
	return free, held
}

// stalled blocks a group that stopped stallLimit times without progress, before any orchestrator is asked.
func stalled(s *State, r *round) bool {
	if r.stalls < stallLimit {
		return false
	}
	s.Answered, s.Stop = true, true
	r.needs = fmt.Sprintf("the group stopped %d times without progress; last: %s", r.stalls, r.reason)
	return true
}
