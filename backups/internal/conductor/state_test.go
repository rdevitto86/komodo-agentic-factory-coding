package conductor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/review"
)

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
			Group: "TG-1", Current: Reviewing,
			Findings: map[review.Lens][]Finding{review.Economy: {{Severity: "high", Verified: true}}},
		}, Repairing},
		{"one lens of three has a verified finding", State{
			Group: "TG-1", Current: Reviewing, Findings: map[review.Lens][]Finding{
				review.Correctness: {{Severity: "high"}}, review.Security: nil,
				review.Quality: {{Severity: "high", Verified: true}},
			},
		}, Repairing},
		{"no finding is verified", State{
			Group: "TG-1", Current: Reviewing,
			Findings: map[review.Lens][]Finding{review.Economy: {{Severity: "high"}}},
		}, Preparing},
		{"repair session still running", State{Group: "TG-1", Current: Repairing}, Repairing},
		{"repair session ended", State{Group: "TG-1", Current: Repairing, SessionDone: true}, Checking},
		{"prepare hit a conflict", State{Group: "TG-1", Current: Preparing, Conflict: true}, Repairing},
		{"prepare still running", State{Group: "TG-1", Current: Preparing}, Preparing},
		{"prepare finished", State{Group: "TG-1", Current: Preparing, SessionDone: true}, Shipping},
		{"ship still running", State{Group: "TG-1", Current: Shipping}, Shipping},
		{"ship finished", State{Group: "TG-1", Current: Shipping, ShipDone: true}, Shipped},
		{"shipped, not merged", State{Group: "TG-1", Current: Shipped}, Shipped},
		{"blocked, not yet edited", State{Group: "TG-1", Current: Blocked}, Blocked},
		{"blocked, a person edited it", State{Group: "TG-1", Current: Blocked, Edited: true}, Ready},
		{"a stop it cannot retry interrupts any running state", State{
			Group: "TG-1", Current: Building, Blocking: true,
		}, Blocked},
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

func TestAStateWrittenWithOneReviewerLoadsAsTheSingleEconomyLens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	legacy := `{"group":"TG-1","state":"Reviewing","sessions":["builder-1","reviewer-2"],` +
		`"findings":[{"severity":"high","verified":true,"file":"a.go","line":3,"title":"nil map"}],` +
		`"reviewer":"reviewer-2","reviewed":"abc","review_rounds":2,"cold_pass":true,"repairs":1}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadState(path)
	if err != nil {
		t.Fatalf("load = %v; a state.json from before lenses must still load", err)
	}
	if s.Group != "TG-1" || s.Current != Reviewing || s.Reviewed != "abc" || s.Repairs != 1 || len(s.Sessions) != 2 {
		t.Fatalf("state = %+v, want every field beside the lens maps read as written", s)
	}
	if len(s.Reviewer) != 1 || s.Reviewer[review.Economy] != "reviewer-2" {
		t.Fatalf("reviewer = %v, want the one reviewer as the economy lens", s.Reviewer)
	}
	if s.ReviewRounds[review.Economy] != 2 || !s.ColdPass[review.Economy] {
		t.Fatalf("rounds = %v, cold pass = %v, want both under the economy lens", s.ReviewRounds, s.ColdPass)
	}
	open := s.Open(review.Economy)
	if len(open) != 1 || open[0].File != "a.go" || open[0].Line != 3 {
		t.Fatalf("open = %+v, want the one verified finding under the economy lens", open)
	}
	if next := Next(s); next.Move != Repairing {
		t.Fatalf("Next = %+v, want the legacy verified finding to still send the group to repair", next)
	}
}

func TestAStateKeyedByLensSurvivesARoundTrip(t *testing.T) {
	want := State{
		Group: "TG-1", Current: Repairing,
		Findings: map[review.Lens][]Finding{
			review.Correctness: {{Lens: review.Correctness, Severity: "high", Verified: true, File: "a.go", Line: 1}},
			review.Quality:     {{Lens: review.Quality, Severity: "low", File: "b.go", Line: 2}},
		},
		Reviewer:     map[review.Lens]string{review.Correctness: "reviewer-2", review.Quality: "reviewer-3"},
		ReviewRounds: map[review.Lens]int{review.Correctness: 2, review.Quality: 1},
		ColdPass:     map[review.Lens]bool{review.Quality: true},
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := SaveState(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("round trip = %s, want %s", gotJSON, wantJSON)
	}
}

func TestTimeUsedSavesUnderItsUnitAndAnUnitlessRecordStillLoads(t *testing.T) {
	data, err := json.Marshal(State{Group: "TG-1", TimeUsed: 90 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"time_used_ns":90000000000`) || strings.Contains(string(data), `"time_used":`) {
		t.Fatalf("json = %s, want time used under time_used_ns alone", data)
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"group":"TG-1","time_used":90000000000}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadState(path)
	if err != nil || s.TimeUsed != 90*time.Second {
		t.Fatalf("time used = %s (%v), want an old record's 1m30s", s.TimeUsed, err)
	}
}

func TestAStateWithAMalformedLensMapFailsToLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"group":"TG-1","review_rounds":{"quality":"two"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(path); err == nil {
		t.Fatal("load = nil, want an error for a round count that is not a number")
	}
}

func TestNextNeverReblocksAGroupAlreadyBlocked(t *testing.T) {
	next := Next(State{Group: "TG-1", Current: Blocked, Blocking: true})
	if next.Move != Blocked {
		t.Fatalf("Next(Blocked, blocking) = %+v, want it to stay at Blocked", next)
	}
}
