package conductor

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"komodo/internal/backlog"
)

// pushbackLimit is how many review-driven repair rounds a builder's retries get before it blocks.
const pushbackLimit = 3

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

// block moves a stuck group straight to Blocked: there is no orchestrator left to ask, so a
// builder's spent retries, or any other stop that cannot retry, writes its blocker note instead.
func block(s *State, r *round) {
	if r.repairs >= pushbackLimit {
		r.needs = fmt.Sprintf("the group spent its %d repair rounds; last: %s", pushbackLimit, r.reason)
	}
	s.Left = s.Current
	s.Blocking = true
}

// timeout saves a group whose own budget ran out straight to Blocked, its blocker note written on
// a fresh context, so the spent budget never cuts off the note.
func (d *Driver) timeout(s State, r *round, cause error) (State, error) {
	r.reason = fmt.Sprintf("%s ran out of its budget after %s", s.Group, s.TimeUsed.Round(time.Second))
	block(&s, r)
	s.Current = Blocked
	r.keep(&s)
	if err := d.saveState(&s); err != nil {
		return s, fmt.Errorf("saving %s at %s: %w", s.Group, s.Current, err)
	}
	if err := d.stop(context.Background(), &s, r); err != nil {
		return s, fmt.Errorf("%s stopped at %s: %w", s.Group, s.Current, err)
	}
	return s, cause
}

// reason is why the group blocked: the failure that stopped it, else each blocked task's question, else the
// blocked builder's own question or summary.
func (d *Driver) reason(s State, r *round) string {
	if r.reason != "" {
		return r.reason
	}
	result, err := d.Host.Result(lastSession(s))
	if err != nil {
		return "the session that stopped the group left no result: " + err.Error()
	}
	tasks, _ := result.Value["tasks"].([]any)
	var questions []string
	for _, each := range tasks {
		task, _ := each.(map[string]any)
		id, _ := task["task"].(string)
		question, _ := task["question"].(string)
		if task["result"] == resultBlocked && question != "" {
			questions = append(questions, id+": "+question)
		}
	}
	if len(questions) > 0 {
		return strings.Join(questions, "; ")
	}
	for _, key := range []string{"question", "summary"} {
		if text, ok := result.Value[key].(string); ok && text != "" {
			return text
		}
	}
	return "the builder returned BLOCKED with no question"
}
