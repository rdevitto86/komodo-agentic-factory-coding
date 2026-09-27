package conductor

import (
	"context"
	"errors"
	"testing"

	"komodo/internal/ledger"
)

// TestRunStageRunsOneStageAndStampsTheAdhocLedger proves each stage runs its own work once and leaves an ad hoc row.
func TestRunStageRunsOneStageAndStampsTheAdhocLedger(t *testing.T) {
	cases := []struct {
		stage    Stage
		script   func(r *rig)
		state    GroupState
		stations []string
		outcome  string
	}{
		{StageBuild, func(*rig) {}, Building, []string{StationBuild, "stage-build"}, "done"},
		{StageBuild, func(r *rig) { r.host.builds = []map[string]any{{"result": resultBlocked}} },
			Building, []string{StationBuild, "stage-build"}, "blocked"},
		{StageReview, func(*rig) {}, Reviewing, []string{StationReview, "stage-review"}, "done"},
		{StageShip, func(*rig) {}, Shipping, []string{"stage-ship"}, "done"},
		{StageShip, func(r *rig) { r.stations.shipErr = errors.New("push refused") },
			Shipping, []string{"stage-ship"}, "failed"},
	}
	for _, each := range cases {
		t.Run(string(each.stage)+" "+each.outcome, func(t *testing.T) {
			r := newRig(t)
			each.script(r)
			final, err := r.driver.RunStage(context.Background(), each.stage, State{Group: "TG-1", Current: Ready})
			if (err != nil) != (each.outcome == "failed") {
				t.Fatalf("err = %v, want one only when the stage fails", err)
			}
			if final.Current != each.state {
				t.Fatalf("state = %s, want %s", final.Current, each.state)
			}
			if got := r.transitions(); !equal(got, []GroupState{each.state}) {
				t.Fatalf("transitions = %v, want only %s", got, each.state)
			}
			run, err := r.driver.Ledger.Read(ledger.RunFile)
			if err != nil || len(run) != 0 {
				t.Fatalf("run ledger = %v, %v; an ad hoc stage belongs to no run", run, err)
			}
			adhoc, err := r.driver.Ledger.Read(ledger.AdhocFile)
			if err != nil {
				t.Fatal(err)
			}
			var stations []string
			for _, entry := range adhoc {
				if entry.Group != "TG-1" || entry.Run != "" {
					t.Fatalf("ad hoc row = %+v, want the group and no run", entry)
				}
				stations = append(stations, entry.Station)
			}
			if !equal(stations, each.stations) {
				t.Fatalf("ad hoc stations = %v, want %v", stations, each.stations)
			}
			if last := adhoc[len(adhoc)-1]; last.Outcome != each.outcome {
				t.Fatalf("stage outcome = %q, want %q", last.Outcome, each.outcome)
			}
		})
	}
}

func TestParseStageRefusesAnythingButBuildReviewAndShip(t *testing.T) {
	for _, name := range []string{"build", "review", "ship"} {
		if stage, err := ParseStage(name); err != nil || string(stage) != name {
			t.Fatalf("ParseStage(%q) = %q, %v", name, stage, err)
		}
	}
	if _, err := ParseStage("repair"); !errors.Is(err, errUnknownStage) {
		t.Fatalf("err = %v, want the unknown stage refused", err)
	}
	if _, err := newRig(t).driver.RunStage(context.Background(), Stage("repair"), State{}); !errors.Is(err, errUnknownStage) {
		t.Fatalf("err = %v, want RunStage to refuse a stage ParseStage would", err)
	}
	if _, err := (&Driver{}).RunStage(context.Background(), StageBuild, State{}); !errors.Is(err, errNotWired) {
		t.Fatalf("err = %v, want an unwired driver refused", err)
	}
}
