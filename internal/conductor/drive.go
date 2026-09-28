package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/changelog"
	"komodo/internal/check"
	"komodo/internal/git"
	"komodo/internal/ledger"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/pr"
	"komodo/internal/proc"
)

// Ledger stations for the conductor's model sessions; a cold review is review, a warm one re-review.
const (
	StationBuild    = "build"
	StationReview   = "review"
	StationReReview = "re-review"
	StationRepair   = "repair"
)

// warmRounds is how many rounds a warm reviewer runs before the group earns its one cold pass.
const warmRounds = 2

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
	// Check reruns every check and returns one fix per failure, none when all passed.
	Check() ([]string, error)
	// Prepare verifies the group's integrated worktree and returns one fix per failure.
	Prepare() ([]string, error)
	// Ship pushes the group and opens its draft PR.
	Ship() error
	// Head returns the worktree's HEAD commit, which a review records as the commit it saw.
	Head() (string, error)
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
	// Review builds the reviewer's request from the group's diff as it stands at review; nil uses Reviewer.
	Review func() (mount.StartRequest, error)
	// ReReview builds the input a resumed reviewer reads from the group's state; nil sends the open findings alone.
	ReReview func(State) (string, error)
	// SeverityFloor is the lowest review severity that blocks; empty blocks every finding.
	SeverityFloor string
	// Repairs is how many repair rounds a group gets before it escalates; zero means line.MaxRepairs.
	Repairs int
	// Save writes the group's state to state.json.
	Save func(State) error
	// WriteReview keeps the reviewer's whole result where Ship's findings gate reads it; nil skips it.
	WriteReview func(group string, result mount.Result) error
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
		if err := d.Stations.Snapshot(); err != nil {
			return err
		}
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
	case Shipped:
		merged, err := d.Stations.Merge()
		if err != nil {
			return err
		}
		s.Merged = merged
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

// review runs one review round, then, once a warm reviewer has passed a later round, the group's one
// cold pass over the whole diff, so a reviewer anchored on its own earlier view never ships a group alone.
func (d *Driver) review(ctx context.Context, s *State, r *round) error {
	if err := d.reviewRound(ctx, s, r); err != nil {
		return err
	}
	if s.ColdPass || s.ReviewRounds < warmRounds || len(r.fixes) > 0 {
		return nil
	}
	s.ColdPass, s.Reviewer = true, ""
	return d.reviewRound(ctx, s, r)
}

// reviewRound resumes the group's reviewer, or starts a cold one when there is none or the resume fails,
// then turns its findings at or above the floor into the fix list.
func (d *Driver) reviewRound(ctx context.Context, s *State, r *round) error {
	station, request := StationReReview, d.Reviewer
	var handle mount.Handle
	var err error
	if s.Reviewer != "" && d.Host.Capabilities().Resume {
		input := line.OpenFindings(s.Open())
		if d.ReReview != nil {
			if input, err = d.ReReview(*s); err != nil {
				return err
			}
		}
		handle, err = d.Host.Resume(mount.Handle(s.Reviewer), input)
	}
	// A reviewer from an earlier process is gone after a restart; a cold one gets the open findings.
	if handle == "" {
		station, s.ReviewRounds = StationReview, 0
		if d.Review != nil {
			if request, err = d.Review(); err != nil {
				return err
			}
		}
		if open := s.Open(); len(open) > 0 {
			request.Brief += "\n\n" + line.OpenFindings(open)
		}
		handle, err = d.Host.Start(request)
	}
	if err != nil {
		return err
	}
	head, err := d.Stations.Head()
	if err != nil {
		return err
	}
	s.Reviewer, s.Reviewed = string(handle), head
	s.ReviewRounds++
	result, err := d.session(ctx, s, station, request, handle)
	if err != nil {
		return err
	}
	if d.WriteReview != nil {
		if err := d.WriteReview(s.Group, result); err != nil {
			return err
		}
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
		s.Findings = append(s.Findings, Finding{
			Severity: finding.Severity, Verified: verified, File: finding.File, Line: finding.Line, Title: finding.Title,
		})
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
	input := fixList(r.fixes)
	if err := d.Stations.Snapshot(); err != nil {
		return err
	}
	req := d.Builder
	var handle mount.Handle
	var err error
	if r.builder != "" && d.Host.Capabilities().Resume {
		handle, err = d.Host.Resume(r.builder, input)
	}
	// A builder from an earlier process is gone after a restart; a fresh one gets the brief and fixes.
	if handle == "" {
		req.Brief = repairBrief(input, req.Brief)
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
// follows, failing with errNoSnapshot when there is none.
func (l *Line) Check() ([]string, error) {
	before, err := l.loadSnapshot()
	if err != nil {
		return nil, err
	}
	fixes, err := l.rerun()
	if err != nil {
		return nil, err
	}
	after, err := check.TakeSnapshot(line.WorktreePath(l.Root, l.Plan.Worktree))
	if err != nil {
		return nil, err
	}
	fixes = append(fixes, check.Compare(before, after)...)
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
// secrets and changed-line coverage.
func (l *Line) rerun() ([]string, error) {
	worktree := line.WorktreePath(l.Root, l.Plan.Worktree)
	base := line.StartRef(worktree, l.Plan.Base)
	// Ship also stages BACKLOG.md and the group's release note, so both count as in scope.
	files := []string{filepath.ToSlash(changelog.FragmentPath("", l.Plan.Version, l.Plan.Group))}
	if found, err := backlog.Find(worktree); err == nil {
		if rel, err := filepath.Rel(worktree, found); err == nil {
			files = append(files, filepath.ToSlash(rel))
		}
	}
	for _, task := range l.Plan.Tasks {
		files = append(files, task.Files...)
	}
	checks := append(line.CompileCommands(l.Root, worktree), line.VerifyCommand(l.Root, worktree))
	l.ran = checks
	fixes := check.Run(check.Group{Worktree: worktree, Base: base, Files: files}, "", "", checks)
	if base == "" {
		return fixes, nil
	}
	diff, err := check.Diff(worktree, base)
	if err != nil {
		return nil, err
	}
	fixes = append(fixes, check.Secrets(diff)...)
	coverage, err := l.coverage(worktree, diff)
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
func (l *Line) coverage(worktree, diff string) ([]string, error) {
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
	if ran := proc.Exec(worktree, check.CommandTimeout, "go", args...); !ran.OK() {
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

// Prepare reruns every check in the worktree the one group builder worked, since there are no task
// branches to merge, then marks each task DONE so Ship commits and reports them.
func (l *Line) Prepare() ([]string, error) {
	fixes, err := l.rerun()
	if err != nil || len(fixes) > 0 {
		return fixes, err
	}
	for _, task := range l.Plan.Tasks {
		if err := line.RecordStatus(l.Root, task.ID, "DONE"); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// Head returns the group's worktree HEAD commit.
func (l *Line) Head() (string, error) {
	return git.Run(line.WorktreePath(l.Root, l.Plan.Worktree), "rev-parse", "HEAD")
}

// Ship commits, pushes and opens the group's draft PR, whose body reports the checks this process ran;
// a run resumed at Ship reruns them first.
func (l *Line) Ship() error {
	if l.checked == nil || !l.checked.OK {
		// Only the checks rerun: the build has committed since the last session, so no snapshot applies.
		fixes, err := l.rerun()
		if err != nil {
			return err
		}
		if len(fixes) > 0 {
			return fmt.Errorf("the checks fail at ship: %s", strings.Join(fixes, "; "))
		}
		l.checked = passed(l.ran)
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
