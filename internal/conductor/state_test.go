package conductor

import "testing"

func TestNextDecidesFromTheStateAlone(t *testing.T) {
	cases := []struct {
		name  string
		state State
		want  GroupState
	}{
		{"waiting for a slot", State{Group: "TG-1", Current: Ready}, Ready},
		{"slot free starts building", State{Group: "TG-1", Current: Ready, SlotFree: true}, Building},
		{"builder still running", State{Group: "TG-1", Current: Building}, Building},
		{"builder session ended", State{Group: "TG-1", Current: Building, SessionDone: true}, Checking},
		{"a check failed", State{Group: "TG-1", Current: Checking}, Repairing},
		{"every check passed", State{Group: "TG-1", Current: Checking, ChecksPassed: true}, Reviewing},
		{"a finding is verified", State{
			Group: "TG-1", Current: Reviewing, Findings: []Finding{{Severity: "high", Verified: true}},
		}, Repairing},
		{"no finding is verified", State{
			Group: "TG-1", Current: Reviewing, Findings: []Finding{{Severity: "high"}},
		}, Preparing},
		{"repair session still running", State{Group: "TG-1", Current: Repairing}, Repairing},
		{"repair session ended", State{Group: "TG-1", Current: Repairing, SessionDone: true}, Checking},
		{"prepare hit a conflict", State{Group: "TG-1", Current: Preparing, Conflict: true}, Repairing},
		{"prepare still running", State{Group: "TG-1", Current: Preparing}, Preparing},
		{"prepare finished", State{Group: "TG-1", Current: Preparing, SessionDone: true}, Shipping},
		{"ship still running", State{Group: "TG-1", Current: Shipping}, Shipping},
		{"ship finished", State{Group: "TG-1", Current: Shipping, ShipDone: true}, Shipped},
		{"shipped, not merged", State{Group: "TG-1", Current: Shipped}, Shipped},
		{"escalated, not yet answered", State{Group: "TG-1", Current: Escalated}, Escalated},
		{"escalated, orchestrator stops it", State{
			Group: "TG-1", Current: Escalated, Answered: true, Stop: true,
		}, Blocked},
		{"escalated, orchestrator settles it", State{
			Group: "TG-1", Current: Escalated, Answered: true, Left: Building,
		}, Building},
		{"blocked, not yet edited", State{Group: "TG-1", Current: Blocked}, Blocked},
		{"blocked, a person edited it", State{Group: "TG-1", Current: Blocked, Edited: true}, Ready},
		{"an escalation interrupts any running state", State{
			Group: "TG-1", Current: Building, Escalate: true,
		}, Escalated},
		{"an unrecognised state starts over from Ready", State{Group: "TG-1", Current: "Bogus"}, Ready},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next := Next(tc.state)
			if next.Move != tc.want {
				t.Fatalf("Next(%+v) = %+v, want move %s", tc.state, next, tc.want)
			}
		})
	}
}

func TestNextRemovesAMergedShippedGroup(t *testing.T) {
	next := Next(State{Group: "TG-1", Current: Shipped, Merged: true})
	if !next.Remove || next.Move != "" {
		t.Fatalf("Next = %+v, want a removed record with no next move", next)
	}
}

func TestNextNeverEscalatesAGroupAlreadyEscalatedOrBlocked(t *testing.T) {
	cases := []struct {
		current GroupState
		want    GroupState
	}{
		{Escalated, Escalated},
		{Blocked, Blocked},
	}
	for _, tc := range cases {
		next := Next(State{Group: "TG-1", Current: tc.current, Escalate: true})
		if next.Move != tc.want {
			t.Fatalf("Next(%s, escalate) = %+v, want it to stay at %s", tc.current, next, tc.want)
		}
	}
}
