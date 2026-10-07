package proc

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHeartbeatKillsASilentSessionButNotAnActiveOne(t *testing.T) {
	old := BeatInterval
	BeatInterval = 10 * time.Millisecond
	defer func() { BeatInterval = old }()

	var quietKills atomic.Int32
	quiet := Beat(func() { quietKills.Add(1) }, 0, 50*time.Millisecond)
	defer quiet.Stop()

	var busyKills atomic.Int32
	busy := Beat(func() { busyKills.Add(1) }, 0, 50*time.Millisecond)
	defer busy.Stop()
	for i := 0; i < 15; i++ {
		busy.Write([]byte("x"))
		time.Sleep(10 * time.Millisecond)
	}
	if busyKills.Load() != 0 {
		t.Fatalf("an active session was killed: %q", busy.Breach())
	}

	time.Sleep(100 * time.Millisecond)
	if quietKills.Load() != 1 || !strings.Contains(quiet.Breach(), "silent") {
		t.Fatalf("silent session: kills=%d breach=%q", quietKills.Load(), quiet.Breach())
	}
}

func TestHeartbeatKillsASessionPastItsWallClock(t *testing.T) {
	old := BeatInterval
	BeatInterval = 10 * time.Millisecond
	defer func() { BeatInterval = old }()

	var kills atomic.Int32
	beat := Beat(func() { kills.Add(1) }, 40*time.Millisecond, 0)
	defer beat.Stop()
	for i := 0; i < 10; i++ {
		beat.Write([]byte("x"))
		time.Sleep(10 * time.Millisecond)
	}
	if kills.Load() != 1 || !strings.Contains(beat.Breach(), "wall clock") {
		t.Fatalf("kills=%d breach=%q", kills.Load(), beat.Breach())
	}
}
