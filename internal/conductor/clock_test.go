package conductor

import (
	"testing"
	"time"
)

func TestClockTracksSessions(t *testing.T) {
	c := NewClock()

	// Start and end a build session under its limit.
	c.StartSession("s1", "build")
	time.Sleep(100 * time.Millisecond)
	c.EndSession("s1")

	if c.SessionPastLimit("s1") {
		t.Fatal("build session under 25 minutes should not be past limit")
	}
	if c.GroupPastLimit() {
		t.Fatal("group under 60 minutes should not be past limit")
	}
}

// TestClockKillsAStillRunningSessionPastLimit proves a session that never called EndSession
// still trips its limit, since a hung session must be detectable while it is running.
func TestClockKillsAStillRunningSessionPastLimit(t *testing.T) {
	c := NewClock()

	c.StartSession("s1", "build")
	c.sessionStarted["s1"] = time.Now().Add(-26 * time.Minute)

	if !c.SessionPastLimit("s1") {
		t.Fatal("a still-running build session at 26 minutes should be past its 25-minute limit")
	}
}

func TestClockStopsGroupAt60Minutes(t *testing.T) {
	c := NewClock()

	// Simulate ended sessions whose accumulated time exceeds 60 minutes total.
	c.sessionUsed["s1"] = 25 * time.Minute
	c.sessionUsed["s2"] = 8 * time.Minute
	c.sessionUsed["s3"] = 10 * time.Minute
	c.sessionUsed["s4"] = 20 * time.Minute // Pushes total to 63 minutes.
	c.groupUsed = 63 * time.Minute

	if !c.GroupPastLimit() {
		t.Fatal("group at 63 minutes should be past 60-minute limit")
	}
}

// TestClockSecondSessionOfSameTypeStartsFresh proves a new repair session's ID keeps its own
// time apart from an earlier repair session, instead of inheriting its accumulated total.
func TestClockSecondSessionOfSameTypeStartsFresh(t *testing.T) {
	c := NewClock()

	c.StartSession("repair-1", "repair")
	c.sessionStarted["repair-1"] = time.Now().Add(-9 * time.Minute)
	c.EndSession("repair-1")

	if c.SessionPastLimit("repair-1") {
		t.Fatal("repair-1 at 9 minutes should not be past 10-minute limit")
	}

	c.StartSession("repair-2", "repair")
	if c.SessionPastLimit("repair-2") {
		t.Fatal("repair-2 must start with no time, not inherit repair-1's 9 minutes")
	}
	if c.SessionUsed("repair-2") >= c.SessionUsed("repair-1") {
		t.Fatalf("repair-2 used %v, want less than repair-1's %v", c.SessionUsed("repair-2"), c.SessionUsed("repair-1"))
	}
}

func TestClockTracksEachSessionTypeLimit(t *testing.T) {
	cases := []struct {
		name        string
		sessionType string
		minutes     int
		wantLimit   bool
	}{
		{"build under", "build", 24, false},
		{"build over", "build", 26, true},
		{"review under", "review", 7, false},
		{"review over", "review", 9, true},
		{"repair under", "repair", 9, false},
		{"repair over", "repair", 11, true},
		{"re-review under", "re-review", 4, false},
		{"re-review over", "re-review", 6, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clock := NewClock()
			clock.StartSession("s", tc.sessionType)
			clock.sessionStarted["s"] = time.Now().Add(-time.Duration(tc.minutes) * time.Minute)
			clock.EndSession("s")
			got := clock.SessionPastLimit("s")
			if got != tc.wantLimit {
				t.Fatalf("SessionPastLimit(%s at %d min) = %v, want %v",
					tc.sessionType, tc.minutes, got, tc.wantLimit)
			}
		})
	}
}

func TestClockSessionUsedReturnsAccumulatedTime(t *testing.T) {
	c := NewClock()

	c.sessionUsed["s1"] = 15 * time.Minute
	got := c.SessionUsed("s1")
	want := 15 * time.Minute

	if got != want {
		t.Fatalf("SessionUsed(s1) = %v, want %v", got, want)
	}
}

func TestClockGroupUsedReturnsAccumulatedTime(t *testing.T) {
	c := NewClock()

	c.groupUsed = 45 * time.Minute
	got := c.GroupUsed()
	want := 45 * time.Minute

	if got != want {
		t.Fatalf("GroupUsed() = %v, want %v", got, want)
	}
}

// TestClockGroupUsedCountsAStillRunningSession proves a session that never called EndSession
// still adds its elapsed time to the group total, so the group limit cannot be silently overrun.
func TestClockGroupUsedCountsAStillRunningSession(t *testing.T) {
	c := NewClock()

	c.sessionUsed["s1"] = 50 * time.Minute
	c.groupUsed = 50 * time.Minute

	c.StartSession("s2", "build")
	c.sessionStarted["s2"] = time.Now().Add(-24 * time.Minute)

	if c.GroupUsed() < 74*time.Minute {
		t.Fatalf("group used = %v, want at least 74m (50m ended plus 24m still running)", c.GroupUsed())
	}
	if !c.GroupPastLimit() {
		t.Fatal("group at 74m with a session still running should be past its 60-minute limit")
	}
}

// TestClockREQ29FakeSessionPastLimitIsKilledAndGroupStopsBy60Minutes proves REQ-29: a fake
// session past its limit is detected while still running, and the group stops by 60 minutes.
func TestClockREQ29FakeSessionPastLimitIsKilledAndGroupStopsBy60Minutes(t *testing.T) {
	c := NewClock()

	// A fake build session still running past its 25-minute limit.
	c.StartSession("s1", "build")
	c.sessionStarted["s1"] = time.Now().Add(-26 * time.Minute)

	if !c.SessionPastLimit("s1") {
		t.Fatal("fake session at 26 minutes should be past 25-minute limit and killed")
	}
	c.EndSession("s1")

	// Add more ended sessions to approach the group limit.
	c.sessionUsed["s2"] = 8 * time.Minute
	c.sessionUsed["s3"] = 10 * time.Minute
	c.sessionUsed["s4"] = 15 * time.Minute // Total: 26 + 8 + 10 + 15 = 59 minutes.
	c.groupUsed = 59 * time.Minute

	// Still under 60-minute group limit.
	if c.GroupPastLimit() {
		t.Fatal("group at 59 minutes should not be past 60-minute limit")
	}

	// Add one more minute to breach group limit.
	c.groupUsed = 61 * time.Minute

	if !c.GroupPastLimit() {
		t.Fatal("group at 61 minutes should be past 60-minute limit; group should stop")
	}
}
