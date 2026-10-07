package conductor

import (
	"sync/atomic"
	"testing"
	"time"
)

// fastRegistry builds a Registry on clock whose own tick is a millisecond, never the package default.
func fastRegistry(clock Clock) *Registry {
	reg := NewRegistry(clock)
	reg.tick = time.Millisecond
	return reg
}

// settle gives a registry's goroutine a moment to react to an Advance before the test reads state.
func settle() { time.Sleep(10 * time.Millisecond) }

// TestRegistryWindowExpiryKillsTheEntry proves a builder's 30-minute window firing Kill once the
// clock passes it with no Pushback to reset it.
func TestRegistryWindowExpiryKillsTheEntry(t *testing.T) {
	clock := NewFakeClock(time.Now())
	var killed atomic.Int32
	reg := fastRegistry(clock)
	reg.Kill = func(string) { killed.Add(1) }
	reg.Register("b1", "tg-1", "sess-1", "builder")

	clock.Advance(31 * time.Minute)
	settle()

	if killed.Load() != 1 {
		t.Fatalf("killed = %d, want 1", killed.Load())
	}
	entry := reg.Snapshot()["b1"]
	if entry.State != Stalled {
		t.Fatalf("state = %s, want %s", entry.State, Stalled)
	}
}

// TestRegistryIdleGoesStalled proves ten minutes with no Touch moves an entry to Stalled and
// nudges it once.
func TestRegistryIdleGoesStalled(t *testing.T) {
	clock := NewFakeClock(time.Now())
	var nudges atomic.Int32
	reg := fastRegistry(clock)
	reg.Nudge = func(string) { nudges.Add(1) }
	reg.Register("b1", "tg-1", "sess-1", "builder")

	clock.Advance(11 * time.Minute)
	settle()

	if nudges.Load() != 1 {
		t.Fatalf("nudges = %d, want 1", nudges.Load())
	}
	if entry := reg.Snapshot()["b1"]; entry.State != Stalled {
		t.Fatalf("state = %s, want %s", entry.State, Stalled)
	}
}

// TestRegistryNudgeThenGraceKills proves a nudged entry that stays silent through its grace is
// killed exactly once.
func TestRegistryNudgeThenGraceKills(t *testing.T) {
	clock := NewFakeClock(time.Now())
	var nudges, kills atomic.Int32
	reg := fastRegistry(clock)
	reg.Nudge = func(string) { nudges.Add(1) }
	reg.Kill = func(string) { kills.Add(1) }
	reg.Register("b1", "tg-1", "sess-1", "builder")

	clock.Advance(11 * time.Minute)
	settle()
	if nudges.Load() != 1 || kills.Load() != 0 {
		t.Fatalf("after idle: nudges=%d kills=%d", nudges.Load(), kills.Load())
	}

	clock.Advance(5 * time.Minute)
	settle()
	if kills.Load() != 1 {
		t.Fatalf("after grace: kills=%d, want 1", kills.Load())
	}
	if entry := reg.Snapshot()["b1"]; entry.State != SpunDown {
		t.Fatalf("state = %s, want %s", entry.State, SpunDown)
	}
}

// TestRegistryNudgeThenActivityNeverKills proves a Touch inside the grace window cancels the kill.
func TestRegistryNudgeThenActivityNeverKills(t *testing.T) {
	clock := NewFakeClock(time.Now())
	var kills atomic.Int32
	reg := fastRegistry(clock)
	reg.Kill = func(string) { kills.Add(1) }
	reg.Register("b1", "tg-1", "sess-1", "builder")

	clock.Advance(11 * time.Minute)
	settle()
	reg.Touch("b1")
	clock.Advance(5 * time.Minute)
	settle()

	if kills.Load() != 0 {
		t.Fatalf("kills = %d, want 0", kills.Load())
	}
	if entry := reg.Snapshot()["b1"]; entry.State != Running {
		t.Fatalf("state = %s, want %s", entry.State, Running)
	}
}

// TestRegistryPushbackResetsWindowOnTheSameSession proves a retry resumes the same builder and
// session, resetting the window instead of starting a new entry.
func TestRegistryPushbackResetsWindowOnTheSameSession(t *testing.T) {
	clock := NewFakeClock(time.Now())
	reg := fastRegistry(clock)
	before := reg.Register("b1", "tg-1", "sess-1", "builder")

	clock.Advance(20 * time.Minute)
	blocked := reg.Pushback("b1")
	after := reg.Snapshot()["b1"]

	if blocked {
		t.Fatal("the first pushback must not block")
	}
	if after.SessionID != before.SessionID || after.BuilderID != before.BuilderID {
		t.Fatalf("pushback changed identity: before=%+v after=%+v", before, after)
	}
	if !after.WindowStart.Equal(clock.Now()) {
		t.Fatalf("window not reset: %v, want %v", after.WindowStart, clock.Now())
	}
	if after.Retries != 1 {
		t.Fatalf("retries = %d, want 1", after.Retries)
	}
}

// TestRegistryThirdPushbackBlocks proves the third pushback spins the builder down instead of
// letting a fourth retry run.
func TestRegistryThirdPushbackBlocks(t *testing.T) {
	reg := fastRegistry(NewFakeClock(time.Now()))
	reg.Register("b1", "tg-1", "sess-1", "builder")

	if reg.Pushback("b1") {
		t.Fatal("pushback 1 must not block")
	}
	if reg.Pushback("b1") {
		t.Fatal("pushback 2 must not block")
	}
	if !reg.Pushback("b1") {
		t.Fatal("pushback 3 must block")
	}
	if entry := reg.Snapshot()["b1"]; entry.State != SpunDown {
		t.Fatalf("state = %s, want %s", entry.State, SpunDown)
	}
}

// TestRegistryThreeReviewerStrikesBlockTheBuilderPermanently proves three rounds of reviewer
// pushback on one builder block it, a state Pushback alone never reaches.
func TestRegistryThreeReviewerStrikesBlockTheBuilderPermanently(t *testing.T) {
	reg := fastRegistry(NewFakeClock(time.Now()))
	reg.Register("b1", "tg-1", "sess-1", "builder")

	if reg.Strike("b1") {
		t.Fatal("strike 1 must not block")
	}
	if reg.Strike("b1") {
		t.Fatal("strike 2 must not block")
	}
	if !reg.Strike("b1") {
		t.Fatal("strike 3 must block")
	}
	if entry := reg.Snapshot()["b1"]; entry.State != RegBlocked {
		t.Fatalf("state = %s, want %s", entry.State, RegBlocked)
	}
}

// TestRegistryStateSurvivesReload proves Load restores every entry Snapshot recorded, including
// a stalled entry's own window keeping on ticking.
func TestRegistryStateSurvivesReload(t *testing.T) {
	clock := NewFakeClock(time.Now())
	first := fastRegistry(clock)
	first.Register("b1", "tg-1", "sess-1", "builder")
	first.Pushback("b1")
	saved := first.Snapshot()

	second := fastRegistry(clock)
	second.Load(saved)
	reloaded := second.Snapshot()["b1"]
	if reloaded != saved["b1"] {
		t.Fatalf("reloaded = %+v, want %+v", reloaded, saved["b1"])
	}

	var killed atomic.Int32
	second.Kill = func(string) { killed.Add(1) }
	clock.Advance(31 * time.Minute)
	settle()
	if killed.Load() != 1 {
		t.Fatalf("a reloaded entry's own goroutine never resumed ticking: killed=%d", killed.Load())
	}
}

// TestRegistryReleaseStopsWatchingAndDropsTheEntry proves Release both frees the id for a fresh
// Register and stops its goroutine from firing again.
func TestRegistryReleaseStopsWatchingAndDropsTheEntry(t *testing.T) {
	clock := NewFakeClock(time.Now())
	var killed atomic.Int32
	reg := fastRegistry(clock)
	reg.Kill = func(string) { killed.Add(1) }
	reg.Register("b1", "tg-1", "sess-1", "builder")
	reg.Release("b1")

	clock.Advance(31 * time.Minute)
	settle()

	if killed.Load() != 0 {
		t.Fatalf("killed = %d, want 0 after Release", killed.Load())
	}
	if _, ok := reg.Snapshot()["b1"]; ok {
		t.Fatal("Release must drop the entry")
	}
}

// TestRegistryDoneMarksTheEntryWithNoFurtherTicking proves Done stops the goroutine and leaves
// the entry's state as done for Snapshot to report.
func TestRegistryDoneMarksTheEntryWithNoFurtherTicking(t *testing.T) {
	clock := NewFakeClock(time.Now())
	var killed atomic.Int32
	reg := fastRegistry(clock)
	reg.Kill = func(string) { killed.Add(1) }
	reg.Register("b1", "tg-1", "sess-1", "builder")
	reg.Done("b1")

	clock.Advance(31 * time.Minute)
	settle()

	if killed.Load() != 0 {
		t.Fatalf("killed = %d, want 0 after Done", killed.Load())
	}
	if entry := reg.Snapshot()["b1"]; entry.State != RegDone {
		t.Fatalf("state = %s, want %s", entry.State, RegDone)
	}
}

// TestRegistryTouchOnAnUnknownIDIsANoOp proves Touch, Pushback and Strike on an id never
// registered change nothing and never panic.
func TestRegistryTouchOnAnUnknownIDIsANoOp(t *testing.T) {
	reg := fastRegistry(NewFakeClock(time.Now()))
	reg.Touch("nobody")
	if reg.Pushback("nobody") {
		t.Fatal("pushback on an unknown id must not block")
	}
	if reg.Strike("nobody") {
		t.Fatal("strike on an unknown id must not block")
	}
	if len(reg.Snapshot()) != 0 {
		t.Fatal("no entry must be created")
	}
}

// TestRegistryOtherRoleGetsNoRetryLimit proves a role LimitsFor gives zero Retries never blocks
// on pushback, since there is no cap to reach.
func TestRegistryOtherRoleGetsNoRetryLimit(t *testing.T) {
	reg := fastRegistry(NewFakeClock(time.Now()))
	reg.Register("p1", "tg-1", "sess-1", "planner")
	for i := 0; i < 5; i++ {
		if reg.Pushback("p1") {
			t.Fatalf("pushback %d blocked a role with no retry limit", i+1)
		}
	}
}

// TestRegistryResumedUpdatesTheSessionID proves Resumed records a fresh handle on the same entry,
// never a new one, so a later Nudge or Kill always reaches the live session.
func TestRegistryResumedUpdatesTheSessionID(t *testing.T) {
	reg := NewRegistry(NewFakeClock(time.Now()))
	reg.Register("b1", "tg-1", "sess-1", "builder")
	reg.Resumed("b1", "sess-2")
	if len(reg.Snapshot()) != 1 {
		t.Fatalf("entries = %d, want one", len(reg.Snapshot()))
	}
	if entry := reg.Snapshot()["b1"]; entry.SessionID != "sess-2" {
		t.Fatalf("session id = %q, want sess-2", entry.SessionID)
	}
}
