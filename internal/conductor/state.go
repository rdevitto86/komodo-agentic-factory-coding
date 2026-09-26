// Package conductor decides each task group's next move from its state.json record alone.
package conductor

import "time"

// GroupState is one of the states a task group's state.json can hold (system-design.md#group-states).
type GroupState string

const (
	Ready     GroupState = "Ready"
	Building  GroupState = "Building"
	Checking  GroupState = "Checking"
	Reviewing GroupState = "Reviewing"
	Repairing GroupState = "Repairing"
	Preparing GroupState = "Preparing"
	Shipping  GroupState = "Shipping"
	Shipped   GroupState = "Shipped"
	Escalated GroupState = "Escalated"
	Blocked   GroupState = "Blocked"
)

// Finding is one open review finding carried in a group's state.json.
type Finding struct {
	Severity string `json:"severity"`
	Verified bool   `json:"verified"`
}

// State is what state.json holds for one group when Next reads it, before the state's work starts.
type State struct {
	Group    string        `json:"group"`
	Current  GroupState    `json:"state"`
	Worktree string        `json:"worktree"`
	Branch   string        `json:"branch"`
	LastWIP  string        `json:"last_wip"`
	Sessions []string      `json:"sessions"`
	Findings []Finding     `json:"findings"`
	TimeUsed time.Duration `json:"time_used"`
	// Fixes is the open fix list the next repair round works.
	Fixes []string `json:"fixes,omitempty"`
	// Builder is the builder session a repair round resumes.
	Builder string `json:"builder,omitempty"`
	// Repairs counts the repair rounds the group has spent, across every run that drove it.
	Repairs int `json:"repairs,omitempty"`

	// SlotFree is read at Ready: a build slot is free for the group to take.
	SlotFree bool `json:"slot_free"`
	// SessionDone is read at Building, Repairing and Preparing: the running session has ended.
	SessionDone bool `json:"session_done"`
	// ChecksPassed is read at Checking: every rerun check passed.
	ChecksPassed bool `json:"checks_passed"`
	// Conflict is read at Preparing: the rebase or the integration build failed.
	Conflict bool `json:"conflict"`
	// ShipDone is read at Shipping: the push, the draft PR and its labels are all done.
	ShipDone bool `json:"ship_done"`
	// Merged is read at Shipped: a person merged the group's PR.
	Merged bool `json:"merged"`
	// Escalate is read at every state but Escalated and Blocked: the group hit a stop it can't retry.
	Escalate bool `json:"escalate"`
	// Answered is read at Escalated: the orchestrator returned exactly one action for it.
	Answered bool `json:"answered"`
	// Stop is read at Escalated once Answered: the orchestrator's action was to stop the group.
	Stop bool `json:"stop"`
	// Left is read at Escalated once Answered and not Stop: the state the group left to escalate.
	Left GroupState `json:"left"`
	// Edited is read at Blocked: a person edited the group since it stopped.
	Edited bool `json:"edited"`
}

// Action is the one next move Next decides for a group.
type Action struct {
	Move   GroupState `json:"move"`
	Remove bool       `json:"remove,omitempty"`
	Why    string     `json:"why"`
}

// Next decides a group's next move from its state alone: no disk, no clock, no session.
func Next(s State) Action {
	if s.Escalate && s.Current != Escalated && s.Current != Blocked {
		return Action{Move: Escalated, Why: s.Group + " hit an escalation and is waiting on the orchestrator"}
	}
	switch s.Current {
	case Ready:
		return readyNext(s)
	case Building:
		return sessionNext(s, Building, Checking, s.Group+"'s builder session is still running",
			s.Group+"'s builder session ended; rerun every check")
	case Checking:
		return checkingNext(s)
	case Reviewing:
		return reviewingNext(s)
	case Repairing:
		return sessionNext(s, Repairing, Checking, s.Group+"'s resumed builder session is still running",
			s.Group+"'s fix list is done; rerun every check")
	case Preparing:
		return preparingNext(s)
	case Shipping:
		return shippingNext(s)
	case Shipped:
		return shippedNext(s)
	case Escalated:
		return escalatedNext(s)
	case Blocked:
		return blockedNext(s)
	default:
		return Action{Move: Ready, Why: s.Group + " has no recorded state; start from Ready"}
	}
}

// readyNext waits for a free slot, then moves to Building.
func readyNext(s State) Action {
	if !s.SlotFree {
		return Action{Move: Ready, Why: s.Group + " is waiting for a slot"}
	}
	return Action{Move: Building, Why: s.Group + " has a slot free and starts its builder session"}
}

// sessionNext stays at from until its session ends, then moves to to.
func sessionNext(s State, from, to GroupState, waiting, done string) Action {
	if !s.SessionDone {
		return Action{Move: from, Why: waiting}
	}
	return Action{Move: to, Why: done}
}

// checkingNext moves to Reviewing once every check passed, else back to Repairing.
func checkingNext(s State) Action {
	if !s.ChecksPassed {
		return Action{Move: Repairing, Why: s.Group + " failed a check and needs a fix list"}
	}
	return Action{Move: Reviewing, Why: s.Group + " passed every check; run the lenses"}
}

// reviewingNext moves to Repairing once a finding is verified, else to Preparing.
func reviewingNext(s State) Action {
	for _, finding := range s.Findings {
		if finding.Verified {
			return Action{Move: Repairing, Why: s.Group + " has a verified finding and needs a fix list"}
		}
	}
	return Action{Move: Preparing, Why: s.Group + " has no verified finding; commit and integrate"}
}

// preparingNext moves to Repairing on a conflict or an integration failure, else waits then ships.
func preparingNext(s State) Action {
	if s.Conflict {
		return Action{Move: Repairing, Why: s.Group + " hit a conflict or an integration failure"}
	}
	return sessionNext(s, Preparing, Shipping, s.Group+" is still committing, hooking, rebasing and integrating",
		s.Group+" is committed and integrated; push and open the draft PR")
}

// shippingNext waits for the push, the draft PR and its labels, then moves to Shipped.
func shippingNext(s State) Action {
	if !s.ShipDone {
		return Action{Move: Shipping, Why: s.Group + " is still pushing and labelling the draft PR"}
	}
	return Action{Move: Shipped, Why: s.Group + " is pushed with its draft PR open"}
}

// shippedNext removes the group's record once merged, else waits for a person to merge it.
func shippedNext(s State) Action {
	if s.Merged {
		return Action{Remove: true, Why: s.Group + " merged and is removed from state.json"}
	}
	return Action{Move: Shipped, Why: s.Group + " is waiting for a person to merge its PR"}
}

// escalatedNext waits on the orchestrator, then returns to the state it left, or stops it.
func escalatedNext(s State) Action {
	if !s.Answered {
		return Action{Move: Escalated, Why: s.Group + " is still waiting on the orchestrator"}
	}
	if s.Stop {
		return Action{Move: Blocked, Why: s.Group + " could not be settled and stops for a person"}
	}
	return Action{Move: s.Left, Why: s.Group + "'s escalation settled; back to " + string(s.Left)}
}

// blockedNext waits for a person's edit, then returns the group to Ready.
func blockedNext(s State) Action {
	if !s.Edited {
		return Action{Move: Blocked, Why: s.Group + " is stopped until a person edits it"}
	}
	return Action{Move: Ready, Why: s.Group + " was edited; wait for a slot again"}
}
