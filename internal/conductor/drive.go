package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/check"
	"komodo/internal/git"
	"komodo/internal/ledger"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/pr"
	"komodo/internal/review"
)

// Ledger stations for the conductor's model sessions; a cold review is review, a warm one re-review.
const (
	StationBuild    = "build"
	StationReview   = "review"
	StationReReview = "re-review"
	StationRepair   = "repair"
)

// warmRounds is how many rounds a lens's warm reviewer runs before the lens earns its one cold pass.
const warmRounds = 2

// blastRadii are the reviewer's blast-radius tiers, narrowest first.
var blastRadii = []string{"low", "low-med", "med", "med-high", "high", "critical"}

// repairLead opens a fresh repair's brief, so the builder fixes the list instead of re-verifying a finished build.
const repairLead = "# Repair\n\nThe group is already built. Apply every item on the fix list below, then rerun its checks.\n" +
	"Report DONE only once each item is fixed.\n\n"

// continueLead opens the brief of a fresh session replacing one lost mid-build, so it picks up the worktree's work.
const continueLead = "# Continue\n\nA session building this group was cut off, and its work is in this worktree.\n" +
	"Read `git status` and the changed files first; finish what is left, and redo nothing already done.\n\n"

// fixList renders fixes as the checklist a repair works through.
func fixList(fixes []string) string {
	lines := make([]string, 0, len(fixes))
	for _, fix := range fixes {
		lines = append(lines, "- [ ] "+fix)
	}
	return "## Fix list\n\n" + strings.Join(lines, "\n")
}

// taskFiles renders each task's files as the plan holds them, which close a repair's fix list.
func taskFiles(tasks []line.PlanTask) string {
	if len(tasks) == 0 {
		return ""
	}
	lines := make([]string, 0, len(tasks))
	for _, task := range tasks {
		files := "none"
		if len(task.Files) > 0 {
			files = "`" + strings.Join(task.Files, "`, `") + "`"
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", task.ID, files))
	}
	return "\n\n## Each task's files, as the plan holds them now\n\n" + strings.Join(lines, "\n")
}

// repairBrief is a fresh repair's brief: the fix list first, then the group's brief for reference.
func repairBrief(fixList, brief string) string {
	return repairLead + fixList + "\n\n## The group's brief, for reference\n\n" + brief
}

// resultBlocked is the builder result that stops a group for the orchestrator.
const resultBlocked = "BLOCKED"

// errNotWired refuses a Driver missing its host, stations, ledger or state writer.
var errNotWired = errors.New("the conductor needs a host, stations, a ledger and a state writer")

// Stations are the stages the conductor runs itself, with no model: Check, Prepare and Ship.
type Stations interface {
	// Snapshot records the worktree's git state before a build or repair session, for Check to compare.
	Snapshot() error
	// Check reruns every check and returns one fix per failure, none when all passed; its commands die with ctx.
	Check(ctx context.Context) ([]string, error)
	// Prepare verifies the group's integrated worktree and returns one fix per failure.
	Prepare(ctx context.Context) ([]string, error)
	// Ship pushes the group and opens its draft PR.
	Ship(ctx context.Context) error
	// Head returns the worktree's HEAD commit, which a review records as the commit it saw.
	Head() (string, error)
	// Diff returns the worktree's diff since a reviewed commit: the lines a repair changed after that review.
	Diff(since string) (string, error)
	// Merge merges a shipped PR into its epic branch, reporting whether it did; other bases wait for a person.
	Merge() (bool, error)
}

// Driver moves one group through its states, starting model sessions only to build, review and repair.
type Driver struct {
	Host     mount.Contract
	Stations Stations
	Ledger   *ledger.Ledger
	Run      string
	Builder  mount.StartRequest
	Reviewer mount.StartRequest
	// Lenses are the lenses every review runs in parallel; none runs the one combined economy lens.
	Lenses []review.Lens
	// Review builds a lens's request from the group's diff as it stands at review; nil uses Reviewer.
	Review func(review.Lens) (mount.StartRequest, error)
	// ReReview builds the input a lens's resumed reviewer reads; nil sends the lens's open findings alone.
	ReReview func(review.Lens, State) (string, error)
	// SeverityFloor is the lowest review severity that blocks; empty blocks every finding.
	SeverityFloor string
	// Tasks are the group's tasks as the plan the conductor loaded holds them; their files close each fix list.
	Tasks []line.PlanTask
	// Save writes the group's state to state.json.
	Save func(State) error
	// WriteReview keeps the reviewer's whole result where Ship's findings gate reads it; nil skips it.
	WriteReview func(group string, result mount.Result) error
	// Orchestrator builds the request of the headless session that settles an escalation; nil waits for a person.
	Orchestrator func(Escalation) (mount.StartRequest, error)
	// Heavy is the machine the orchestrator's one retry runs the builder on; empty refuses the retry.
	Heavy mount.Machine
	// Lint returns the task list's lint problems, which a split or clarify must leave empty.
	Lint func() ([]string, error)
	// Block commits a stopped group's note on its branch and publishes it as a blocked draft PR; nil skips it.
	Block func(context.Context, backlog.BlockerNote) error
}

// round is what one Drive call carries between states: the fix list, builder, repairs, and open escalation.
type round struct {
	fixes   []string
	builder mount.Handle
	repairs int
	reason  string
	answer  string
	needs   string
	heavy   bool
	stalls  int
}

// newRound is the round state.json saved for s, so a resumed run keeps its fixes, builder, repairs and escalation.
func newRound(s State) round {
	return round{
		fixes: s.Fixes, builder: mount.Handle(s.Builder), repairs: s.Repairs,
		reason: s.Reason, answer: s.Answer, stalls: s.Stalls, heavy: s.Heavy,
	}
}

// keep writes the round into s, so the next save of s carries it.
func (r *round) keep(s *State) {
	s.Fixes, s.Builder, s.Repairs = r.fixes, string(r.builder), r.repairs
	s.Reason, s.Answer, s.Stalls, s.Heavy = r.reason, r.answer, r.stalls, r.heavy
}

// Drive moves a group from s until it waits on something outside the conductor, and returns that state.
// A host or station failure escalates the group and is returned alongside it, unless the orchestrator settles it.
func (d *Driver) Drive(ctx context.Context, s State) (State, error) {
	if d.Host == nil || d.Stations == nil || d.Ledger == nil || d.Save == nil {
		return s, errNotWired
	}
	r := newRound(s)
	var failure error
	// A group saved at Escalated with no answer yet asks the orchestrator before Next reads it.
	if s.Current == Escalated && !s.Answered {
		err := d.escalate(ctx, &s, &r)
		r.keep(&s)
		if err != nil {
			return s, fmt.Errorf("%s escalated at %s: %w", s.Group, s.Current, err)
		}
		if err := d.Save(s); err != nil {
			return s, fmt.Errorf("saving %s at %s: %w", s.Group, s.Current, err)
		}
	}
	for {
		r.keep(&s)
		if err := ctx.Err(); err != nil {
			return s, err
		}
		next := Next(s)
		if next.Remove || next.Move == s.Current {
			return s, failure
		}
		// A settled escalation clears the failure that raised it; a stop keeps it for the caller.
		if s.Current == Escalated && next.Move != Blocked {
			failure, r.reason, s.Reason = nil, "", ""
		}
		s = enter(s, next.Move)
		if err := d.Save(s); err != nil {
			return s, fmt.Errorf("saving %s at %s: %w", s.Group, s.Current, err)
		}
		started := time.Now()
		err := d.work(ctx, &s, &r)
		r.keep(&s)
		s.TimeUsed += time.Since(started)
		// The orchestrator's action is saved as it lands, so a run killed before acting on it resumes with it.
		if s.Current == Escalated && s.Answered {
			if err := d.Save(s); err != nil {
				return s, fmt.Errorf("saving %s at %s: %w", s.Group, s.Current, err)
			}
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return s, ctxErr
		}
		if err != nil {
			s.Escalate = true
			r.reason = err.Error()
			if errors.Is(err, errNoProgress) {
				r.stalls++
			}
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
		if err := d.Stations.Snapshot(); err != nil {
			return err
		}
		req, handle, err := d.startBuilder(ctx, d.builderRequest(r), r)
		if err != nil {
			return err
		}
		r.builder, r.fixes = handle, nil
		return d.build(ctx, s, StationBuild, req, handle)
	case Checking:
		fixes, err := d.Stations.Check(ctx)
		if err != nil {
			return err
		}
		// The open fix list is the one the last repair worked, so the same failures mean it fixed nothing.
		if len(fixes) > 0 && slices.Equal(fixes, r.fixes) {
			return fmt.Errorf("%w: the checks fail identically after a repair: %s", errNoProgress, strings.Join(fixes, "; "))
		}
		r.fixes = fixes
		s.ChecksPassed = len(fixes) == 0
	case Reviewing:
		return d.review(ctx, s, r)
	case Repairing:
		return d.repair(ctx, s, r)
	case Preparing:
		fixes, err := d.Stations.Prepare(ctx)
		if err != nil {
			return err
		}
		r.fixes = fixes
		s.Conflict = len(fixes) > 0
		s.SessionDone = !s.Conflict
	case Shipping:
		if err := d.Stations.Ship(ctx); err != nil {
			return err
		}
		s.ShipDone = true
	case Shipped:
		merged, err := d.Stations.Merge()
		if err != nil {
			return err
		}
		s.Merged = merged
	case Escalated:
		return d.escalate(ctx, s, r)
	case Blocked:
		return d.stop(ctx, s, r)
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

// lensSession is one lens's session in a review round: the station it stamps, its request and its handle.
type lensSession struct {
	lens    review.Lens
	station string
	request mount.StartRequest
	handle  mount.Handle
}

// lenses returns the lenses every review runs, the one economy lens when the driver names none.
func (d *Driver) lenses() []review.Lens {
	if len(d.Lenses) == 0 {
		return []review.Lens{review.Economy}
	}
	return d.Lenses
}

// review runs every lens's round in parallel, then, when no lens has a fix, the cold pass of each lens
// whose warm reviewer passed a later round; a round closing no open finding stops the loop.
func (d *Driver) review(ctx context.Context, s *State, r *round) error {
	lenses := d.lenses()
	results := map[review.Lens]mount.Result{}
	fixes := map[review.Lens][]string{}
	before := openFindings(*s, lenses)
	if err := d.reviewRound(ctx, s, lenses, results, fixes); err != nil {
		return err
	}
	var stalled error
	if after := openFindings(*s, lenses); closesNothing(before, after) {
		stalled = stalledReview(after)
	}
	r.fixes = joinFixes(lenses, fixes)
	var cold []review.Lens
	for _, lens := range lenses {
		if !s.ColdPass[lens] && s.ReviewRounds[lens] >= warmRounds {
			cold = append(cold, lens)
		}
	}
	if len(cold) > 0 && len(r.fixes) == 0 {
		s.ColdPass, s.Reviewer = cloned(s.ColdPass), cloned(s.Reviewer)
		for _, lens := range cold {
			s.ColdPass[lens] = true
			delete(s.Reviewer, lens)
		}
		if err := d.reviewRound(ctx, s, cold, results, fixes); err != nil {
			return err
		}
		r.fixes = joinFixes(lenses, fixes)
	}
	// Findings from a lens this driver does not run, such as a single-reviewer record's, are dropped.
	for lens := range s.Findings {
		if !slices.Contains(lenses, lens) {
			delete(s.Findings, lens)
		}
	}
	if d.WriteReview == nil {
		return stalled
	}
	if err := d.WriteReview(s.Group, mergedReview(lenses, results)); err != nil {
		return err
	}
	return stalled
}

// reviewRound opens each lens's session, drains them all in parallel, then files each lens's findings,
// and its fixes for those at or above the floor, into results and fixes.
func (d *Driver) reviewRound(
	ctx context.Context, s *State, lenses []review.Lens,
	results map[review.Lens]mount.Result, fixes map[review.Lens][]string,
) error {
	s.Findings, s.Reviewer, s.ReviewRounds = cloned(s.Findings), cloned(s.Reviewer), cloned(s.ReviewRounds)
	since, warm := s.Reviewed, false
	sessions := make([]lensSession, 0, len(lenses))
	for _, lens := range lenses {
		session, err := d.openLens(ctx, s, lens)
		if err != nil {
			return errors.Join(err, d.stopAll(ctx, sessions))
		}
		sessions = append(sessions, session)
		warm = warm || session.station == StationReReview
	}
	head, err := d.Stations.Head()
	if err != nil {
		return errors.Join(err, d.stopAll(ctx, sessions))
	}
	s.Reviewed = head
	for _, each := range sessions {
		s.Reviewer[each.lens] = string(each.handle)
		s.ReviewRounds[each.lens]++
		s.Sessions = append(s.Sessions, string(each.handle))
	}
	if err := d.Save(*s); err != nil {
		return errors.Join(err, d.stopAll(ctx, sessions))
	}
	group := s.Group
	drained := make([]mount.Result, len(sessions))
	failed := make([]error, len(sessions))
	var wg sync.WaitGroup
	for i, each := range sessions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			drained[i], failed[i] = d.drain(ctx, group, each.station, each.request, each.handle)
		}()
	}
	wg.Wait()
	if err := errors.Join(failed...); err != nil {
		return err
	}
	var repair string
	if warm && since != "" {
		if repair, err = d.Stations.Diff(since); err != nil {
			return err
		}
	}
	for i, each := range sessions {
		returned, err := returnedFindings(each.lens, drained[i])
		if err != nil {
			return err
		}
		// A resumed lens may only keep its open findings or flag the lines the repair changed.
		if each.station == StationReReview && since != "" {
			returned = review.Rereview(openSpots(s.Findings[each.lens]), returned, repair)
			if drained[i], err = withFindings(drained[i], returned); err != nil {
				return err
			}
		}
		found, lensFixes := d.fileFindings(each.lens, returned)
		results[each.lens], s.Findings[each.lens], fixes[each.lens] = drained[i], found, lensFixes
	}
	return nil
}

// openLens resumes a lens's reviewer with its re-review input, or starts a cold one carrying the lens's
// open findings when it has none, the host cannot resume, or the resume fails.
func (d *Driver) openLens(ctx context.Context, s *State, lens review.Lens) (lensSession, error) {
	session := lensSession{lens: lens, station: StationReReview, request: d.Reviewer}
	var err error
	if previous := s.Reviewer[lens]; previous != "" && d.Host.Capabilities().Resume {
		input := line.OpenFindings(s.Open(lens))
		if d.ReReview != nil {
			if input, err = d.ReReview(lens, *s); err != nil {
				return session, err
			}
		}
		session.handle, err = d.Host.Resume(ctx, mount.Handle(previous), input)
	}
	// A reviewer from an earlier process is gone after a restart; a cold one gets the open findings.
	if session.handle == "" {
		session.station, s.ReviewRounds[lens] = StationReview, 0
		if d.Review != nil {
			if session.request, err = d.Review(lens); err != nil {
				return session, err
			}
		}
		if open := s.Open(lens); len(open) > 0 {
			session.request.Brief += "\n\n" + line.OpenFindings(open)
		}
		session.handle, err = d.Host.Start(ctx, session.request)
	}
	return session, err
}

// stopAll stops every lens session a failed round already opened.
func (d *Driver) stopAll(ctx context.Context, sessions []lensSession) error {
	errs := make([]error, 0, len(sessions))
	for _, each := range sessions {
		errs = append(errs, d.Host.Stop(ctx, each.handle))
	}
	return errors.Join(errs...)
}

// returnedFindings reads the findings one lens's session returned.
func returnedFindings(lens review.Lens, result mount.Result) ([]review.Finding, error) {
	data, err := json.Marshal(result.Value["findings"])
	if err != nil {
		return nil, err
	}
	var returned []review.Finding
	if err := json.Unmarshal(data, &returned); err != nil {
		return nil, fmt.Errorf("reading the %s lens's findings: %w", lens, err)
	}
	return returned, nil
}

// openSpots is where a lens's open findings sit, the lines its re-review may keep.
func openSpots(findings []Finding) []review.Finding {
	var open []review.Finding
	for _, finding := range findings {
		if finding.Verified {
			open = append(open, review.Finding{File: finding.File, Line: finding.Line})
		}
	}
	return open
}

// withFindings returns result with its findings swapped for kept, so Ship reads only what a re-review may raise.
func withFindings(result mount.Result, kept []review.Finding) (mount.Result, error) {
	data, err := json.Marshal(kept)
	if err != nil {
		return result, err
	}
	var list []any
	if err := json.Unmarshal(data, &list); err != nil {
		return result, err
	}
	value := make(map[string]any, len(result.Value))
	for key, each := range result.Value {
		value[key] = each
	}
	value["findings"] = list
	return mount.Result{Value: value}, nil
}

// fileFindings files one lens's findings under their lenses, with a fix for each at or above the floor.
func (d *Driver) fileFindings(lens review.Lens, returned []review.Finding) ([]Finding, []string) {
	var found []Finding
	var fixes []string
	for _, finding := range returned {
		owner := review.Lens(finding.Lens)
		if owner == "" {
			owner = lens
		}
		verified := line.AtOrAbove(finding.Severity, d.SeverityFloor)
		found = append(found, Finding{
			Lens: owner, Severity: finding.Severity, Verified: verified,
			File: finding.File, Line: finding.Line, Title: finding.Title,
		})
		if verified {
			fixes = append(fixes, fmt.Sprintf("%s:%d %s: %s", finding.File, finding.Line, finding.Title, finding.Fix))
		}
	}
	return found, fixes
}

// joinFixes lists every lens's fixes, in lens order.
func joinFixes(lenses []review.Lens, fixes map[review.Lens][]string) []string {
	var out []string
	for _, lens := range lenses {
		out = append(out, fixes[lens]...)
	}
	return out
}

// cloned copies a lens-keyed map, so a review never writes one an earlier copy of the state shares.
func cloned[V any](m map[review.Lens]V) map[review.Lens]V {
	out := make(map[review.Lens]V, len(m))
	for lens, value := range m {
		out[lens] = value
	}
	return out
}

// mergedReview joins every lens's result into the one review Ship reads: each lens's findings and
// summary, and the widest blast radius any lens scored. A single lens's result passes through whole.
func mergedReview(lenses []review.Lens, results map[review.Lens]mount.Result) mount.Result {
	if len(lenses) == 1 {
		return results[lenses[0]]
	}
	findings := []any{}
	var summaries []string
	merged := map[string]any{}
	widest := -1
	for _, lens := range lenses {
		value := results[lens].Value
		if list, ok := value["findings"].([]any); ok {
			findings = append(findings, list...)
		}
		if summary, ok := value["summary"].(string); ok && summary != "" {
			summaries = append(summaries, string(lens)+": "+summary)
		}
		radius, _ := value["blast_radius"].(string)
		if tier := slices.Index(blastRadii, radius); tier > widest {
			widest = tier
			merged["blast_radius"], merged["blast_radius_why"] = radius, value["blast_radius_why"]
		}
	}
	merged["findings"], merged["summary"] = findings, strings.Join(summaries, " ")
	return mount.Result{Value: merged}
}

// repair resumes the builder with the fix list, or starts a fresh one when the host cannot resume,
// and stops the loop when the repair changed no file.
func (d *Driver) repair(ctx context.Context, s *State, r *round) error {
	r.repairs++
	input := fixList(r.fixes) + taskFiles(d.Tasks)
	if r.answer != "" {
		input, r.answer = answerLead+r.answer+"\n\n"+input, ""
	}
	if err := d.Stations.Snapshot(); err != nil {
		return err
	}
	head, err := d.Stations.Head()
	if err != nil {
		return err
	}
	before, err := d.Stations.Diff(head)
	if err != nil {
		return err
	}
	req := d.builderRequest(r)
	var handle mount.Handle
	if r.builder != "" && d.Host.Capabilities().Resume {
		handle, err = d.Host.Resume(ctx, r.builder, input)
	}
	// A builder from an earlier process is gone after a restart; a fresh one gets the brief and fixes.
	if handle == "" {
		req.Brief = repairBrief(input, req.Brief)
		handle, err = d.Host.Start(ctx, req)
	}
	if err != nil {
		return err
	}
	r.builder = handle
	if err := d.build(ctx, s, StationRepair, req, handle); err != nil || !s.SessionDone {
		return err
	}
	after, err := d.Stations.Diff(head)
	if err != nil {
		return err
	}
	if after == before {
		return fmt.Errorf("%w: the repair changed no file, leaving %s", errNoProgress, strings.Join(r.fixes, "; "))
	}
	return nil
}

// session records a session's handle in state.json, then drains it.
func (d *Driver) session(
	ctx context.Context, s *State, station string, req mount.StartRequest, handle mount.Handle,
) (mount.Result, error) {
	s.Sessions = append(s.Sessions, string(handle))
	if err := d.Save(*s); err != nil {
		return mount.Result{}, err
	}
	return d.drain(ctx, s.Group, station, req, handle)
}

// drain streams a session to its end, stamps it in the ledger, and returns its result; a cancelled
// context stops the session.
func (d *Driver) drain(
	ctx context.Context, group, station string, req mount.StartRequest, handle mount.Handle,
) (mount.Result, error) {
	started := time.Now()
	events, err := d.Host.Stream(ctx, handle)
	if err != nil {
		return mount.Result{}, err
	}
	entry := ledger.Entry{Run: d.Run, Group: group, Station: station, Role: req.Role, Model: req.Model}
	for open := true; open; {
		select {
		case <-ctx.Done():
			return mount.Result{}, errors.Join(ctx.Err(), d.Host.Stop(ctx, handle))
		case event, ok := <-events:
			open = ok
			entry.Turns += event.Turns
			entry.TokensIn += event.Usage.TokensIn
			entry.TokensOut += event.Usage.TokensOut
			entry.TokensCached += event.Usage.TokensCached
			if event.RateLimit != nil {
				mount.ObserveRateLimit(*event.RateLimit)
			}
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
	// checked is the last Check's gates and verify, which Ship reports in the PR body.
	checked *line.WaveResult
	// shipped is what Ship did, whose base Merge reads.
	shipped *line.ShipResult
	// ran is the check commands the last rerun ran.
	ran []string
}

// passed records every check command that ran as passed, for the PR body's validation lines.
func passed(commands []string) *line.WaveResult {
	result := &line.WaveResult{OK: true}
	for _, command := range commands {
		if command != "" {
			result.Gates = append(result.Gates, line.CommandResult{Command: command})
		}
	}
	return result
}

// coverageBarFile holds the calibrated changed-line coverage bar, under the root's state directory.
const coverageBarFile = "coverage.json"

// snapshotFile holds the git state Snapshot took ahead of the last session, under the group's run directory.
const snapshotFile = "snapshot.json"

// errNoSnapshot refuses a Check with no snapshot to compare the session it follows against.
var errNoSnapshot = errors.New("no snapshot was taken before the session, so its output cannot be checked")

// snapshotPath is where the group's last snapshot lives, beside its state.json.
func (l *Line) snapshotPath() string {
	return filepath.Join(line.RunDir(l.Root, l.Plan.Group), snapshotFile)
}

// Snapshot records the worktree's HEAD, refs, hooks and git config ahead of a model session.
func (l *Line) Snapshot() error {
	snapshot, err := check.TakeSnapshot(line.WorktreePath(l.Root, l.Plan.Worktree))
	if err != nil {
		return err
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	path := l.snapshotPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// loadSnapshot reads the group's last snapshot, failing with errNoSnapshot when none was taken.
func (l *Line) loadSnapshot() (check.Snapshot, error) {
	var snapshot check.Snapshot
	data, err := os.ReadFile(l.snapshotPath())
	if errors.Is(err, fs.ErrNotExist) {
		return snapshot, fmt.Errorf("%s: %w", l.Plan.Group, errNoSnapshot)
	}
	if err != nil {
		return snapshot, err
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return snapshot, fmt.Errorf("reading %s: %w", l.snapshotPath(), err)
	}
	return snapshot, nil
}

// Check reruns every check, then compares the git state against the Snapshot taken before the session it
// follows, failing with errNoSnapshot when there is none; once ctx is done it never commits.
func (l *Line) Check(ctx context.Context) ([]string, error) {
	before, err := l.loadSnapshot()
	if err != nil {
		return nil, err
	}
	fixes, err := l.rerun(ctx)
	if err != nil {
		return nil, err
	}
	after, err := check.TakeSnapshot(line.WorktreePath(l.Root, l.Plan.Worktree))
	if err != nil {
		return nil, err
	}
	fixes = append(fixes, check.Compare(before, after)...)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(fixes) > 0 {
		l.checked = nil
		return fixes, nil
	}
	l.checked = passed(l.ran)
	// Committing runs the repo's own pre-commit hooks; a refusal is a fix for the builder, not an escalation.
	if err := line.CommitBuild(l.Root, l.Plan); err != nil {
		return []string{"the build does not commit: " + err.Error()}, nil
	}
	return nil, nil
}

// rerun reruns the compile gates, the verify command and scope, then scans the worktree's diff for
// secrets and changed-line coverage, killing its commands once ctx is done.
func (l *Line) rerun(ctx context.Context) ([]string, error) {
	worktree := line.WorktreePath(l.Root, l.Plan.Worktree)
	base := line.StartRef(worktree, l.Plan.Base)
	// Ship stages, or removes, the group's own file, so it stays in scope.
	files := []string{backlog.GroupFilesDir}
	for _, task := range l.Plan.Tasks {
		files = append(files, task.Files...)
	}
	checks := append(line.CompileCommands(l.Root, worktree), line.VerifyCommand(l.Root, worktree))
	l.ran = checks
	fixes := check.RunContext(ctx, check.Group{Worktree: worktree, Base: base, Files: files}, "", "", checks)
	if base == "" {
		return fixes, nil
	}
	diff, err := check.Diff(worktree, base)
	if err != nil {
		return nil, err
	}
	fixes = append(fixes, check.Secrets(diff)...)
	coverage, err := l.coverage(ctx, worktree, diff)
	if err != nil {
		return nil, err
	}
	return append(fixes, coverage...), nil
}

// coveredPackages lists each Go package the diff adds lines to, skipping any directory go test ./... skips:
// testdata, and names starting with a dot or an underscore.
func coveredPackages(diff string) []string {
	seen := map[string]bool{}
	var packages []string
	for _, added := range check.ParseAddedLines(diff) {
		dir := filepath.ToSlash(filepath.Dir(added.File))
		if !strings.HasSuffix(added.File, ".go") || seen[dir] || ignoredByGo(dir) {
			continue
		}
		seen[dir] = true
		packages = append(packages, "./"+dir)
	}
	return packages
}

// ignoredByGo reports whether a slash path holds a directory the go tool leaves out of ./... patterns.
func ignoredByGo(dir string) bool {
	for _, part := range strings.Split(dir, "/") {
		if part == "testdata" || strings.HasPrefix(part, ".") && part != "." || strings.HasPrefix(part, "_") {
			return true
		}
	}
	return false
}

// coverage runs the touched Go packages' tests under a cover profile and evaluates the diff's
// changed-line coverage against the calibrated bar; a worktree with no go.mod is skipped.
func (l *Line) coverage(ctx context.Context, worktree, diff string) ([]string, error) {
	module, err := check.ModulePath(worktree)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	packages := coveredPackages(diff)
	if len(packages) == 0 {
		return nil, nil
	}
	out, err := os.CreateTemp("", "komodo-cover-*.out")
	if err != nil {
		return nil, err
	}
	if err := out.Close(); err != nil {
		return nil, err
	}
	defer os.Remove(out.Name())
	args := append([]string{"test", "-count=1", "-coverprofile=" + out.Name()}, packages...)
	if ran := check.Exec(ctx, worktree, check.CommandTimeout, "go", args...); !ran.OK() {
		return []string{fmt.Sprintf("coverage: `go %s` %v\n%s", strings.Join(args, " "), ran.Err(), ran.Output)}, nil
	}
	text, err := os.ReadFile(out.Name())
	if err != nil {
		return nil, err
	}
	profile, err := check.ParseCoverProfile(string(text), module)
	if err != nil {
		return nil, err
	}
	percent, known := check.ChangedLineCoverage(diff, profile)
	if !known {
		return nil, nil
	}
	dir := filepath.Join(l.Root, line.StateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return check.Evaluate(filepath.Join(dir, coverageBarFile), percent)
}

// Prepare reruns every check, marks each task DONE, commits the group, runs its hooks, rebases it and test-merges
// every ready group; a refusal, a conflict or a breakage is a fix. It never pushes.
func (l *Line) Prepare(ctx context.Context) ([]string, error) {
	fixes, err := l.rerun(ctx)
	if err != nil || len(fixes) > 0 {
		return fixes, err
	}
	for _, task := range l.Plan.Tasks {
		if err := line.RecordStatus(l.Root, task.ID, "DONE"); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if fixes, err := line.PrepareGroup(l.Root, l.Plan); err != nil || len(fixes) > 0 {
		return fixes, err
	}
	return l.integrate(ctx)
}

// Head returns the group's worktree HEAD commit.
func (l *Line) Head() (string, error) {
	return git.Run(line.WorktreePath(l.Root, l.Plan.Worktree), "rev-parse", "HEAD")
}

// Diff returns the group's worktree diff since a commit, uncommitted and untracked files included.
func (l *Line) Diff(since string) (string, error) {
	return check.Diff(line.WorktreePath(l.Root, l.Plan.Worktree), since)
}

// Ship commits, pushes and opens the group's draft PR, whose body reports the checks this process ran;
// a run resumed at Ship reruns them first. Once ctx is done it ships nothing.
func (l *Line) Ship(ctx context.Context) error {
	if l.checked == nil || !l.checked.OK {
		// Only the checks rerun: the build has committed since the last session, so no snapshot applies.
		fixes, err := l.rerun(ctx)
		if err != nil {
			return err
		}
		if len(fixes) > 0 {
			return fmt.Errorf("the checks fail at ship: %s", strings.Join(fixes, "; "))
		}
		l.checked = passed(l.ran)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	shipped, err := line.ShipGroup(l.Root, l.Plan, []*line.WaveResult{l.checked}, l.Client)
	l.shipped = shipped
	return err
}

// Merge merges the group's PR into its epic branch by a merge commit, when it shipped on that base.
func (l *Line) Merge() (bool, error) {
	base := l.Plan.Base
	if l.shipped != nil {
		base = l.shipped.Base
	}
	epic := line.EpicBranchName(l.Plan.Version)
	if epic == "" || base != epic {
		return false, nil
	}
	if _, err := line.MergeGroup(l.Root, l.Plan, l.Client); err != nil {
		return false, err
	}
	return true, nil
}
