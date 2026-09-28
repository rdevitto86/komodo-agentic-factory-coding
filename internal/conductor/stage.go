package conductor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"komodo/internal/ledger"
	"komodo/internal/mount"
	"komodo/internal/review"
)

// Stage is one stage a person runs ad hoc, outside the pipeline.
type Stage string

// The stages a person can run ad hoc.
const (
	StageBuild  Stage = "build"
	StageReview Stage = "review"
	StageShip   Stage = "ship"
)

// stationPrefix opens the ledger station of an ad hoc stage's own row, as in stage-build.
const stationPrefix = "stage-"

// errUnknownStage refuses a stage name other than build, review or ship.
var errUnknownStage = errors.New("the stage is build, review or ship")

// stageStates maps each ad hoc stage to the group state its work runs in.
var stageStates = map[Stage]GroupState{StageBuild: Building, StageReview: Reviewing, StageShip: Shipping}

// ParseStage reads a stage name, refusing any but build, review and ship.
func ParseStage(name string) (Stage, error) {
	stage := Stage(name)
	if _, ok := stageStates[stage]; !ok {
		return "", fmt.Errorf("%q: %w", name, errUnknownStage)
	}
	return stage, nil
}

// RunStage runs one stage once on s, outside any run, and stamps it in the ad hoc ledger.
// A builder that blocks or a review that finds something is returned in the state, not as an error.
func (d *Driver) RunStage(ctx context.Context, stage Stage, s State) (State, error) {
	if d.Host == nil || d.Stations == nil || d.Ledger == nil || d.Save == nil {
		return s, errNotWired
	}
	move, ok := stageStates[stage]
	if !ok {
		return s, fmt.Errorf("%q: %w", stage, errUnknownStage)
	}
	// No run owns an ad hoc stage, so its sessions stamp the ad hoc file.
	adhoc := *d
	adhoc.Run = ""
	s = enter(s, move)
	if err := adhoc.Save(s); err != nil {
		return s, fmt.Errorf("saving %s at %s: %w", s.Group, s.Current, err)
	}
	started := time.Now()
	r := round{fixes: s.Fixes, builder: mount.Handle(s.Builder), repairs: s.Repairs}
	var err error
	if stage == StageReview {
		// One round through every lens, with no cold pass: an ad hoc review never loops.
		lenses := adhoc.lenses()
		results, fixes := map[review.Lens]mount.Result{}, map[review.Lens][]string{}
		err = adhoc.reviewRound(ctx, &s, lenses, results, fixes)
		r.fixes = joinFixes(lenses, fixes)
		if err == nil && adhoc.WriteReview != nil {
			err = adhoc.WriteReview(s.Group, mergedReview(lenses, results))
		}
	} else {
		err = adhoc.work(ctx, &s, &r)
	}
	s.Fixes, s.Builder = r.fixes, string(r.builder)
	s.TimeUsed += time.Since(started)
	outcome := "done"
	switch {
	case err != nil:
		outcome = "failed"
	case s.Escalate:
		outcome = "blocked"
	}
	findings := 0
	for _, each := range s.Findings {
		findings += len(each)
	}
	entry := ledger.Entry{
		Group: s.Group, Station: stationPrefix + string(stage), Seconds: time.Since(started).Seconds(),
		Outcome: outcome, Findings: findings,
	}
	if stampErr := d.Ledger.Stamp(entry); stampErr != nil {
		return s, errors.Join(err, stampErr)
	}
	return s, err
}
