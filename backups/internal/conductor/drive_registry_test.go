package conductor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"komodo/internal/ledger"
	"komodo/internal/mount"
)

// TestDrivePushesBackTwiceThenApprovesOnOneSessionIDWithEachWindowReset proves a group a reviewer
// pushes back twice, then approves, keeps the one builder its registry entry ever tracked.
func TestDrivePushesBackTwiceThenApprovesOnOneSessionIDWithEachWindowReset(t *testing.T) {
	r := newRig(t)
	clock := NewFakeClock(time.Now())
	r.driver.Registry = fastRegistry(clock)
	finding := func(file string) map[string]any {
		return map[string]any{"severity": "high", "file": file, "line": 3, "title": "wrong", "fix": "fix it"}
	}
	// Round two's re-review keeps one of round one's two findings open, so each round progresses.
	r.host.reviews = []map[string]any{
		{"findings": []any{finding("a.go"), finding("b.go")}},
		{"findings": []any{finding("a.go")}},
	}
	before := clock.Now()
	clock.Advance(20 * time.Minute)
	final, err := r.drive(t)
	if err != nil || final.Current != Shipped {
		t.Fatalf("drive = %s, %v; want Shipped", final.Current, err)
	}
	snapshot := r.driver.Registry.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("registry entries = %+v, want the one builder and the one lens's reviewer tracked throughout", snapshot)
	}
	entry, ok := snapshot[final.Group]
	if !ok {
		t.Fatalf("registry = %+v, want an entry keyed by %s", snapshot, final.Group)
	}
	reviewerEntry, ok := snapshot[final.Group+":economy"]
	if !ok {
		t.Fatalf("registry = %+v, want a reviewer entry keyed by %s:economy", snapshot, final.Group)
	}
	if reviewerEntry.Role != "reviewer" || reviewerEntry.State != RegDone {
		t.Fatalf("reviewer entry = %+v, want role reviewer, state RegDone once shipped", reviewerEntry)
	}
	if entry.Retries != 2 {
		t.Fatalf("retries = %d, want 2 pushbacks before the approval", entry.Retries)
	}
	if !entry.WindowStart.After(before) {
		t.Fatalf("window start = %s, want it reset past %s by the last pushback", entry.WindowStart, before)
	}
	if entry.State != RegDone {
		t.Fatalf("state = %s, want RegDone once shipped", entry.State)
	}
	if final.Registry[final.Group].Retries != 2 {
		t.Fatalf("saved state's registry = %+v, want the same two retries persisted", final.Registry)
	}
}

// idleHost hangs its builder session until Stop closes its stream, then its Result fails.
type idleHost struct {
	mu      sync.Mutex
	events  chan mount.Event
	started bool
	stopped bool
}

func (h *idleHost) Preflight(context.Context) error { return nil }

func (h *idleHost) Start(context.Context, mount.StartRequest) (mount.Handle, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events, h.started = make(chan mount.Event), true
	return "builder-1", nil
}

func (h *idleHost) Resume(context.Context, mount.Handle, string) (mount.Handle, error) {
	return "", errors.New("idleHost never resumes")
}

func (h *idleHost) Stream(context.Context, mount.Handle) (<-chan mount.Event, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.events, nil
}

func (h *idleHost) Result(mount.Handle) (mount.Result, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		return mount.Result{}, errors.New("the session was stopped before it answered")
	}
	return mount.Result{Value: map[string]any{"result": "DONE"}}, nil
}

func (h *idleHost) Stop(context.Context, mount.Handle) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.stopped {
		h.stopped = true
		close(h.events)
	}
	return nil
}

func (h *idleHost) Capabilities() mount.Capabilities { return mount.Capabilities{} }

// TestDriveNudgesThenKillsAnIdleBuilderAndBlocks proves a builder that never streams a byte gets
// nudged once its idle grace starts, then killed once the grace runs out, blocking the group.
func TestDriveNudgesThenKillsAnIdleBuilderAndBlocks(t *testing.T) {
	saved := &[]State{}
	host := &idleHost{}
	stations := &fakeStations{saved: saved}
	clock := NewFakeClock(time.Now())
	registry := fastRegistry(clock)
	driver := &Driver{
		Host: host, Stations: stations, Ledger: ledger.New(t.TempDir()), Run: "run-1",
		Builder:  mount.StartRequest{Role: "builder", Brief: "build TG-1"},
		Registry: registry,
		Save:     func(s State) error { *saved = append(*saved, s); return nil },
	}

	type outcome struct {
		state State
		err   error
	}
	done := make(chan outcome, 1)
	go func() {
		final, err := driver.Drive(context.Background(), State{Group: "TG-1", Current: Ready, SlotFree: true})
		done <- outcome{final, err}
	}()

	waitForEntry(t, registry, "TG-1")
	clock.Advance(11 * time.Minute) // past the builder's 10-minute idle: one nudge
	waitForState(t, registry, "TG-1", Stalled)
	clock.Advance(5 * time.Minute) // past its 5-minute grace: the kill

	select {
	case result := <-done:
		if result.state.Current != Blocked {
			t.Fatalf("final = %s, %v; want Blocked", result.state.Current, result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("drive never returned after the idle builder was killed")
	}
	if entry := registry.Snapshot()["TG-1"]; entry.State != SpunDown {
		t.Fatalf("registry state = %s, want SpunDown", entry.State)
	}
}

// waitForEntry blocks until id is registered, or fails the test.
func waitForEntry(t *testing.T, reg *Registry, id string) {
	t.Helper()
	for i := 0; i < 500; i++ {
		if _, ok := reg.Snapshot()[id]; ok {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%s was never registered", id)
}

// waitForState blocks until id reaches want, or fails the test.
func waitForState(t *testing.T, reg *Registry, id string, want BuilderState) {
	t.Helper()
	for i := 0; i < 500; i++ {
		if reg.Snapshot()[id].State == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%s never reached %s; last seen %+v", id, want, reg.Snapshot()[id])
}

// TestDriveThreeReviewerStrikesBlockTheBuilderPermanentlyAcrossAReload proves three rounds of
// reviewer pushback on one builder block it for good, and that a reload keeps it blocked.
func TestDriveThreeReviewerStrikesBlockTheBuilderPermanentlyAcrossAReload(t *testing.T) {
	r := newRig(t)
	r.driver.Registry = fastRegistry(NewFakeClock(time.Now()))
	finding := func(file string) map[string]any {
		return map[string]any{"severity": "high", "file": file, "line": 3, "title": "wrong", "fix": "fix it"}
	}
	// Each re-review drops one finding and keeps the rest, so three rounds of pushback land.
	r.host.reviews = []map[string]any{
		{"findings": []any{finding("a.go"), finding("b.go"), finding("c.go")}},
		{"findings": []any{finding("a.go"), finding("b.go")}},
		{"findings": []any{finding("a.go")}},
	}
	final, err := r.drive(t)
	if err == nil || final.Current != Blocked {
		t.Fatalf("drive = %s, %v; want Blocked after the third reviewer strike", final.Current, err)
	}
	entry := final.Registry[final.Group]
	if entry.Strikes != 3 || entry.State != RegBlocked {
		t.Fatalf("registry entry = %+v, want 3 strikes and RegBlocked, persisted in state.json", entry)
	}

	reloaded := fastRegistry(NewFakeClock(time.Now()))
	reloaded.Load(final.Registry)
	if got := reloaded.Snapshot()[final.Group]; got.State != RegBlocked || got.Strikes != 3 {
		t.Fatalf("reloaded entry = %+v, want it still blocked with its three strikes", got)
	}
}

// reviewerHangHost completes a builder session at once but hangs a reviewer's own stream until Stop
// closes it, every method safe for the registry's background goroutines to call concurrently.
type reviewerHangHost struct {
	mu       sync.Mutex
	handles  int
	reviewer mount.Handle
	events   chan mount.Event
	stopped  bool
}

func (h *reviewerHangHost) Preflight(context.Context) error { return nil }

func (h *reviewerHangHost) Start(ctx context.Context, req mount.StartRequest) (mount.Handle, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handles++
	if req.Role == "reviewer" {
		h.reviewer = mount.Handle(fmt.Sprintf("reviewer-%d", h.handles))
		h.events = make(chan mount.Event)
		return h.reviewer, nil
	}
	return mount.Handle(fmt.Sprintf("builder-%d", h.handles)), nil
}

func (h *reviewerHangHost) Resume(context.Context, mount.Handle, string) (mount.Handle, error) {
	return "", errors.New("reviewerHangHost never resumes")
}

func (h *reviewerHangHost) Stream(ctx context.Context, handle mount.Handle) (<-chan mount.Event, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if handle == h.reviewer {
		return h.events, nil
	}
	out := make(chan mount.Event, 1)
	out <- mount.Event{Turns: 1}
	close(out)
	return out, nil
}

func (h *reviewerHangHost) Result(handle mount.Handle) (mount.Result, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if handle == h.reviewer {
		if h.stopped {
			return mount.Result{}, errors.New("the reviewer was stopped before it answered")
		}
		return mount.Result{Value: map[string]any{"findings": []any{}}}, nil
	}
	return mount.Result{Value: map[string]any{"result": "DONE"}}, nil
}

func (h *reviewerHangHost) Stop(ctx context.Context, handle mount.Handle) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if handle == h.reviewer && !h.stopped {
		h.stopped = true
		close(h.events)
	}
	return nil
}

func (h *reviewerHangHost) Capabilities() mount.Capabilities { return mount.Capabilities{} }

// TestDriveKillsAHungReviewerAndFailsTheRoundWithoutStrikingTheBuilder proves a reviewer's own
// window is enforced, its kill fails the review round, and never counts as a builder strike.
func TestDriveKillsAHungReviewerAndFailsTheRoundWithoutStrikingTheBuilder(t *testing.T) {
	saved := &[]State{}
	host := &reviewerHangHost{}
	stations := &fakeStations{saved: saved}
	clock := NewFakeClock(time.Now())
	registry := fastRegistry(clock)
	driver := &Driver{
		Host: host, Stations: stations, Ledger: ledger.New(t.TempDir()), Run: "run-1",
		Builder:  mount.StartRequest{Role: "builder", Brief: "build TG-1"},
		Reviewer: mount.StartRequest{Role: "reviewer", Brief: "review TG-1"},
		Registry: registry,
		Save:     func(s State) error { *saved = append(*saved, s); return nil },
	}

	done := make(chan struct{})
	var final State
	var err error
	go func() {
		final, err = driver.Drive(context.Background(), State{Group: "TG-1", Current: Ready, SlotFree: true})
		close(done)
	}()

	waitForEntry(t, registry, "TG-1:economy")
	clock.Advance(11 * time.Minute) // past the reviewer's own 10-minute window: the kill

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("drive never returned after the hung reviewer was killed")
	}
	if err == nil || final.Current != Blocked {
		t.Fatalf("drive = %s, %v; want Blocked once the hung reviewer's round fails", final.Current, err)
	}
	reviewerEntry := registry.Snapshot()["TG-1:economy"]
	if reviewerEntry.Role != "reviewer" {
		t.Fatalf("reviewer entry = %+v, want role reviewer", reviewerEntry)
	}
	host.mu.Lock()
	killed := host.stopped
	host.mu.Unlock()
	if !killed {
		t.Fatal("want the hung reviewer's session killed past its window")
	}
	builderEntry := registry.Snapshot()["TG-1"]
	if builderEntry.Strikes != 0 || builderEntry.Retries != 0 {
		t.Fatalf("builder entry = %+v, want no strike or pushback from the reviewer's own kill", builderEntry)
	}
}
