package conductor

import (
	"testing"

	"komodo/internal/backlog"
)

// dependent is a pending group that depends on the given groups.
func dependent(id string, parents ...string) backlog.Group {
	list := make([]any, 0, len(parents))
	for _, parent := range parents {
		list = append(list, parent)
	}
	var fields backlog.Fields
	fields.Set("depends_on", list)
	return backlog.Group{ID: id, Fields: fields}
}

func TestHoldKeepsBackEveryGroupThatWaitsOnAStoppedOne(t *testing.T) {
	pending := []backlog.Group{
		dependent("TG-3", "TG-2"),
		dependent("TG-2", "TG-1"),
		dependent("TG-4"),
		dependent("TG-5", "TG-9"),
	}
	free, held := Hold(pending, []string{"TG-1"})
	if len(free) != 2 || free[0].ID != "TG-4" || free[1].ID != "TG-5" {
		t.Fatalf("free = %v, want TG-4 and TG-5, which wait on no stopped group", free)
	}
	if len(held) != 2 || held[0].Group.ID != "TG-3" || held[1].Group.ID != "TG-2" {
		t.Fatalf("held = %+v, want TG-3 and TG-2 in pending order", held)
	}
	for _, each := range held {
		if each.On != "TG-1" {
			t.Fatalf("%s is held on %s, want the stopped TG-1 it waits on", each.Group.ID, each.On)
		}
	}
}

func TestHoldNeitherFreesNorHoldsAStoppedGroupStillPending(t *testing.T) {
	free, held := Hold([]backlog.Group{dependent("TG-1"), dependent("TG-2")}, []string{"TG-1"})
	if len(free) != 1 || free[0].ID != "TG-2" || len(held) != 0 {
		t.Fatalf("free %v, held %v; want only TG-2 free", free, held)
	}
}

func TestHoldFreesEveryGroupWhenNothingStopped(t *testing.T) {
	pending := []backlog.Group{dependent("TG-2", "TG-1"), dependent("TG-3")}
	if free, held := Hold(pending, nil); len(free) != 2 || len(held) != 0 {
		t.Fatalf("free %v, held %v; want both free", free, held)
	}
}

// TestAGroupThatStopsWithoutProgressIsBlockedWithNoOrchestratorToAsk proves a check failing
// identically after its repair blocks the group at once: there is no orchestrator left to ask.
func TestAGroupThatStopsWithoutProgressIsBlockedWithNoOrchestratorToAsk(t *testing.T) {
	r := newRig(t)
	r.stations.checks = [][]string{{"fail"}, {"fail"}}
	final, err := r.drive(t)
	if err == nil {
		t.Fatal("drive = nil; a blocked group returns the stop that blocked it")
	}
	if final.Current != Blocked {
		t.Fatalf("final = %s, want Blocked after the stop without progress", final.Current)
	}
}
