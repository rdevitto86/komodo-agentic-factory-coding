package conductor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/ledger"
	"komodo/internal/line"
	"komodo/internal/mount"
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
	startsAt []GroupState
	next     int
	hang     bool
	cancel   context.CancelFunc
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

func (f *fakeHost) Preflight() error { return nil }

func (f *fakeHost) Start(req mount.StartRequest) (mount.Handle, error) {
	f.starts = append(f.starts, req)
	if f.cancel != nil {
		f.cancel()
	}
	if req.Role == "reviewer" {
		return f.handle(req.Role, pop(&f.reviews, map[string]any{"findings": []any{}})), nil
	}
	return f.handle(req.Role, pop(&f.builds, map[string]any{"result": "DONE"})), nil
}

func (f *fakeHost) Resume(handle mount.Handle, input string) (mount.Handle, error) {
	if _, ok := f.results[handle]; !ok {
		return "", errors.New("no such session")
	}
	f.inputs = append(f.inputs, input)
	return f.handle("builder", pop(&f.repairs, map[string]any{"result": "DONE"})), nil
}

func (f *fakeHost) Stream(handle mount.Handle) (<-chan mount.Event, error) {
	if f.hang {
		return make(chan mount.Event), nil
	}
	out := make(chan mount.Event, 1)
	out <- mount.Event{Turns: 2, Usage: mount.TaskUsage{TokensIn: 100, TokensOut: 20, Turns: 2}}
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

func (f *fakeHost) Stop(handle mount.Handle) error {
	f.stopped = append(f.stopped, handle)
	return nil
}

func (f *fakeHost) Capabilities() mount.Capabilities {
	return mount.Capabilities{Resume: f.resume, Sandbox: true, Hooks: true, Structured: true}
}

// fakeStations scripts Check and Prepare fix lists and a Ship error, and records the state each began in.
type fakeStations struct {
	saved    *[]State
	checks   [][]string
	prepares [][]string
	shipErr  error
	calledAt []string
}

// record notes which station ran and the state saved before it.
func (f *fakeStations) record(name string) {
	f.calledAt = append(f.calledAt, name+"@"+string((*f.saved)[len(*f.saved)-1].Current))
}

func (f *fakeStations) Snapshot() error {
	f.record("snapshot")
	return nil
}

func (f *fakeStations) Check() ([]string, error) {
	f.record("check")
	if len(f.checks) == 0 {
		return nil, nil
	}
	fixes := f.checks[0]
	f.checks = f.checks[1:]
	return fixes, nil
}

func (f *fakeStations) Prepare() ([]string, error) {
	f.record("prepare")
	if len(f.prepares) == 0 {
		return nil, nil
	}
	fixes := f.prepares[0]
	f.prepares = f.prepares[1:]
	return fixes, nil
}

func (f *fakeStations) Ship() error {
	f.record("ship")
	return f.shipErr
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
		Repairs:       2,
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
		case StationBuild, StationReview, StationRepair:
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
	stations := []string{"snapshot@Building", "check@Checking", "prepare@Preparing", "ship@Shipping"}
	if !equal(r.stations.calledAt, stations) {
		t.Fatalf("stations ran at %v, want %v", r.stations.calledAt, stations)
	}
	if len(final.Sessions) != 2 {
		t.Fatalf("sessions = %v, want the builder's and the reviewer's", final.Sessions)
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
		"prepare@Preparing", "ship@Shipping",
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
	if got := r.sessions(t); !equal(got, []string{StationBuild, StationReview, StationRepair, StationReview}) {
		t.Fatalf("ledger sessions = %v", got)
	}
	input := r.host.inputs[0]
	if !strings.Contains(input, "a.go:3 nil map: make it") || strings.Contains(input, "b.go") {
		t.Fatalf("fix list = %q, want only the finding at or above the floor", input)
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

func TestDriveEscalatesOnceItsRepairRoundsAreSpent(t *testing.T) {
	r := newRig(t)
	r.stations.checks = [][]string{{"fail"}, {"fail"}, {"fail"}}
	final, err := r.drive(t)
	if err != nil {
		t.Fatalf("drive = %v", err)
	}
	if final.Current != Escalated || final.Left != Repairing {
		t.Fatalf("final = %s left %s, want Escalated from Repairing", final.Current, final.Left)
	}
	if got := r.sessions(t); !equal(got, []string{StationBuild, StationRepair, StationRepair}) {
		t.Fatalf("ledger sessions = %v, want one build and two repairs", got)
	}
}

func TestDriveKeepsItsRoundInTheStateAcrossCalls(t *testing.T) {
	r := newRig(t)
	r.stations.checks = [][]string{{"fail"}, {"fail"}, {"fail"}}
	first, err := r.drive(t)
	if err != nil || first.Current != Escalated {
		t.Fatalf("first drive = %s, %v; want Escalated", first.Current, err)
	}
	if first.Repairs == 0 || first.Builder == "" || len(first.Fixes) != 1 || first.Fixes[0] != "fail" {
		t.Fatalf("state = %+v; the repair count, builder and fix list must be saved", first)
	}
	sessions := len(r.sessions(t))
	resumed := first
	resumed.Answered = true
	second, err := r.driver.Drive(context.Background(), resumed)
	if err != nil || second.Current != Escalated || second.Repairs < first.Repairs {
		t.Fatalf("second drive = %s with %d repairs, %v; the spent rounds must carry over", second.Current, second.Repairs, err)
	}
	if got := len(r.sessions(t)); got != sessions {
		t.Fatalf("sessions went from %d to %d; a spent repair budget must not start another", sessions, got)
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
	fixes, err := stations.Check()
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
		{"ship's backlog and changelog edits stay in scope", "true", "true", "", []string{"a.txt"},
			func(t *testing.T, root string) {
				writeIn(t, root, "BACKLOG.md", "# Backlog\n")
				writeIn(t, root, "changelog.d/1.0.0/TG-1.md", "- a line\n")
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
			fixes, err := stations.Check()
			if err != nil {
				t.Fatalf("check = %v", err)
			}
			if tc.want == "" && len(fixes) != 0 {
				t.Fatalf("fixes = %q, want none", fixes)
			}
			if tc.want != "" && !strings.Contains(strings.Join(fixes, "\n"), tc.want) {
				t.Fatalf("fixes = %q, want one naming %q", fixes, tc.want)
			}
		})
	}
}

// shipless runs a real Line's Snapshot and Check, with Prepare rerunning Check and Ship doing nothing.
type shipless struct{ *Line }

func (s shipless) Prepare() ([]string, error) { return s.Check() }

func (s shipless) Ship() error { return nil }

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

	if _, err := r.driver.Resume(context.Background(), start); err != nil {
		t.Fatalf("resume = %v", err)
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
