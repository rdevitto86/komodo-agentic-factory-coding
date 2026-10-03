package conductor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"komodo/internal/ledger"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/review"
)

// fakeHost is a canned host: each session streams one usage event and returns the next scripted result.
type fakeHost struct {
	resume   bool
	saved    *[]State
	builds   []map[string]any
	repairs  []map[string]any
	reviews  []map[string]any
	results  map[mount.Handle]mount.Result
	starts   []mount.StartRequest
	inputs   []string
	stopped  []mount.Handle
	stopMu   sync.Mutex
	startsAt []GroupState
	next     int
	hang     bool
	cancel   context.CancelFunc
	// resumed and reReviews are the reviewer sessions resumed and the input each got.
	resumed   []mount.Handle
	reReviews []string
	// together, when set, holds each reviewer stream until that many are open at once.
	together int
	waiting  int
	release  chan struct{}
	gatherMu sync.Mutex
	// rateLimit, when set, rides the builder's next streamed event, as a session's own rate_limit_event would.
	rateLimit *mount.RateLimit
}

// gatherTimeout bounds how long a reviewer stream waits for the rest of its round to open.
const gatherTimeout = 10 * time.Second

// gather holds a reviewer stream until together of them are open at once, which only parallel lenses reach.
func (f *fakeHost) gather() error {
	f.gatherMu.Lock()
	if f.release == nil {
		f.release = make(chan struct{})
	}
	f.waiting++
	release := f.release
	if f.waiting == f.together {
		close(f.release)
		f.release, f.waiting = nil, 0
	}
	f.gatherMu.Unlock()
	select {
	case <-release:
		return nil
	case <-time.After(gatherTimeout):
		return errors.New("the lens sessions did not run in parallel")
	}
}

// newFakeHost returns a fake host that resumes sessions and reads the states saved so far from saved.
func newFakeHost(saved *[]State) *fakeHost {
	return &fakeHost{resume: true, saved: saved, results: map[mount.Handle]mount.Result{}}
}

// pop takes the next scripted result, or fallback once the script runs out.
func pop(script *[]map[string]any, fallback map[string]any) map[string]any {
	if len(*script) == 0 {
		return fallback
	}
	value := (*script)[0]
	*script = (*script)[1:]
	return value
}

// handle names a new session and records the state saved when it began.
func (f *fakeHost) handle(role string, value map[string]any) mount.Handle {
	f.next++
	handle := mount.Handle(fmt.Sprintf("%s-%d", role, f.next))
	f.results[handle] = mount.Result{Value: value}
	f.startsAt = append(f.startsAt, (*f.saved)[len(*f.saved)-1].Current)
	return handle
}

func (f *fakeHost) Preflight(ctx context.Context) error { return nil }

func (f *fakeHost) Start(ctx context.Context, req mount.StartRequest) (mount.Handle, error) {
	f.starts = append(f.starts, req)
	if f.cancel != nil {
		f.cancel()
	}
	if req.Role == "reviewer" {
		return f.handle(req.Role, pop(&f.reviews, map[string]any{"findings": []any{}})), nil
	}
	return f.handle(req.Role, pop(&f.builds, map[string]any{"result": "DONE"})), nil
}

func (f *fakeHost) Resume(ctx context.Context, handle mount.Handle, input string) (mount.Handle, error) {
	if _, ok := f.results[handle]; !ok {
		return "", errors.New("no such session")
	}
	if strings.HasPrefix(string(handle), "reviewer") {
		f.resumed = append(f.resumed, handle)
		f.reReviews = append(f.reReviews, input)
		return f.handle("reviewer", pop(&f.reviews, map[string]any{"findings": []any{}})), nil
	}
	f.inputs = append(f.inputs, input)
	return f.handle("builder", pop(&f.repairs, map[string]any{"result": "DONE"})), nil
}

func (f *fakeHost) Stream(ctx context.Context, handle mount.Handle) (<-chan mount.Event, error) {
	if f.hang {
		return make(chan mount.Event), nil
	}
	if f.together > 0 && strings.HasPrefix(string(handle), "reviewer") {
		if err := f.gather(); err != nil {
			return nil, err
		}
	}
	out := make(chan mount.Event, 1)
	out <- mount.Event{Turns: 2, Usage: mount.TaskUsage{TokensIn: 100, TokensOut: 20, Turns: 2}, RateLimit: f.rateLimit}
	close(out)
	return out, nil
}

func (f *fakeHost) Result(handle mount.Handle) (mount.Result, error) {
	result, ok := f.results[handle]
	if !ok {
		return mount.Result{}, errors.New("no such session")
	}
	return result, nil
}

func (f *fakeHost) Stop(ctx context.Context, handle mount.Handle) error {
	f.stopMu.Lock()
	defer f.stopMu.Unlock()
	f.stopped = append(f.stopped, handle)
	return nil
}

func (f *fakeHost) Capabilities() mount.Capabilities {
	return mount.Capabilities{Resume: f.resume, Sandbox: true, Hooks: true, Structured: true}
}

// fakeStations scripts Check and Prepare fixes, Ship and Merge outcomes, and records the state each began in.
type fakeStations struct {
	saved    *[]State
	checks   [][]string
	prepares [][]string
	shipErr  error
	headErr  error
	merged   bool
	mergeErr error
	calledAt []string
	// repair is the diff Diff returns, and diffedSince each commit it was asked for.
	repair      string
	diffedSince []string
	// idle leaves the worktree as it was across every session, as a builder that edits nothing does.
	idle bool
}

// record notes which station ran and the state saved before it.
func (f *fakeStations) record(name string) {
	f.calledAt = append(f.calledAt, name+"@"+string((*f.saved)[len(*f.saved)-1].Current))
}

func (f *fakeStations) Snapshot() error {
	f.record("snapshot")
	return nil
}

func (f *fakeStations) Check(context.Context) ([]string, error) {
	f.record("check")
	if len(f.checks) == 0 {
		return nil, nil
	}
	fixes := f.checks[0]
	f.checks = f.checks[1:]
	return fixes, nil
}

func (f *fakeStations) Prepare(context.Context) ([]string, error) {
	f.record("prepare")
	if len(f.prepares) == 0 {
		return nil, nil
	}
	fixes := f.prepares[0]
	f.prepares = f.prepares[1:]
	return fixes, nil
}

func (f *fakeStations) Ship(context.Context) error {
	f.record("ship")
	return f.shipErr
}

func (f *fakeStations) Head() (string, error) {
	if f.headErr != nil {
		return "", f.headErr
	}
	return fmt.Sprintf("commit-%d", len(*f.saved)), nil
}

func (f *fakeStations) Diff(since string) (string, error) {
	f.diffedSince = append(f.diffedSince, since)
	if f.idle {
		return f.repair, nil
	}
	// Each session edits the worktree, so the diff grows by one line of work.txt per session saved so far.
	edits := len((*f.saved)[len(*f.saved)-1].Sessions)
	return f.repair + fmt.Sprintf("--- a/work.txt\n+++ b/work.txt\n@@ -0,0 +1,%d @@\n", edits) +
		strings.Repeat("+edit\n", edits), nil
}

func (f *fakeStations) Merge() (bool, error) {
	f.record("merge")
	return f.merged, f.mergeErr
}

// rig is one test's driver with its fake host, fake stations, ledger and every saved state.
type rig struct {
	driver   *Driver
	host     *fakeHost
	stations *fakeStations
	saved    *[]State
}

// newRig wires a Driver to a fake host, fake stations and a ledger in a temporary directory.
func newRig(t *testing.T) *rig {
	t.Helper()
	saved := &[]State{}
	host := newFakeHost(saved)
	stations := &fakeStations{saved: saved}
	driver := &Driver{
		Host:          host,
		Stations:      stations,
		Ledger:        ledger.New(t.TempDir()),
		Run:           "run-1",
		Builder:       mount.StartRequest{Role: "builder", Brief: "build TG-1", Model: "sonnet"},
		Reviewer:      mount.StartRequest{Role: "reviewer", Brief: "review TG-1", Model: "opus"},
		SeverityFloor: "high",
		Save: func(s State) error {
			*saved = append(*saved, s)
			return nil
		},
	}
	return &rig{driver: driver, host: host, stations: stations, saved: saved}
}

// transitions lists the states saved, with each run of repeated saves of one state collapsed.
func (r *rig) transitions() []GroupState {
	var out []GroupState
	for _, s := range *r.saved {
		if len(out) == 0 || out[len(out)-1] != s.Current {
			out = append(out, s.Current)
		}
	}
	return out
}

// sessions reads the ledger and returns the station of each row, failing if any row is not a session.
func (r *rig) sessions(t *testing.T) []string {
	t.Helper()
	entries, err := r.driver.Ledger.Read(ledger.RunFile)
	if err != nil {
		t.Fatalf("reading the ledger: %v", err)
	}
	var stations []string
	for _, entry := range entries {
		switch entry.Station {
		case StationBuild, StationReview, StationReReview, StationRepair:
		default:
			t.Fatalf("the ledger holds a %q row; only build, review and repair sessions belong there", entry.Station)
		}
		if entry.Run != "run-1" || entry.Group != "TG-1" || entry.TokensIn != 100 || entry.Turns != 2 {
			t.Fatalf("session row = %+v, want the run, group, tokens and turns of its session", entry)
		}
		stations = append(stations, entry.Station)
	}
	return stations
}

// drive runs the rig's driver from a Ready group with a slot free.
func (r *rig) drive(t *testing.T) (State, error) {
	t.Helper()
	return r.driver.Drive(context.Background(), State{Group: "TG-1", Current: Ready, SlotFree: true})
}

// equal reports whether two string-like slices hold the same values in order.
func equal[T ~string](got, want []T) bool {
	return strings.Join(toStrings(got), ",") == strings.Join(toStrings(want), ",")
}

// toStrings converts a slice of a string type to plain strings.
func toStrings[T ~string](values []T) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, string(value))
	}
	return out
}

func TestDriveRunsAGroupFromReadyToShipped(t *testing.T) {
	r := newRig(t)
	final, err := r.drive(t)
	if err != nil {
		t.Fatalf("drive = %v", err)
	}
	if final.Current != Shipped {
		t.Fatalf("final state = %s, want Shipped", final.Current)
	}
	want := []GroupState{Building, Checking, Reviewing, Preparing, Shipping, Shipped}
	if got := r.transitions(); !equal(got, want) {
		t.Fatalf("transitions = %v, want %v", got, want)
	}
	if got := r.sessions(t); !equal(got, []string{StationBuild, StationReview}) {
		t.Fatalf("ledger sessions = %v, want build then review", got)
	}
	if !equal(r.host.startsAt, []GroupState{Building, Reviewing}) {
		t.Fatalf("sessions started at %v, want each after its state was saved", r.host.startsAt)
	}
	stations := []string{"snapshot@Building", "check@Checking", "prepare@Preparing", "ship@Shipping", "merge@Shipped"}
	if !equal(r.stations.calledAt, stations) {
		t.Fatalf("stations ran at %v, want %v", r.stations.calledAt, stations)
	}
	if len(final.Sessions) != 2 {
		t.Fatalf("sessions = %v, want the builder's and the reviewer's", final.Sessions)
	}
}

func TestDriveFeedsASessionsRateLimitEventToPacing(t *testing.T) {
	mount.ClearRateLimit()
	t.Cleanup(mount.ClearRateLimit)
	reset := time.Now().Add(time.Hour).Truncate(time.Second)
	r := newRig(t)
	r.host.rateLimit = &mount.RateLimit{FiveHour: 0.95, ResetsAt: reset}
	if _, err := r.drive(t); err != nil {
		t.Fatalf("drive = %v", err)
	}
	limit, ok := mount.LatestRateLimit()
	if !ok {
		t.Fatal("the driver's stream never fed pacing a rate_limit_event")
	}
	if limit.FiveHour != 0.95 || !limit.ResetsAt.Equal(reset) {
		t.Fatalf("limit = %+v", limit)
	}
}

func TestDriveRepairsAFailedCheckByResumingTheBuilder(t *testing.T) {
	r := newRig(t)
	r.stations.checks = [][]string{{"`go vet` exited 1"}}
	final, err := r.drive(t)
	if err != nil || final.Current != Shipped {
		t.Fatalf("drive = %s, %v; want Shipped", final.Current, err)
	}
	want := []GroupState{Building, Checking, Repairing, Checking, Reviewing, Preparing, Shipping, Shipped}
	if got := r.transitions(); !equal(got, want) {
		t.Fatalf("transitions = %v, want %v", got, want)
	}
	if got := r.sessions(t); !equal(got, []string{StationBuild, StationRepair, StationReview}) {
		t.Fatalf("ledger sessions = %v, want build, repair, review", got)
	}
	if len(r.host.inputs) != 1 || !strings.Contains(r.host.inputs[0], "- [ ] `go vet` exited 1") {
		t.Fatalf("resume inputs = %q, want the failed check as a fix list", r.host.inputs)
	}
	stations := []string{
		"snapshot@Building", "check@Checking", "snapshot@Repairing", "check@Checking",
		"prepare@Preparing", "ship@Shipping", "merge@Shipped",
	}
	if !equal(r.stations.calledAt, stations) {
		t.Fatalf("stations ran at %v, want a snapshot before each session and a check after it", r.stations.calledAt)
	}
}

// TestDriveNeverReviewsBeforeCheckPasses is REQ-17: the ledger records no review session until
// Check has passed, even across repeated failed checks and their repair rounds.
func TestDriveNeverReviewsBeforeCheckPasses(t *testing.T) {
	r := newRig(t)
	r.stations.checks = [][]string{{"fail 1"}, {"fail 2"}}
	final, err := r.drive(t)
	if err != nil || final.Current != Shipped {
		t.Fatalf("drive = %s, %v; want Shipped", final.Current, err)
	}
	sessions := r.sessions(t)
	reviewIndex := -1
	for i, station := range sessions {
		if station == StationReview {
			reviewIndex = i
			break
		}
	}
	if reviewIndex == -1 {
		t.Fatalf("ledger sessions = %v, want a review session", sessions)
	}
	for _, station := range sessions[:reviewIndex] {
		if station == StationReview {
			t.Fatalf("ledger sessions = %v, want no review before %d", sessions, reviewIndex)
		}
	}
	checkCalls := 0
	for _, call := range r.stations.calledAt {
		if strings.HasPrefix(call, "check@") {
			checkCalls++
		}
	}
	if checkCalls != 3 {
		t.Fatalf("check calls = %d, want 3: two failures and the pass that unblocks review", checkCalls)
	}
	if !equal(sessions, []string{StationBuild, StationRepair, StationRepair, StationReview}) {
		t.Fatalf("ledger sessions = %v, want build, two repairs, then review", sessions)
	}
}

func TestDriveRepairsAVerifiedFindingAndReviewsAgain(t *testing.T) {
	r := newRig(t)
	r.host.reviews = []map[string]any{
		{"findings": []any{
			map[string]any{"severity": "high", "file": "a.go", "line": 3, "title": "nil map", "fix": "make it"},
			map[string]any{"severity": "low", "file": "b.go", "line": 9, "title": "name", "fix": "rename"},
		}},
	}
	final, err := r.drive(t)
	if err != nil || final.Current != Shipped {
		t.Fatalf("drive = %s, %v; want Shipped", final.Current, err)
	}
	want := []GroupState{
		Building, Checking, Reviewing, Repairing, Checking, Reviewing, Preparing, Shipping, Shipped,
	}
	if got := r.transitions(); !equal(got, want) {
		t.Fatalf("transitions = %v, want %v", got, want)
	}
	sessions := []string{StationBuild, StationReview, StationRepair, StationReReview, StationReview}
	if got := r.sessions(t); !equal(got, sessions) {
		t.Fatalf("ledger sessions = %v, want %v", got, sessions)
	}
	input := r.host.inputs[0]
	if !strings.Contains(input, "a.go:3 nil map: make it") || strings.Contains(input, "b.go") {
		t.Fatalf("fix list = %q, want only the finding at or above the floor", input)
	}
}

func TestDriveResumesTheFirstReviewerForTheSecondReview(t *testing.T) {
	r := newRig(t)
	r.host.reviews = []map[string]any{
		{"findings": []any{map[string]any{"severity": "high", "file": "a.go", "line": 3, "title": "nil map"}}},
	}
	if _, err := r.drive(t); err != nil {
		t.Fatalf("drive = %v", err)
	}
	var first mount.Handle
	for _, s := range *r.saved {
		if first == "" && s.Reviewer[review.Economy] != "" {
			first = mount.Handle(s.Reviewer[review.Economy])
		}
	}
	if len(r.host.resumed) != 1 || r.host.resumed[0] != first {
		t.Fatalf("resumed reviewers = %v, want the first reviewer %s once", r.host.resumed, first)
	}
	if input := r.host.reReviews[0]; !strings.Contains(input, "`a.go:3` high: nil map") {
		t.Fatalf("re-review input = %q, want the open finding by file and line", input)
	}
}

func TestDriveStartsAColdReviewerWhenItsReviewerIsGone(t *testing.T) {
	r := newRig(t)
	saved := State{
		Group: "TG-1", Current: Checking,
		Reviewer:     map[review.Lens]string{review.Economy: "reviewer-from-an-earlier-process"},
		ReviewRounds: map[review.Lens]int{review.Economy: 1},
		Findings: map[review.Lens][]Finding{
			review.Economy: {{Severity: "high", Verified: true, File: "a.go", Line: 3, Title: "nil map"}},
		},
	}
	*r.saved = append(*r.saved, saved)
	final, err := r.driver.Resume(context.Background(), saved)
	if err != nil || final.Current != Shipped {
		t.Fatalf("resume = %s, %v; want Shipped", final.Current, err)
	}
	if len(r.host.starts) != 1 || r.host.starts[0].Role != "reviewer" {
		t.Fatalf("starts = %+v, want one fresh reviewer", r.host.starts)
	}
	if brief := r.host.starts[0].Brief; !strings.HasPrefix(brief, "review TG-1") || !strings.Contains(brief, "`a.go:3`") {
		t.Fatalf("cold brief = %q, want the review brief with the open findings", brief)
	}
	if got := r.sessions(t); !equal(got, []string{StationReview}) {
		t.Fatalf("ledger sessions = %v, want one cold review and no cold pass after it", got)
	}
}

func TestDriveStartsAColdReviewerOnAHostWithoutResume(t *testing.T) {
	r := newRig(t)
	r.host.resume = false
	r.host.reviews = []map[string]any{
		{"findings": []any{map[string]any{"severity": "high", "file": "a.go", "line": 3, "title": "nil map"}}},
	}
	if _, err := r.drive(t); err != nil {
		t.Fatalf("drive = %v", err)
	}
	if len(r.host.resumed) != 0 {
		t.Fatalf("resumed %v on a host without resume", r.host.resumed)
	}
	if got := r.sessions(t); !equal(got, []string{StationBuild, StationReview, StationRepair, StationReview}) {
		t.Fatalf("ledger sessions = %v, want two cold reviews and no cold pass", got)
	}
}

func TestDriveBuildsEachReviewFromItsWiredRequests(t *testing.T) {
	r := newRig(t)
	r.host.reviews = []map[string]any{
		{"findings": []any{map[string]any{"severity": "high", "file": "a.go", "line": 3, "title": "nil map"}}},
	}
	r.driver.Review = func(review.Lens) (mount.StartRequest, error) {
		return mount.StartRequest{Role: "reviewer", Brief: "the group's diff"}, nil
	}
	r.driver.ReReview = func(_ review.Lens, s State) (string, error) {
		return "since " + s.Reviewed, nil
	}
	if _, err := r.drive(t); err != nil {
		t.Fatalf("drive = %v", err)
	}
	if r.host.starts[1].Brief != "the group's diff" {
		t.Fatalf("cold brief = %q, want the one Review built", r.host.starts[1].Brief)
	}
	if len(r.host.reReviews) != 1 || !strings.HasPrefix(r.host.reReviews[0], "since commit-") {
		t.Fatalf("re-review inputs = %q, want the one ReReview built from the reviewed commit", r.host.reReviews)
	}
}

func TestDriveEscalatesAReviewThatCannotStart(t *testing.T) {
	failed := errors.New("no diff")
	high := map[string]any{"severity": "high", "file": "a.go", "line": 3, "title": "nil map"}
	cases := []struct {
		name string
		wire func(r *rig)
	}{
		{"the review request fails", func(r *rig) {
			r.driver.Review = func(review.Lens) (mount.StartRequest, error) { return mount.StartRequest{}, failed }
		}},
		{"the re-review input fails", func(r *rig) {
			r.host.reviews = []map[string]any{{"findings": []any{high}}}
			r.driver.ReReview = func(review.Lens, State) (string, error) { return "", failed }
		}},
		{"the reviewed HEAD cannot be read", func(r *rig) {
			r.stations.headErr = failed
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t)
			tc.wire(r)
			final, err := r.drive(t)
			if !errors.Is(err, failed) {
				t.Fatalf("drive error = %v, want %v", err, failed)
			}
			if final.Current != Escalated || final.Left != Reviewing {
				t.Fatalf("final = %s left %s, want Escalated from Reviewing", final.Current, final.Left)
			}
		})
	}
}

func TestLineHeadReadsTheWorktreesHead(t *testing.T) {
	root := checkRepo(t)
	stations := &Line{Root: root, Plan: &line.Plan{Group: "TG-1", Worktree: root}}
	head, err := stations.Head()
	if err != nil || len(head) != 40 {
		t.Fatalf("head = %q, %v; want the commit's full hash", head, err)
	}
}

func TestDriveRunsOneColdPassBeforePreparingAfterTwoWarmRounds(t *testing.T) {
	high := map[string]any{"severity": "high", "file": "a.go", "line": 3, "title": "nil map"}
	cases := []struct {
		name     string
		reviews  []map[string]any
		sessions []string
	}{
		{
			name:     "passed its first review",
			sessions: []string{StationBuild, StationReview},
		},
		{
			name:     "two warm rounds",
			reviews:  []map[string]any{{"findings": []any{high}}},
			sessions: []string{StationBuild, StationReview, StationRepair, StationReReview, StationReview},
		},
		{
			name:    "a cold pass that blocks",
			reviews: []map[string]any{{"findings": []any{high}}, {"findings": []any{}}, {"findings": []any{high}}},
			sessions: []string{
				StationBuild, StationReview, StationRepair, StationReReview, StationReview, StationRepair, StationReReview,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t)
			r.host.reviews = tc.reviews
			final, err := r.drive(t)
			if err != nil || final.Current != Shipped {
				t.Fatalf("drive = %s, %v; want Shipped", final.Current, err)
			}
			if got := r.sessions(t); !equal(got, tc.sessions) {
				t.Fatalf("ledger sessions = %v, want %v", got, tc.sessions)
			}
			for i, state := range r.host.startsAt {
				if state == Preparing || state == Shipping {
					t.Fatalf("session %d began at %s, want every review before Preparing", i, state)
				}
			}
		})
	}
}

func TestDriveRunsEveryLensInParallelWithItsOwnSessionRoundsAndFindings(t *testing.T) {
	r := newRig(t)
	lenses := review.ForMode("full")
	r.driver.Lenses = lenses
	r.driver.Review = func(lens review.Lens) (mount.StartRequest, error) {
		return mount.StartRequest{Role: "reviewer", Brief: "the diff and the task list, through " + string(lens)}, nil
	}
	r.host.together = len(lenses)
	high := map[string]any{
		"lens": "security", "severity": "high", "file": "a.go", "line": 3, "title": "no escaping", "fix": "escape it",
	}
	low := map[string]any{"severity": "low", "file": "b.go", "line": 9, "title": "name"}
	r.host.reviews = []map[string]any{
		{"findings": []any{}, "blast_radius": "med"},
		{"findings": []any{high}, "blast_radius": "high", "blast_radius_why": "crosses a trust boundary"},
		{"findings": []any{low}, "blast_radius": "low"},
	}
	var written []mount.Result
	r.driver.WriteReview = func(_ string, result mount.Result) error {
		written = append(written, result)
		return nil
	}

	final, err := r.drive(t)
	if err != nil || final.Current != Shipped {
		t.Fatalf("drive = %s, %v; want Shipped", final.Current, err)
	}
	sessions := []string{
		StationBuild, StationReview, StationReview, StationReview, StationRepair,
		StationReReview, StationReReview, StationReReview, StationReview, StationReview, StationReview,
	}
	if got := r.sessions(t); !equal(got, sessions) {
		t.Fatalf("ledger sessions = %v, want three lenses, their three re-reviews, then three cold passes", got)
	}
	for i, lens := range lenses {
		brief := r.host.starts[1+i].Brief
		if !strings.HasSuffix(brief, string(lens)) || strings.Contains(brief, "build TG-1") {
			t.Fatalf("%s lens brief = %q, want its own request and never the builder's", lens, brief)
		}
	}
	var reviewed State
	for _, s := range *r.saved {
		if s.Current == Repairing {
			reviewed = s
			break
		}
	}
	if len(reviewed.Reviewer) != len(lenses) {
		t.Fatalf("reviewers = %v, want one session per lens", reviewed.Reviewer)
	}
	for i, lens := range lenses {
		if r.host.resumed[i] != mount.Handle(reviewed.Reviewer[lens]) {
			t.Fatalf("resumed = %v, want each lens's own reviewer %v", r.host.resumed, reviewed.Reviewer)
		}
	}
	if security := reviewed.Findings[review.Security]; len(security) != 1 || !security[0].Verified ||
		security[0].Lens != review.Security {
		t.Fatalf("security findings = %+v, want its one verified finding", security)
	}
	if quality := reviewed.Findings[review.Quality]; len(quality) != 1 || quality[0].Verified ||
		quality[0].Lens != review.Quality {
		t.Fatalf("quality findings = %+v, want its unverified finding under its session's lens", quality)
	}
	if !strings.Contains(r.host.reReviews[1], "`a.go:3` high: no escaping") ||
		strings.Contains(r.host.reReviews[0], "a.go") || strings.Contains(r.host.reReviews[2], "a.go") {
		t.Fatalf("re-review inputs = %q, want each lens given only its own open findings", r.host.reReviews)
	}
	first := written[0].Value
	if merged, _ := first["findings"].([]any); len(merged) != 2 || first["blast_radius"] != "high" ||
		first["blast_radius_why"] != "crosses a trust boundary" {
		t.Fatalf("review written for ship = %+v, want every lens's findings and the widest blast radius", first)
	}
	if fixes := r.host.inputs[0]; !strings.Contains(fixes, "a.go:3 no escaping: escape it") || strings.Contains(fixes, "b.go") {
		t.Fatalf("fix list = %q, want only the verified finding", fixes)
	}
	for _, lens := range lenses {
		if !final.ColdPass[lens] || final.ReviewRounds[lens] != 1 {
			t.Fatalf("cold pass = %v, rounds = %v, want every lens's cold pass run", final.ColdPass, final.ReviewRounds)
		}
	}
}

func TestDriveDropsANewReReviewFindingOnALineTheRepairLeftAlone(t *testing.T) {
	r := newRig(t)
	finding := func(file string, line int) map[string]any {
		return map[string]any{"severity": "high", "file": file, "line": line, "title": "wrong", "fix": "fix it"}
	}
	// The re-review closes e.go:7, so the round progresses and its kept findings reach a second repair.
	r.host.reviews = []map[string]any{
		{"findings": []any{finding("a.go", 3), finding("e.go", 7)}},
		{"findings": []any{finding("a.go", 3), finding("c.go", 1), finding("a.go", 4)}},
	}
	r.stations.repair = "--- a/a.go\n+++ b/a.go\n@@ -4,1 +4,1 @@\n-\told()\n+\tnew()\n"
	var written []mount.Result
	r.driver.WriteReview = func(_ string, result mount.Result) error {
		written = append(written, result)
		return nil
	}
	final, err := r.drive(t)
	if err != nil || final.Current != Shipped {
		t.Fatalf("drive = %s, %v; want Shipped", final.Current, err)
	}
	if len(r.stations.diffedSince) == 0 || !strings.HasPrefix(r.stations.diffedSince[0], "commit-") {
		t.Fatalf("diffed since %v, want the commit the first review saw", r.stations.diffedSince)
	}
	fixes := r.host.inputs[1]
	if !strings.Contains(fixes, "a.go:3") || !strings.Contains(fixes, "a.go:4") || strings.Contains(fixes, "c.go") {
		t.Fatalf("second fix list = %q, want the kept and the repaired-line findings, never the unchanged line's", fixes)
	}
	if kept, ok := written[1].Value["findings"].([]any); !ok || len(kept) != 2 {
		t.Fatalf("re-review written for ship = %+v, want only its two allowed findings", written[1].Value)
	}
}

func TestDriveStopsTheOpenedLensesWhenAnotherCannotStart(t *testing.T) {
	failed := errors.New("no brief")
	r := newRig(t)
	r.driver.Lenses = review.ForMode("full")
	r.driver.Review = func(lens review.Lens) (mount.StartRequest, error) {
		if lens == review.Quality {
			return mount.StartRequest{}, failed
		}
		return mount.StartRequest{Role: "reviewer", Brief: string(lens)}, nil
	}
	final, err := r.drive(t)
	if !errors.Is(err, failed) || final.Current != Escalated {
		t.Fatalf("drive = %s, %v; want an escalation carrying %v", final.Current, err, failed)
	}
	if len(r.host.stopped) != 2 {
		t.Fatalf("stopped = %v, want the two lens sessions opened before the failure", r.host.stopped)
	}
}

func TestDriveResumesAReviewerSavedBeforeLensesAsTheEconomyLens(t *testing.T) {
	legacy := `{"group":"TG-1","state":"Reviewing","reviewer":"reviewer-9","review_rounds":1,` +
		`"findings":[{"severity":"high","verified":true,"file":"a.go","line":3,"title":"nil map"}]}`
	cases := []struct {
		name    string
		lenses  []review.Lens
		resumed int
		starts  int
	}{
		{"economy mode resumes it", review.ForMode("economy"), 1, 1},
		{"full mode starts every lens cold", review.ForMode("full"), 0, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
				t.Fatal(err)
			}
			s, err := LoadState(path)
			if err != nil {
				t.Fatal(err)
			}
			r := newRig(t)
			r.driver.Lenses = tc.lenses
			r.host.results["reviewer-9"] = mount.Result{Value: map[string]any{"findings": []any{}}}
			*r.saved = append(*r.saved, s)
			final, err := r.driver.Resume(context.Background(), s)
			if err != nil || final.Current != Shipped {
				t.Fatalf("resume = %s, %v; want Shipped", final.Current, err)
			}
			if len(r.host.resumed) != tc.resumed || len(r.host.starts) != tc.starts {
				t.Fatalf("resumed %v and started %d, want %d resumed and %d started",
					r.host.resumed, len(r.host.starts), tc.resumed, tc.starts)
			}
			if tc.resumed == 1 && (r.host.resumed[0] != "reviewer-9" || !strings.Contains(r.host.reReviews[0], "`a.go:3`")) {
				t.Fatalf("resumed %v with %q, want the saved reviewer given its open finding", r.host.resumed, r.host.reReviews)
			}
			if len(final.Findings) != len(tc.lenses) {
				t.Fatalf("findings = %v, want one entry per lens the driver runs", final.Findings)
			}
		})
	}
}

func TestDriveRepairsAPrepareConflict(t *testing.T) {
	r := newRig(t)
	r.stations.prepares = [][]string{{"conflict in a.go"}}
	final, err := r.drive(t)
	if err != nil || final.Current != Shipped {
		t.Fatalf("drive = %s, %v; want Shipped", final.Current, err)
	}
	want := []GroupState{
		Building, Checking, Reviewing, Preparing, Repairing, Checking, Reviewing, Preparing, Shipping, Shipped,
	}
	if got := r.transitions(); !equal(got, want) {
		t.Fatalf("transitions = %v, want %v", got, want)
	}
}

func TestDriveStartsAFreshBuilderWhenTheHostCannotResume(t *testing.T) {
	r := newRig(t)
	r.host.resume = false
	r.stations.checks = [][]string{{"`go test` exited 1"}}
	if _, err := r.drive(t); err != nil {
		t.Fatalf("drive = %v", err)
	}
	if len(r.host.inputs) != 0 {
		t.Fatalf("resumed %q on a host without resume", r.host.inputs)
	}
	repair := r.host.starts[1]
	if repair.Role != "builder" || !strings.Contains(repair.Brief, "build TG-1") ||
		!strings.Contains(repair.Brief, "- [ ] `go test` exited 1") {
		t.Fatalf("fresh repair request = %+v, want the builder's brief with the fix list", repair)
	}
	if strings.Index(repair.Brief, "- [ ] `go test` exited 1") > strings.Index(repair.Brief, "build TG-1") {
		t.Fatalf("brief = %q; a fresh repair must lead with its fix list, not bury it after the build brief", repair.Brief)
	}
}

func TestDriveStartsAFreshBuilderWhenItsBuilderIsGone(t *testing.T) {
	r := newRig(t)
	r.stations.checks = [][]string{{"`go test` exited 1"}}
	saved := State{Group: "TG-1", Current: Checking, Builder: "builder-from-an-earlier-process"}
	*r.saved = append(*r.saved, saved)
	if _, err := r.driver.Resume(context.Background(), saved); err != nil {
		t.Fatalf("resume = %v", err)
	}
	if len(r.host.starts) == 0 {
		t.Fatal("no session started; the repair needs a fresh builder")
	}
	repair := r.host.starts[0]
	if repair.Role != "builder" || !strings.Contains(repair.Brief, "- [ ] `go test` exited 1") {
		t.Fatalf("repair request = %+v, want the builder's role with the fix list, never another session's", repair)
	}
}

func TestDriveClosesEachFixListWithEveryTasksFilesAsThePlanHoldsThem(t *testing.T) {
	for _, resume := range []bool{true, false} {
		t.Run(fmt.Sprintf("resume %v", resume), func(t *testing.T) {
			r := newRig(t)
			r.host.resume = resume
			r.host.results["builder-0"] = mount.Result{Value: map[string]any{"result": "DONE"}}
			// The plan loaded on resume holds c.go, a file the second task gained after the group was built.
			r.driver.Tasks = []line.PlanTask{
				{ID: "TSK-1", Files: []string{"a.go"}}, {ID: "TSK-2", Files: []string{"b.go", "c.go"}},
			}
			r.stations.checks = [][]string{{"`go vet` exited 1"}}
			saved := State{Group: "TG-1", Current: Checking, Builder: "builder-0"}
			*r.saved = append(*r.saved, saved)
			if _, err := r.driver.Resume(context.Background(), saved); err != nil {
				t.Fatalf("resume = %v", err)
			}
			input := ""
			if resume && len(r.host.inputs) == 1 {
				input = r.host.inputs[0]
			} else if !resume && len(r.host.starts) > 0 {
				input = r.host.starts[0].Brief
			}
			fix := strings.Index(input, "- [ ] `go vet` exited 1")
			files := strings.Index(input, "- TSK-1: `a.go`\n- TSK-2: `b.go`, `c.go`")
			if fix == -1 || files < fix {
				t.Fatalf("repair input = %q, want the fix list closed by every task's files", input)
			}
		})
	}
}

// TestDriveResumesARepairStoppedBeforeItFinishedWithItsFixListAndTaskFiles resumes a Repairing
// state whose session never finished, through pendingSession rather than Driver.repair.
func TestDriveResumesARepairStoppedBeforeItFinishedWithItsFixListAndTaskFiles(t *testing.T) {
	r := newRig(t)
	r.host.results["builder-0"] = mount.Result{Value: map[string]any{"result": "DONE"}}
	r.driver.Tasks = []line.PlanTask{
		{ID: "TSK-1", Files: []string{"a.go"}}, {ID: "TSK-2", Files: []string{"c.go"}},
	}
	saved := State{
		Group: "TG-1", Current: Repairing, Sessions: []string{"builder-0"},
		Fixes: []string{"a.go:3 the loop never ends"},
	}
	*r.saved = append(*r.saved, saved)
	if _, err := r.driver.Resume(context.Background(), saved); err != nil {
		t.Fatalf("resume = %v", err)
	}
	if len(r.host.inputs) != 1 {
		t.Fatalf("resume inputs = %v, want the stopped repair resumed once", r.host.inputs)
	}
	input := r.host.inputs[0]
	fix := strings.Index(input, "- [ ] a.go:3 the loop never ends")
	files := strings.Index(input, "- TSK-1: `a.go`\n- TSK-2: `c.go`")
	if fix == -1 || files == -1 || files < fix {
		t.Fatalf("resumed repair input = %q, want the fix list closed by every task's files", input)
	}
}

func TestDriveEscalatesABlockedBuilderAndWaits(t *testing.T) {
	r := newRig(t)
	r.host.builds = []map[string]any{{"result": "BLOCKED"}}
	final, err := r.drive(t)
	if err != nil {
		t.Fatalf("drive = %v", err)
	}
	if final.Current != Escalated || final.Left != Building {
		t.Fatalf("final = %s left %s, want Escalated from Building", final.Current, final.Left)
	}
	if !equal(r.stations.calledAt, []string{"snapshot@Building"}) {
		t.Fatalf("stations ran %v after the builder blocked, want only the snapshot before it", r.stations.calledAt)
	}
	if got := r.sessions(t); !equal(got, []string{StationBuild}) {
		t.Fatalf("ledger sessions = %v, want the one build", got)
	}
}

func TestDriveKeepsItsRoundInTheStateAcrossCalls(t *testing.T) {
	r := newRig(t)
	r.stations.checks = [][]string{{"fail"}, {"fail"}}
	first, err := r.drive(t)
	if !errors.Is(err, errNoProgress) || first.Current != Escalated {
		t.Fatalf("first drive = %s, %v; want Escalated with no progress", first.Current, err)
	}
	if first.Repairs == 0 || first.Builder == "" || len(first.Fixes) != 1 || first.Fixes[0] != "fail" {
		t.Fatalf("state = %+v; the repair count, builder and fix list must be saved", first)
	}
	sessions := len(r.sessions(t))
	resumed := first
	resumed.Answered = true
	r.stations.checks = [][]string{{"fail"}}
	second, err := r.driver.Drive(context.Background(), resumed)
	if !errors.Is(err, errNoProgress) || second.Current != Blocked || second.Repairs < first.Repairs ||
		second.Stalls != 2 {
		t.Fatalf("second drive = %s with %d repairs and %d stalls, %v; the spent rounds and the first stall must carry over",
			second.Current, second.Repairs, second.Stalls, err)
	}
	if got := len(r.sessions(t)); got != sessions {
		t.Fatalf("sessions went from %d to %d; the saved fix list must stop a check failing on it again", sessions, got)
	}
}

func TestARunKilledWhileEscalatedResumesWithItsReasonAnswerStallsAndRetry(t *testing.T) {
	r := newRig(t)
	r.stations.checks = [][]string{{"fail"}, {"fail"}}
	r.host.builds = []map[string]any{{"result": "DONE"}, {"result": "BLOCKED", "question": "which clock?"}}
	escalating(r, nil,
		map[string]any{"action": ActionRetry},
		map[string]any{"action": ActionAnswer, "answer": "inject a clock parameter"},
	)
	ctx, kill := context.WithCancel(context.Background())
	defer kill()
	r.driver.Save = func(s State) error {
		*r.saved = append(*r.saved, s)
		if s.Current == Escalated && s.Answer != "" {
			kill()
		}
		return nil
	}
	if _, err := r.driver.Drive(ctx, State{Group: "TG-1", Current: Ready, SlotFree: true}); !errors.Is(err, context.Canceled) {
		t.Fatalf("drive = %v, want the run killed", err)
	}
	killed := (*r.saved)[len(*r.saved)-1]
	if killed.Current != Escalated || !killed.Answered || killed.Reason != "which clock?" ||
		killed.Answer != "inject a clock parameter" || killed.Stalls != 1 || !killed.Heavy {
		t.Fatalf("saved = %+v, want the escalation's reason, answer, one stall and the retry taken", killed)
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := SaveState(path, killed); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}

	next := newRig(t)
	next.host.resume = false
	next.stations.checks = [][]string{{"fail"}, {"fail"}}
	host := escalating(next, nil)
	final, _ := next.driver.Drive(context.Background(), loaded)
	builder := next.host.starts[0]
	if builder.Model != "opus" || !strings.Contains(builder.Brief, "inject a clock parameter") {
		t.Fatalf("resumed builder = %+v, want the heavy machine reading the saved answer", builder)
	}
	if final.Current != Blocked || final.Stalls != 2 || len(host.asked) != 0 {
		t.Fatalf("final = %s with %d stalls after %d orchestrator session(s); want the second stall blocked unasked",
			final.Current, final.Stalls, len(host.asked))
	}
}

func TestARunKilledBeforeTheOrchestratorAnsweredAsksWithTheSavedReason(t *testing.T) {
	r := newRig(t)
	r.stations.checks = [][]string{{"fail"}, {"fail"}}
	ctx, kill := context.WithCancel(context.Background())
	defer kill()
	r.driver.Save = func(s State) error {
		*r.saved = append(*r.saved, s)
		if s.Current == Escalated {
			kill()
		}
		return nil
	}
	if _, err := r.driver.Drive(ctx, State{Group: "TG-1", Current: Ready, SlotFree: true}); err == nil {
		t.Fatal("drive = nil, want the run killed")
	}
	killed := (*r.saved)[len(*r.saved)-1]
	if killed.Current != Escalated || killed.Answered || killed.Stalls != 1 ||
		!strings.Contains(killed.Reason, "the checks fail identically") {
		t.Fatalf("saved = %+v, want the unanswered escalation with its reason and one stall", killed)
	}

	next := newRig(t)
	host := escalating(next, nil, map[string]any{"action": ActionAnswer, "answer": "ship it"})
	*next.saved = append(*next.saved, killed)
	if _, err := next.driver.Drive(context.Background(), killed); err != nil {
		t.Fatalf("resumed drive = %v", err)
	}
	if len(host.asked) != 1 || !strings.Contains(host.asked[0].Brief, "the checks fail identically") {
		t.Fatalf("orchestrator briefs = %+v, want the saved reason, not the lost session's", host.asked)
	}
}

func TestDriveReturnsAFailedSaveOfAnAnsweredEscalation(t *testing.T) {
	refused := errors.New("disk full")
	cases := []struct {
		name  string
		start State
	}{
		{"an escalation answered mid-drive", State{Group: "TG-1", Current: Ready, SlotFree: true}},
		{"a saved escalation answered on resume", State{Group: "TG-1", Current: Escalated, Escalate: true, Left: Shipping}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t)
			r.host.builds = []map[string]any{{"result": "BLOCKED", "question": "which clock?"}}
			escalating(r, nil, map[string]any{"action": ActionAnswer, "answer": "inject a clock parameter"})
			*r.saved = append(*r.saved, tc.start)
			r.driver.Save = func(s State) error {
				if s.Current == Escalated && s.Answered {
					return refused
				}
				*r.saved = append(*r.saved, s)
				return nil
			}
			if _, err := r.driver.Drive(context.Background(), tc.start); !errors.Is(err, refused) {
				t.Fatalf("drive = %v, want the failed save", err)
			}
		})
	}
}

func TestDriveEscalatesAStationFailureAndReturnsIt(t *testing.T) {
	r := newRig(t)
	r.stations.shipErr = errors.New("no forge credential")
	final, err := r.drive(t)
	if err == nil || !strings.Contains(err.Error(), "no forge credential") {
		t.Fatalf("drive error = %v, want the ship failure", err)
	}
	if final.Current != Escalated || final.Left != Shipping {
		t.Fatalf("final = %s left %s, want Escalated from Shipping", final.Current, final.Left)
	}
}

func TestDriveMergesAShippedGroupIntoItsEpicAndRemovesIt(t *testing.T) {
	r := newRig(t)
	r.stations.merged = true
	final, err := r.drive(t)
	if err != nil || final.Current != Shipped || !final.Merged {
		t.Fatalf("drive = %+v, %v; a merged group ends Shipped with merged set", final, err)
	}
	if action := Next(final); !action.Remove {
		t.Fatalf("next = %+v; a merged group is removed from state.json", action)
	}
}

func TestDriveLeavesAGroupOffItsEpicForAPersonToMerge(t *testing.T) {
	r := newRig(t)
	final, err := r.drive(t)
	if err != nil || final.Current != Shipped || final.Merged {
		t.Fatalf("drive = %+v, %v; a PR the conductor may not merge waits at Shipped", final, err)
	}
}

func TestDriveEscalatesAFailedMerge(t *testing.T) {
	r := newRig(t)
	r.stations.mergeErr = errors.New("a failed check")
	final, err := r.drive(t)
	if err == nil || !strings.Contains(err.Error(), "a failed check") {
		t.Fatalf("drive error = %v, want the merge failure", err)
	}
	if final.Current != Escalated || final.Left != Shipped {
		t.Fatalf("final = %s left %s, want Escalated from Shipped", final.Current, final.Left)
	}
}

func TestDriveResumesAnAnsweredEscalationInTheStateItLeft(t *testing.T) {
	r := newRig(t)
	start := State{Group: "TG-1", Current: Escalated, Answered: true, Left: Shipping, Escalate: true}
	final, err := r.driver.Drive(context.Background(), start)
	if err != nil || final.Current != Shipped {
		t.Fatalf("drive = %s, %v; want Shipped", final.Current, err)
	}
	if final.Escalate || final.Answered {
		t.Fatalf("final = %+v, want the escalation cleared", final)
	}
	if got := r.sessions(t); len(got) != 0 {
		t.Fatalf("ledger sessions = %v, want none for a ship", got)
	}
}

func TestDriveWaitsForASlot(t *testing.T) {
	r := newRig(t)
	final, err := r.driver.Drive(context.Background(), State{Group: "TG-1", Current: Ready})
	if err != nil || final.Current != Ready || len(*r.saved) != 0 {
		t.Fatalf("drive = %s, %v with %d saves; want Ready untouched", final.Current, err, len(*r.saved))
	}
}

func TestDriveStopsTheSessionWhenCancelled(t *testing.T) {
	r := newRig(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.host.hang = true
	r.host.cancel = cancel
	final, err := r.driver.Drive(ctx, State{Group: "TG-1", Current: Ready, SlotFree: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("drive error = %v, want context.Canceled", err)
	}
	if final.Current != Building || len(r.host.stopped) != 1 {
		t.Fatalf("final = %s, stopped %v; want the building session stopped", final.Current, r.host.stopped)
	}
}

func TestDriveRefusesAnUnwiredDriver(t *testing.T) {
	var driver Driver
	if _, err := driver.Drive(context.Background(), State{Group: "TG-1"}); !errors.Is(err, errNotWired) {
		t.Fatalf("drive error = %v, want errNotWired", err)
	}
}

// gitIn runs one git command in dir, failing the test when it fails.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// writeIn writes content to name under dir, creating its parent directories.
func writeIn(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// checkRepo creates a Go module committed on main, its state directory ignored, as a group's root and worktree.
func checkRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "builder@example.com")
	gitIn(t, root, "config", "user.name", "builder")
	writeIn(t, root, "go.mod", "module example.com/tmp\n\ngo 1.22\n")
	writeIn(t, root, ".gitignore", "/.komodo/\n")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "base")
	return root
}

func TestLineCheckTurnsARefusedCommitIntoAFix(t *testing.T) {
	root := checkRepo(t)
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho 'comment runs 21 words' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	stations := &Line{Root: root, Plan: &line.Plan{
		Group: "TG-1", Title: "A group", Type: "feat", Base: "main", Worktree: root,
		Tasks: []line.PlanTask{{ID: "TSK-1.1", Files: []string{"built.go"}}},
	}}
	if err := stations.Snapshot(); err != nil {
		t.Fatal(err)
	}
	writeIn(t, root, "built.go", "package built\n")
	fixes, err := stations.Check(context.Background())
	if err != nil {
		t.Fatalf("check = %v; a refused commit is a fix, never an error that escalates", err)
	}
	if len(fixes) != 1 || !strings.Contains(fixes[0], "comment runs 21 words") {
		t.Fatalf("fixes = %q, want the hook's refusal", fixes)
	}
}

func TestLineCheckFailsWhenAnyCheckFails(t *testing.T) {
	const uncovered = "package pkg\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"
	cases := []struct {
		name    string
		compile string
		verify  string
		bar     string
		files   []string
		session func(t *testing.T, root string)
		want    string
	}{
		{"every check passes", "true", "true", "", []string{"a.txt"}, func(t *testing.T, root string) {
			writeIn(t, root, "a.txt", "a\n")
		}, ""},
		{"a compile gate fails", "exit 3", "true", "", []string{"a.txt"}, func(t *testing.T, root string) {
			writeIn(t, root, "a.txt", "a\n")
		}, "check: `exit 3`"},
		{"verify fails", "true", "exit 4", "", []string{"a.txt"}, func(t *testing.T, root string) {
			writeIn(t, root, "a.txt", "a\n")
		}, "check: `exit 4`"},
		{"ship's backlog edits stay in scope", "true", "true", "", []string{"a.txt"},
			func(t *testing.T, root string) {
				writeIn(t, root, "docs/backlog/TG-1.md", "## [TG-1] A group [P: C] [READY]\n\n```yaml\ntype: feat\n```\n")
			}, ""},
		{"an edit lands outside scope", "true", "true", "", []string{"a.txt"}, func(t *testing.T, root string) {
			writeIn(t, root, "b.txt", "b\n")
		}, "scope: b.txt"},
		{"the session commits", "true", "true", "", []string{"a.txt"}, func(t *testing.T, root string) {
			writeIn(t, root, "a.txt", "a\n")
			gitIn(t, root, "add", "-A")
			gitIn(t, root, "commit", "-q", "-m", "a model commit")
		}, "moved HEAD"},
		{"an added line holds a secret", "true", "true", "", []string{"a.txt"}, func(t *testing.T, root string) {
			writeIn(t, root, "a.txt", "key = AKIA"+"ABCDEFGHIJKLMNOP\n")
		}, "secret: a.txt:1"},
		{"changed-line coverage falls below its bar", "true", "true", `{"value":1}`, []string{"pkg/thing.go"},
			func(t *testing.T, root string) {
				writeIn(t, root, "pkg/thing.go", uncovered)
			}, "coverage:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := checkRepo(t)
			writeIn(t, root, ".komodo/commands.json", fmt.Sprintf(`{"compile": %q, "verify": %q}`, tc.compile, tc.verify))
			if tc.bar != "" {
				writeIn(t, root, ".komodo/"+coverageBarFile, tc.bar)
			}
			stations := &Line{Root: root, Plan: &line.Plan{
				Group: "TG-1", Version: "1.0.0", Base: "main", Tasks: []line.PlanTask{{ID: "TSK-1", Files: tc.files}},
			}}
			if err := stations.Snapshot(); err != nil {
				t.Fatalf("snapshot = %v", err)
			}
			tc.session(t, root)
			fixes, err := stations.Check(context.Background())
			if err != nil {
				t.Fatalf("check = %v", err)
			}
			if tc.want == "" && len(fixes) != 0 {
				t.Fatalf("fixes = %q, want none", fixes)
			}
			if tc.want != "" && !strings.Contains(strings.Join(fixes, "\n"), tc.want) {
				t.Fatalf("fixes = %q, want one naming %q", fixes, tc.want)
			}
			if tc.want != "" {
				return
			}
			// What Check ran is what Ship's PR body reports, never "no QC gate or verify command ran".
			plan := &line.Plan{Group: "TG-1", Title: "A group"}
			body := line.ReportBody(plan, &line.ShipResult{}, []*line.WaveResult{stations.checked}, line.BodyContext{})
			if !strings.Contains(body, "- `true` passed") || strings.Contains(body, "Unproven") {
				t.Fatalf("body = %q; a passed Check's gates and verify must be reported", body)
			}
		})
	}
}

// shipless runs a real Line's Snapshot and Check, with Prepare rerunning Check and Ship doing nothing.
type shipless struct{ *Line }

func (s shipless) Prepare(ctx context.Context) ([]string, error) { return s.Check(ctx) }

func (s shipless) Ship(context.Context) error { return nil }

// lineRig wires a rig's driver to a real Line over a fresh repo whose group declares sneaky.txt.
func lineRig(t *testing.T) (*rig, *Line) {
	t.Helper()
	r := newRig(t)
	root := checkRepo(t)
	writeIn(t, root, ".komodo/commands.json", `{"compile": "true", "verify": "true"}`)
	stations := &Line{Root: root, Plan: &line.Plan{
		Group: "TG-1", Version: "1.0.0", Base: "main", Tasks: []line.PlanTask{{ID: "TSK-1", Files: []string{"sneaky.txt"}}},
	}}
	r.driver.Stations = shipless{stations}
	return r, stations
}

func TestResumeChecksAKilledSessionAgainstTheSnapshotTakenBeforeIt(t *testing.T) {
	r, killed := lineRig(t)
	if err := killed.Snapshot(); err != nil {
		t.Fatalf("snapshot = %v", err)
	}
	root := killed.Root
	writeIn(t, root, "sneaky.txt", "x\n")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "a model commit")
	r.driver.Stations = shipless{&Line{Root: root, Plan: killed.Plan}}
	last := mount.Handle("builder-1")
	r.host.results[last] = mount.Result{Value: map[string]any{"result": "DONE"}}
	start := State{Group: "TG-1", Current: Building, Sessions: []string{string(last)}}
	*r.saved = append(*r.saved, start)

	// The fake repair edits no file, so the loop stops after it.
	if _, err := r.driver.Resume(context.Background(), start); !errors.Is(err, errNoProgress) {
		t.Fatalf("resume = %v, want the idle repair to stop the loop", err)
	}
	var repair string
	for _, req := range r.host.starts {
		if req.Role == "builder" {
			repair = req.Brief
		}
	}
	if !strings.Contains(repair, "moved HEAD") {
		t.Fatalf("repair brief = %q, want a fix naming the model commit", repair)
	}
}

func TestResumeFailsCheckWithNoSnapshotForTheSessionItFollows(t *testing.T) {
	r, _ := lineRig(t)
	start := State{Group: "TG-1", Current: Checking}
	*r.saved = append(*r.saved, start)

	final, err := r.driver.Resume(context.Background(), start)
	if !errors.Is(err, errNoSnapshot) {
		t.Fatalf("resume error = %v, want errNoSnapshot", err)
	}
	if final.Current == Reviewing || final.Current == Shipped {
		t.Fatalf("final state = %s, want the group stopped before review", final.Current)
	}
}

func TestAStopDuringCheckKillsItsCommandsAndNeverCommits(t *testing.T) {
	const (
		stopAfter = 200 * time.Millisecond
		bound     = 10 * time.Second
	)
	r, stations := lineRig(t)
	writeIn(t, stations.Root, ".komodo/commands.json", `{"compile": "true", "verify": "sleep 60"}`)
	if err := stations.Snapshot(); err != nil {
		t.Fatalf("snapshot = %v", err)
	}
	writeIn(t, stations.Root, "sneaky.txt", "x\n")
	before, err := stations.Head()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	save := r.driver.Save
	r.driver.Save = func(s State) error {
		if s.Current == Checking {
			time.AfterFunc(stopAfter, cancel)
		}
		return save(s)
	}
	start := State{Group: "TG-1", Current: Building, SessionDone: true}
	*r.saved = append(*r.saved, start)

	began := time.Now()
	final, err := r.driver.Drive(ctx, start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("drive = %v, want the stop's cancellation", err)
	}
	if took := time.Since(began); took > bound {
		t.Fatalf("drive took %s after the stop, want under %s", took, bound)
	}
	if final.Current != Checking {
		t.Fatalf("final state = %s, want Checking", final.Current)
	}
	after, err := stations.Head()
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("HEAD moved from %s to %s; a stopped Check must never commit", before, after)
	}
}

func TestALineStationStoppedBeforeItRunsNeverCommitsOrShips(t *testing.T) {
	cases := []struct {
		name string
		run  func(context.Context, *Line) error
		// cancelled is whether the station returns the stop itself rather than the checks it killed.
		cancelled bool
	}{
		{"check", func(ctx context.Context, l *Line) error {
			_, err := l.Check(ctx)
			return err
		}, true},
		{"ship after a passed check", func(ctx context.Context, l *Line) error {
			l.checked = passed(nil)
			return l.Ship(ctx)
		}, true},
		{"ship rerunning its checks", func(ctx context.Context, l *Line) error {
			return l.Ship(ctx)
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stations := lineRig(t)
			if err := stations.Snapshot(); err != nil {
				t.Fatalf("snapshot = %v", err)
			}
			writeIn(t, stations.Root, "sneaky.txt", "x\n")
			before, err := stations.Head()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err = tc.run(ctx, stations)
			if err == nil || tc.cancelled != errors.Is(err, context.Canceled) {
				t.Fatalf("%s = %v, want it stopped (by the cancellation itself: %v)", tc.name, err, tc.cancelled)
			}
			if after, err := stations.Head(); err != nil || after != before {
				t.Fatalf("HEAD = %s (%v), want %s; a stopped station must never commit", after, err, before)
			}
		})
	}
}

func TestAStoppedPrepareMarksNoTaskDone(t *testing.T) {
	_, stations := lineRig(t)
	writeIn(t, stations.Root, "sneaky.txt", "x\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fixes, err := stations.Prepare(ctx)
	if err != nil || len(fixes) == 0 {
		t.Fatalf("prepare = %q, %v; want its killed checks as fixes", fixes, err)
	}
	if status := line.LoadStatus(stations.Root); status["TSK-1"].Status == "DONE" {
		t.Fatal("a stopped Prepare marked its task DONE")
	}
}

func TestCoverageSkipsPackagesGoTestNeverBuilds(t *testing.T) {
	diff := "diff --git a/internal/eval/suite.go b/internal/eval/suite.go\n--- a/internal/eval/suite.go\n+++ b/internal/eval/suite.go\n@@ -0,0 +1 @@\n+package eval\n" +
		"diff --git a/internal/eval/testdata/g/g_test.go b/internal/eval/testdata/g/g_test.go\n--- a/internal/eval/testdata/g/g_test.go\n+++ b/internal/eval/testdata/g/g_test.go\n@@ -0,0 +1 @@\n+package g\n" +
		"diff --git a/_scratch/x.go b/_scratch/x.go\n--- a/_scratch/x.go\n+++ b/_scratch/x.go\n@@ -0,0 +1 @@\n+package x\n" +
		"diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -0,0 +1 @@\n+package main\n"
	if got := coveredPackages(diff); !slices.Equal(got, []string{"./internal/eval", "./."}) {
		t.Fatalf("packages = %v; want internal/eval and the root, never testdata or _scratch", got)
	}
}

func TestLinePrepareCommitsThenPassesNoGroupToShipUntilItsHooksPass(t *testing.T) {
	root := checkRepo(t)
	writeIn(t, root, ".komodo/commands.json", `{"compile": "true", "verify": "true"}`)
	hook := filepath.Join(root, ".git", "hooks", "pre-push")
	writeIn(t, root, filepath.Join(".git", "hooks", "pre-push"), "#!/bin/sh\necho 'the pre-push gate refuses' >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	stations := &Line{Root: root, Plan: &line.Plan{
		Group: "TG-1", Title: "A group", Type: "feat", Base: "main", Branch: "main",
		Tasks: []line.PlanTask{{ID: "TSK-1", Files: []string{"sneaky.txt"}}},
	}}
	writeIn(t, root, "sneaky.txt", "x\n")
	fixes, err := stations.Prepare(context.Background())
	if err != nil || len(fixes) != 1 || !strings.Contains(fixes[0], "the pre-push gate refuses") {
		t.Fatalf("prepare = %q, %v; want the hook's refusal as a fix for a repair round", fixes, err)
	}
	subject, err := exec.Command("git", "-C", root, "log", "-1", "--format=%s").Output()
	if err != nil || strings.TrimSpace(string(subject)) != "feat: A group (TG-1)" {
		t.Fatalf("HEAD = %q (%v), want the group's commit made before the hooks ran", subject, err)
	}
	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	if fixes, err := stations.Prepare(context.Background()); err != nil || len(fixes) > 0 {
		t.Fatalf("prepare = %q, %v; want it passed once the hook does", fixes, err)
	}
	entries, err := line.Book(root).All()
	if err != nil {
		t.Fatal(err)
	}
	var stamped []string
	for _, entry := range entries {
		stamped = append(stamped, entry.Station+":"+entry.Outcome)
	}
	if !slices.Equal(stamped, []string{"prepare:fixes", "prepare:done"}) {
		t.Fatalf("ledger = %v; want no ship before the prepare that passed", stamped)
	}
}
