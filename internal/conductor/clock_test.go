package conductor

import (
	"testing"
	"time"
)

func TestClockTracksSessions(t *testing.T) {
	c := NewClock()

	// Start and end a build session under its limit.
	c.StartSession("build")
	time.Sleep(100 * time.Millisecond)
	c.EndSession("build")

	if c.SessionPastLimit("build") {
		t.Fatal("build session under 25 minutes should not be past limit")
	}
	if c.GroupPastLimit() {
		t.Fatal("group under 60 minutes should not be past limit")
	}
}

func TestClockKillsSessionPastLimit(t *testing.T) {
	c := NewClock()

	// Manually add time to simulate a session over its limit.
	c.sessionUsed["build"] = 26 * time.Minute
	c.groupUsed = 26 * time.Minute

	if !c.SessionPastLimit("build") {
		t.Fatal("build session at 26 minutes should be past 25-minute limit")
	}
}

func TestClockStopsGroupAt60Minutes(t *testing.T) {
	c := NewClock()

	// Simulate session usage that exceeds 60 minutes total.
	c.sessionUsed["build"] = 25 * time.Minute
	c.sessionUsed["review"] = 8 * time.Minute
	c.sessionUsed["repair"] = 10 * time.Minute
	c.sessionUsed["re-review"] = 20 * time.Minute // This pushes total to 63 minutes.
	c.groupUsed = 63 * time.Minute

	if !c.GroupPastLimit() {
		t.Fatal("group at 63 minutes should be past 60-minute limit")
	}
}

func TestClockAccumulatesMultipleSessions(t *testing.T) {
	c := NewClock()

	// Simulate three separate repair sessions accumulating time.
	c.sessionUsed["repair"] = 9 * time.Minute
	c.groupUsed = 9 * time.Minute

	// Repair limit is 10, so not yet past.
	if c.SessionPastLimit("repair") {
		t.Fatal("repair at 9 minutes should not be past 10-minute limit")
	}

	// Add more time to breach the limit.
	c.sessionUsed["repair"] += 2 * time.Minute
	c.groupUsed += 2 * time.Minute

	if !c.SessionPastLimit("repair") {
		t.Fatal("repair at 11 minutes should be past 10-minute limit")
	}
}

func TestClockTracksEachSessionTypeLimit(t *testing.T) {
	cases := []struct {
		name       string
		sessionType string
		minutes    int
		wantLimit  bool
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
			clock.sessionUsed[tc.sessionType] = time.Duration(tc.minutes) * time.Minute
			got := clock.SessionPastLimit(tc.sessionType)
			if got != tc.wantLimit {
				t.Fatalf("SessionPastLimit(%s at %d min) = %v, want %v",
					tc.sessionType, tc.minutes, got, tc.wantLimit)
			}
		})
	}
}

func TestClockSessionUsedReturnsAccumulatedTime(t *testing.T) {
	c := NewClock()

	c.sessionUsed["build"] = 15 * time.Minute
	got := c.SessionUsed("build")
	want := 15 * time.Minute

	if got != want {
		t.Fatalf("SessionUsed(build) = %v, want %v", got, want)
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

func TestClockREQ29FakeSessionPastLimitIsKilledAndGroupStopsBy60Minutes(t *testing.T) {
	c := NewClock()

	// Simulate a fake session past its limit.
	c.sessionUsed["build"] = 26 * time.Minute
	c.groupUsed = 26 * time.Minute

	// The session should be marked as past limit.
	if !c.SessionPastLimit("build") {
		t.Fatal("fake session at 26 minutes should be past 25-minute limit and killed")
	}

	// Add more sessions to approach group limit.
	c.sessionUsed["review"] = 8 * time.Minute
	c.sessionUsed["repair"] = 10 * time.Minute
	c.sessionUsed["re-review"] = 15 * time.Minute // Total: 26 + 8 + 10 + 15 = 59 minutes.
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
