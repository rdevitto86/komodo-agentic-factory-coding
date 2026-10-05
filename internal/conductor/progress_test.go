package conductor

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"komodo/internal/review"
)

// TestDriveNeverHoldsTheSameOpenFindingsForTwoRounds proves a reviewer raising the same finding
// after its repair ends the loop, so no two repair rounds work the same open findings.
func TestDriveNeverHoldsTheSameOpenFindingsForTwoRounds(t *testing.T) {
	r := newRig(t)
	stuck := map[string]any{"severity": "high", "file": "a.go", "line": 3, "title": "nil map", "fix": "make it"}
	r.host.reviews = []map[string]any{
		{"findings": []any{stuck}}, {"findings": []any{stuck}}, {"findings": []any{stuck}},
	}
	final, err := r.drive(t)
	if !errors.Is(err, errNoProgress) || !strings.Contains(err.Error(), "a.go:3 high nil map") {
		t.Fatalf("drive error = %v, want no progress naming the open finding", err)
	}
	if final.Current != Escalated || final.Left != Reviewing {
		t.Fatalf("final = %s left %s, want Escalated from Reviewing", final.Current, final.Left)
	}
	if open := final.Open(review.Economy); len(open) != 1 || open[0].File != "a.go" {
		t.Fatalf("open findings = %+v, want the one the group escalates with", open)
	}
	var rounds [][]string
	for _, s := range *r.saved {
		if s.Current != Repairing {
			continue
		}
		var open []string
		for _, finding := range openFindings(s, r.driver.lenses()) {
			open = append(open, finding.File+":"+finding.Title)
		}
		if !slices.ContainsFunc(rounds, func(seen []string) bool { return slices.Equal(seen, open) }) {
			rounds = append(rounds, open)
		}
	}
	if len(rounds) != 1 || len(r.host.inputs) != 1 {
		t.Fatalf("repair rounds held %v with fix lists %q, want one round before the loop ends", rounds, r.host.inputs)
	}
}

func TestDriveStopsALoopThatStopsProgressing(t *testing.T) {
	cases := []struct {
		name     string
		wire     func(r *rig)
		left     GroupState
		why      string
		sessions []string
	}{
		{
			name: "a repair that changed no file",
			wire: func(r *rig) {
				r.stations.idle = true
				r.stations.checks = [][]string{{"`go vet` exited 1"}}
			},
			left: Repairing, why: "changed no file",
			sessions: []string{StationBuild, StationRepair},
		},
		{
			name: "a check failing identically after a repair",
			wire: func(r *rig) {
				r.stations.checks = [][]string{{"`go vet` exited 1"}, {"`go vet` exited 1"}}
			},
			left: Checking, why: "fail identically",
			sessions: []string{StationBuild, StationRepair},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t)
			tc.wire(r)
			final, err := r.drive(t)
			if !errors.Is(err, errNoProgress) || !strings.Contains(err.Error(), tc.why) {
				t.Fatalf("drive error = %v, want no progress because it %s", err, tc.why)
			}
			if final.Current != Escalated || final.Left != tc.left {
				t.Fatalf("final = %s left %s, want Escalated from %s", final.Current, final.Left, tc.left)
			}
			if got := r.sessions(t); !equal(got, tc.sessions) {
				t.Fatalf("ledger sessions = %v, want %v and no further repair", got, tc.sessions)
			}
		})
	}
}

// TestDriveEscalatesABlockedRepairAndWaits escalates a repair whose builder reports BLOCKED,
// the Repairing counterpart of TestDriveEscalatesABlockedBuilderAndWaits.
func TestDriveEscalatesABlockedRepairAndWaits(t *testing.T) {
	r := newRig(t)
	r.stations.checks = [][]string{{"`go vet` exited 1"}}
	r.host.repairs = []map[string]any{{"result": "BLOCKED"}}
	final, err := r.drive(t)
	if err != nil || final.Current != Escalated || final.Left != Repairing {
		t.Fatalf("drive = %s left %s, %v; want Escalated from Repairing", final.Current, final.Left, err)
	}
	if got := r.sessions(t); !equal(got, []string{StationBuild, StationRepair}) {
		t.Fatalf("ledger sessions = %v, want one build and the one blocked repair", got)
	}
}

func TestDriveKeepsRepairingWhileEachRoundClosesAFinding(t *testing.T) {
	r := newRig(t)
	finding := func(file string) map[string]any {
		return map[string]any{"severity": "high", "file": file, "line": 3, "title": "wrong", "fix": "fix it"}
	}
	r.host.reviews = []map[string]any{
		{"findings": []any{finding("a.go"), finding("b.go")}},
		{"findings": []any{finding("a.go")}},
	}
	final, err := r.drive(t)
	if err != nil || final.Current != Shipped {
		t.Fatalf("drive = %s, %v; want Shipped", final.Current, err)
	}
	if len(r.host.inputs) != 2 || strings.Contains(r.host.inputs[1], "b.go") {
		t.Fatalf("fix lists = %q, want a second repair of only the finding left open", r.host.inputs)
	}
}
