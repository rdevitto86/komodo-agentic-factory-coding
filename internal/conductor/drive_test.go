package conductor

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	stations := []string{"check@Checking", "prepare@Preparing", "ship@Shipping"}
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
	if len(r.stations.calledAt) != 0 {
		t.Fatalf("stations ran %v after the builder blocked", r.stations.calledAt)
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

func TestLineCheckReturnsTheFirstFailedCommand(t *testing.T) {
	cases := []struct {
		name    string
		compile string
		verify  string
		want    string
	}{
		{"every command passes", "true", "true", ""},
		{"a compile gate fails", "exit 3", "true", "`exit 3` exited 3"},
		{"verify fails", "true", "exit 4", "`exit 4` exited 4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			commands := fmt.Sprintf(`{"compile": %q, "verify": %q}`, tc.compile, tc.verify)
			if err := os.MkdirAll(filepath.Join(root, ".komodo"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".komodo", "commands.json"), []byte(commands), 0o644); err != nil {
				t.Fatal(err)
			}
			stations := &Line{Root: root, Plan: &line.Plan{Group: "TG-1"}}
			fixes, err := stations.Check()
			if err != nil {
				t.Fatalf("check = %v", err)
			}
			if tc.want == "" && len(fixes) != 0 {
				t.Fatalf("fixes = %q, want none", fixes)
			}
			if tc.want != "" && (len(fixes) != 1 || !strings.HasPrefix(fixes[0], tc.want)) {
				t.Fatalf("fixes = %q, want one starting %q", fixes, tc.want)
			}
		})
	}
}
