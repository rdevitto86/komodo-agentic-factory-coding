package conductor

import (
	"komodo/internal/backlog"
	"komodo/internal/mount"
	"komodo/internal/plan"
)

// Concurrency is how many groups the plan runs at once, from the active mount's own numbers.
func Concurrency(planName string) int {
	return mount.ConcurrencyFor(planName)
}

// Startable is the pending groups, in order, that may start beside the running ones, up to capacity.
// Each shares no file with a running or earlier pending group, and no group it depends on is unfinished.
func Startable(pending, running []backlog.Group, capacity int) []backlog.Group {
	capacity = max(capacity, 1)
	unfinished := map[string]bool{}
	for _, group := range append(append([]backlog.Group{}, running...), pending...) {
		unfinished[group.ID] = true
	}
	claimed := append([]backlog.Group{}, running...)
	lanes := len(running)
	var out []backlog.Group
	for _, group := range pending {
		if lanes >= capacity {
			break
		}
		waiting := waitsOnParent(group, unfinished)
		free := !waiting && !sharesFile(group, claimed)
		if !waiting {
			// A group held only by a file still claims it, so a later group never jumps the queue.
			claimed = append(claimed, group)
		}
		if free {
			out = append(out, group)
			lanes++
		}
	}
	return out
}

// waitsOnParent reports whether a group the given one depends on has not finished yet.
func waitsOnParent(group backlog.Group, unfinished map[string]bool) bool {
	for _, parent := range group.DependsOn() {
		if parent != group.ID && unfinished[parent] {
			return true
		}
	}
	return false
}

// sharesFile reports whether any task of the group claims a file some task of the others claims.
func sharesFile(group backlog.Group, others []backlog.Group) bool {
	for _, other := range others {
		for _, task := range group.Tasks {
			for _, claim := range other.Tasks {
				if plan.Overlap(task, claim) {
					return true
				}
			}
		}
	}
	return false
}
