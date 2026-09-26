package conductor

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"komodo/internal/mount"
)

// TestResumeContinuesAKilledBuildWithoutRepeatingTheSession resumes a Building group whose
// session was still open when the run was killed, and drives it on to Shipped.
func TestResumeContinuesAKilledBuildWithoutRepeatingTheSession(t *testing.T) {
	r := newRig(t)
	last := mount.Handle("builder-1")
	r.host.results[last] = mount.Result{Value: map[string]any{"result": "DONE"}}
	start := State{Group: "TG-1", Current: Building, Sessions: []string{string(last)}}
	*r.saved = append(*r.saved, start)

	final, err := r.driver.Resume(context.Background(), start)
	if err != nil || final.Current != Shipped {
		t.Fatalf("resume = %s, %v; want Shipped", final.Current, err)
	}
	for _, req := range r.host.starts {
		if req.Role == "builder" {
			t.Fatalf("resume started a fresh builder session %+v, want the killed one resumed", req)
		}
	}
	if len(r.host.inputs) != 1 || r.host.inputs[0] != "" {
		t.Fatalf("resume inputs = %v, want one empty resume", r.host.inputs)
	}
	if got := r.sessions(t); !equal(got, []string{StationBuild, StationReview}) {
		t.Fatalf("ledger sessions = %v, want the resumed build then a fresh review", got)
	}
}

// TestResumeStartsAFreshBuildWhenTheHostCannotResume falls back to a new builder session when
// the host has no way to reattach to the one a kill left running.
func TestResumeStartsAFreshBuildWhenTheHostCannotResume(t *testing.T) {
	r := newRig(t)
	r.host.resume = false
	start := State{Group: "TG-1", Current: Building, Sessions: []string{"builder-1"}}
	*r.saved = append(*r.saved, start)

	final, err := r.driver.Resume(context.Background(), start)
	if err != nil || final.Current != Shipped {
		t.Fatalf("resume = %s, %v; want Shipped", final.Current, err)
	}
	if len(r.host.inputs) != 0 {
		t.Fatalf("resume called Resume %v on a host that cannot resume", r.host.inputs)
	}
	if len(r.host.starts) == 0 || r.host.starts[0].Role != "builder" {
		t.Fatalf("resume starts = %+v, want a fresh builder session first", r.host.starts)
	}
}

// TestResumeDrivesOnWhenNothingIsPending leaves a state with a finished review alone and
// simply carries the group forward, starting no session at all.
func TestResumeDrivesOnWhenNothingIsPending(t *testing.T) {
	r := newRig(t)
	start := State{Group: "TG-1", Current: Reviewing}
	*r.saved = append(*r.saved, start)

	final, err := r.driver.Resume(context.Background(), start)
	if err != nil || final.Current != Shipped {
		t.Fatalf("resume = %s, %v; want Shipped", final.Current, err)
	}
	if len(r.host.starts) != 0 {
		t.Fatalf("resume starts = %+v, want none since the review had already finished", r.host.starts)
	}
}

// TestResumeRefusesAnUnwiredDriver mirrors Drive's refusal of a Driver missing its wiring.
func TestResumeRefusesAnUnwiredDriver(t *testing.T) {
	var driver Driver
	if _, err := driver.Resume(context.Background(), State{Group: "TG-1"}); !errors.Is(err, errNotWired) {
		t.Fatalf("resume error = %v, want errNotWired", err)
	}
}

// TestStateRoundTripsThroughLoadAndSaveState proves state.json survives a save and a reload.
func TestStateRoundTripsThroughLoadAndSaveState(t *testing.T) {
	path := StatePath(t.TempDir(), "TG-1")
	want := State{Group: "TG-1", Current: Repairing, Sessions: []string{"builder-1"}}
	if err := SaveState(path, want); err != nil {
		t.Fatalf("SaveState = %v", err)
	}
	got, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState = %v", err)
	}
	if got.Group != want.Group || got.Current != want.Current || len(got.Sessions) != 1 {
		t.Fatalf("LoadState = %+v, want %+v", got, want)
	}
	if filepath.Base(path) != stateFile {
		t.Fatalf("StatePath = %q, want it to end in %q", path, stateFile)
	}
}

// TestLoadStateFailsOnAMissingFile reports a group with no saved state plainly.
func TestLoadStateFailsOnAMissingFile(t *testing.T) {
	if _, err := LoadState(StatePath(t.TempDir(), "TG-1")); err == nil {
		t.Fatal("LoadState = nil, want an error for a missing state.json")
	}
}
