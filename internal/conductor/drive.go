package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"komodo/internal/ledger"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/pr"
)

// Ledger stations for the only three kinds of model session the conductor starts.
const (
	StationBuild  = "build"
	StationReview = "review"
	StationRepair = "repair"
)

// resultBlocked is the builder result that stops a group for the orchestrator.
const resultBlocked = "BLOCKED"

// errNotWired refuses a Driver missing its host, stations, ledger or state writer.
var errNotWired = errors.New("the conductor needs a host, stations, a ledger and a state writer")

// Stations are the stages the conductor runs itself, with no model: Check, Prepare and Ship.
type Stations interface {
	// Check reruns every check and returns one fix per failure, none when all passed.
	Check() ([]string, error)
	// Prepare integrates the group and returns one fix per conflict or integration failure.
	Prepare() ([]string, error)
	// Ship pushes the group and opens its draft PR.
	Ship() error
}

// Driver moves one group through its states, starting model sessions only to build, review and repair.
type Driver struct {
	Host     mount.Contract
	Stations Stations
	Ledger   *ledger.Ledger
	Run      string
	Builder  mount.StartRequest
	Reviewer mount.StartRequest
	// SeverityFloor is the lowest review severity that blocks; empty blocks every finding.
	SeverityFloor string
	// Repairs is how many repair rounds a group gets before it escalates; zero means line.MaxRepairs.
	Repairs int
	// Save writes the group's state to state.json.
	Save func(State) error
}

// round is what one Drive call carries between states: the open fix list, the builder session, repairs spent.
type round struct {
	fixes   []string
	builder mount.Handle
	repairs int
}

// Drive moves a group from s until it waits on something outside the conductor, and returns that state.
// A host or station failure escalates the group and is returned alongside it.
func (d *Driver) Drive(ctx context.Context, s State) (State, error) {
	if d.Host == nil || d.Stations == nil || d.Ledger == nil || d.Save == nil {
		return s, errNotWired
	}
	// The round lives in state.json, so a resumed run keeps its fix list, builder and repair count.
	r := round{fixes: s.Fixes, builder: mount.Handle(s.Builder), repairs: s.Repairs}
	var failure error
	for {
		if err := ctx.Err(); err != nil {
			return s, err
		}
		next := Next(s)
		if next.Remove || next.Move == s.Current {
			return s, failure
		}
		s = enter(s, next.Move)
		if err := d.Save(s); err != nil {
			return s, fmt.Errorf("saving %s at %s: %w", s.Group, s.Current, err)
		}
		started := time.Now()
		err := d.work(ctx, &s, &r)
		s.Fixes, s.Builder, s.Repairs = r.fixes, string(r.builder), r.repairs
		s.TimeUsed += time.Since(started)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return s, ctxErr
		}
		if err != nil {
			s.Escalate = true
			failure = fmt.Errorf("%s escalated at %s: %w", s.Group, s.Current, err)
		}
	}
}

// enter moves a group into a state and clears every flag the previous state's work set.
func enter(s State, move GroupState) State {
	if move == Escalated {
		s.Left = s.Current
	} else {
		s.Escalate, s.Answered, s.Stop = false, false, false
	}
	s.Current = move
	s.SlotFree, s.SessionDone, s.ChecksPassed, s.Conflict, s.ShipDone, s.Edited = false, false, false, false, false, false
	return s
}

// work runs one state's work and records its outcome in the flags Next reads.
func (d *Driver) work(ctx context.Context, s *State, r *round) error {
	switch s.Current {
	case Building:
		handle, err := d.Host.Start(d.Builder)
		if err != nil {
			return err
		}
		r.builder = handle
		return d.build(ctx, s, StationBuild, d.Builder, handle)
	case Checking:
		fixes, err := d.Stations.Check()
		if err != nil {
			return err
		}
		r.fixes = fixes
		s.ChecksPassed = len(fixes) == 0
	case Reviewing:
		return d.review(ctx, s, r)
	case Repairing:
		return d.repair(ctx, s, r)
	case Preparing:
		fixes, err := d.Stations.Prepare()
		if err != nil {
			return err
		}
		r.fixes = fixes
		s.Conflict = len(fixes) > 0
		s.SessionDone = !s.Conflict
	case Shipping:
		if err := d.Stations.Ship(); err != nil {
			return err
		}
		s.ShipDone = true
	}
	return nil
}

// build drains a builder session and marks it done, or escalates when the builder is blocked.
func (d *Driver) build(
	ctx context.Context, s *State, station string, req mount.StartRequest, handle mount.Handle,
) error {
	result, err := d.session(ctx, s, station, req, handle)
	if err != nil {
		return err
	}
	if result.Value["result"] == resultBlocked {
		s.Escalate = true
		return nil
	}
	s.SessionDone = true
	return nil
}

// review runs the one reviewer session and turns its findings at or above the floor into the fix list.
func (d *Driver) review(ctx context.Context, s *State, r *round) error {
	handle, err := d.Host.Start(d.Reviewer)
	if err != nil {
		return err
	}
	result, err := d.session(ctx, s, StationReview, d.Reviewer, handle)
	if err != nil {
		return err
	}
	data, err := json.Marshal(result.Value["findings"])
	if err != nil {
		return err
	}
	var findings []line.Finding
	if err := json.Unmarshal(data, &findings); err != nil {
		return fmt.Errorf("reading the reviewer's findings: %w", err)
	}
	s.Findings, r.fixes = nil, nil
	for _, finding := range findings {
		verified := line.AtOrAbove(finding.Severity, d.SeverityFloor)
		s.Findings = append(s.Findings, Finding{Severity: finding.Severity, Verified: verified})
		if verified {
			r.fixes = append(r.fixes, fmt.Sprintf("%s:%d %s: %s", finding.File, finding.Line, finding.Title, finding.Fix))
		}
	}
	return nil
}

// repair resumes the builder with the fix list, or starts a fresh one when the host cannot resume,
// and escalates once the group's repair rounds are spent.
func (d *Driver) repair(ctx context.Context, s *State, r *round) error {
	limit := d.Repairs
	if limit == 0 {
		limit = line.MaxRepairs
	}
	r.repairs++
	if r.repairs > limit {
		s.Escalate = true
		return nil
	}
	lines := make([]string, 0, len(r.fixes))
	for _, fix := range r.fixes {
		lines = append(lines, "- [ ] "+fix)
	}
	input := "## Fix list\n\n" + strings.Join(lines, "\n")
	req := d.Builder
	var handle mount.Handle
	var err error
	if r.builder != "" && d.Host.Capabilities().Resume {
		handle, err = d.Host.Resume(r.builder, input)
	} else {
		req.Brief += "\n\n" + input
		handle, err = d.Host.Start(req)
	}
	if err != nil {
		return err
	}
	r.builder = handle
	return d.build(ctx, s, StationRepair, req, handle)
}

// session records a session's handle in state.json, drains its stream, stamps it in the ledger, and
// returns its result; a cancelled context stops the session.
func (d *Driver) session(
	ctx context.Context, s *State, station string, req mount.StartRequest, handle mount.Handle,
) (mount.Result, error) {
	started := time.Now()
	s.Sessions = append(s.Sessions, string(handle))
	if err := d.Save(*s); err != nil {
		return mount.Result{}, err
	}
	events, err := d.Host.Stream(handle)
	if err != nil {
		return mount.Result{}, err
	}
	entry := ledger.Entry{Run: d.Run, Group: s.Group, Station: station, Role: req.Role, Model: req.Model}
	for open := true; open; {
		select {
		case <-ctx.Done():
			return mount.Result{}, errors.Join(ctx.Err(), d.Host.Stop(handle))
		case event, ok := <-events:
			open = ok
			entry.Turns += event.Turns
			entry.TokensIn += event.Usage.TokensIn
			entry.TokensOut += event.Usage.TokensOut
			entry.TokensCached += event.Usage.TokensCached
		}
	}
	result, err := d.Host.Result(handle)
	entry.Seconds = time.Since(started).Seconds()
	entry.Outcome = "done"
	if err != nil {
		entry.Outcome = "failed"
	} else if outcome, ok := result.Value["result"].(string); ok {
		entry.Outcome = strings.ToLower(outcome)
	}
	if findings, ok := result.Value["findings"].([]any); ok {
		entry.Findings = len(findings)
	}
	if stampErr := d.Ledger.Stamp(entry); stampErr != nil {
		return result, errors.Join(err, stampErr)
	}
	return result, err
}

// Line runs Check, Prepare and Ship through the stations internal/line already has.
type Line struct {
	Root   string
	Plan   *line.Plan
	Client *pr.Client
	waves  []*line.WaveResult
}

// Check reruns the compile gates, then the verify command, in the group's worktree.
func (l *Line) Check() ([]string, error) {
	worktree := line.WorktreePath(l.Root, l.Plan.Worktree)
	results := line.RunGate(worktree, line.CompileCommands(l.Root, worktree))
	if failure, failed := line.FirstFailure(results); failed {
		return []string{line.FailureText(failure)}, nil
	}
	if command := line.VerifyCommand(l.Root, worktree); command != "" {
		if verify := line.RunCommand(worktree, command); !verify.OK() {
			return []string{line.FailureText(verify)}, nil
		}
	}
	return nil, nil
}

// Prepare merges each wave into the group branch and reruns its integration gates and verify.
func (l *Line) Prepare() ([]string, error) {
	l.waves = nil
	for index := range l.Plan.Waves {
		wave, err := line.CloseWave(l.Root, l.Plan, index)
		if err != nil {
			return nil, err
		}
		l.waves = append(l.waves, wave)
		if wave.Conflict != "" {
			return []string{wave.Conflict}, nil
		}
		if failure, failed := line.FirstFailure(wave.Gates); failed {
			return []string{line.FailureText(failure)}, nil
		}
		if wave.Verify != nil && !wave.Verify.OK() {
			return []string{line.FailureText(*wave.Verify)}, nil
		}
	}
	return nil, nil
}

// Ship commits, pushes and opens the group's draft PR with the waves Prepare integrated.
func (l *Line) Ship() error {
	_, err := line.ShipGroup(l.Root, l.Plan, l.waves, l.Client)
	return err
}
