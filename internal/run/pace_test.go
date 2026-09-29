package run

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/profile"
)

// resetWindow is how far ahead a simulated rate-limit event puts the window's reset.
const resetWindow = 200 * time.Millisecond

// windowAfter plays the plan probe after a simulated rate-limit event: spent until the reset, fresh after it.
func windowAfter(plan string, limit mount.RateLimit) func(string) profile.Profile {
	return func(string) profile.Profile {
		if time.Now().Before(limit.ResetsAt) {
			return profile.Profile{Plan: plan, PauseAt: 0.9, Utilization: limit.FiveHour, ResetsAt: limit.ResetsAt}
		}
		return profile.Profile{Plan: plan, PauseAt: 0.9}
	}
}

// eventTypes reads events.jsonl's types in order.
func eventTypes(t *testing.T, root string) string {
	t.Helper()
	events, err := line.Book(root).ReadEvents()
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, event := range events {
		types = append(types, event.Type)
	}
	return strings.Join(types, " ")
}

func TestAwaitWindow(t *testing.T) {
	cases := []struct {
		name     string
		plan     string
		used     float64
		deadline time.Duration
		want     bool
		events   string
		waits    bool
	}{
		{"a rate-limit event pauses the drain until the reset, then resumes it", "max_5x", 0.95, time.Minute, true, "pause resume", true},
		{"a reset past the whole budget stops the drain", "max_5x", 0.95, resetWindow / 2, false, "pause", false},
		{"a window under the pause line never pauses", "max_5x", 0.5, time.Minute, true, "", false},
		{"API billing runs unbound past a spent window", "api", 1, time.Minute, true, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			saved := usageWindow
			t.Cleanup(func() { usageWindow = saved })
			root := t.TempDir()
			reset := time.Now().Add(resetWindow)
			usageWindow = windowAfter(tc.plan, mount.RateLimit{FiveHour: tc.used, ResetsAt: reset})
			var out bytes.Buffer
			if got := awaitWindow(root, &out, time.Now().Add(tc.deadline)); got != tc.want {
				t.Fatalf("awaitWindow = %v, want %v; out = %s", got, tc.want, out.String())
			}
			if waited := !time.Now().Before(reset); waited != tc.waits {
				t.Fatalf("waited for the reset = %v, want %v", waited, tc.waits)
			}
			if got := eventTypes(t, root); got != tc.events {
				t.Fatalf("events = %q, want %q", got, tc.events)
			}
		})
	}
}

func TestALiveRateLimitEventOverridesTheProbesEmptyWindow(t *testing.T) {
	t.Cleanup(mount.ClearRateLimit)
	saved := usageWindow
	t.Cleanup(func() { usageWindow = saved })
	// The plan probe carries no usage window on the current CLI; only the plan comes from it.
	usageWindow = func(string) profile.Profile { return profile.Profile{Plan: "max_5x", PauseAt: 0.9} }
	root := t.TempDir()
	reset := time.Now().Add(time.Hour)
	ObserveRateLimit(mount.RateLimit{FiveHour: 0.95, ResetsAt: reset})
	selected := currentWindow(root)
	if !selected.Paused() {
		t.Fatalf("a live rate-limit event did not pace an empty probe: %+v", selected)
	}
	if !selected.WaitUntil().Equal(reset) {
		t.Fatalf("wait until = %v, want %v", selected.WaitUntil(), reset)
	}
}

func TestADrainPausedByARateLimitResumesAtTheResetWithNoPerson(t *testing.T) {
	root := driveDrainRepo(t, driveDrainText)
	setupDrainDriveFakeClaude(t)
	// The rate-limit event lands on the drain's first window read, after its sync.
	var reset time.Time
	saved := usageWindow
	t.Cleanup(func() { usageWindow = saved })
	usageWindow = func(root string) profile.Profile {
		if reset.IsZero() {
			reset = time.Now().Add(resetWindow)
		}
		return windowAfter("max_5x", mount.RateLimit{FiveHour: 0.95, ResetsAt: reset})(root)
	}
	var out bytes.Buffer
	code, err := Launch(Options{Root: root, Budget: time.Minute, Stdout: &out, Stderr: &out, PR: fakeForge(t, root)})
	if err != nil || code != 0 {
		t.Fatalf("code = %d, err = %v, out = %s", code, err, out.String())
	}
	if time.Now().Before(reset) {
		t.Fatal("the drain finished before the window reset")
	}
	for _, want := range []string{"drain paused", "drain resumed", "2 shipped, 0 parked"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output lacks %q:\n%s", want, out.String())
		}
	}
	if got := eventTypes(t, root); got != "pause resume" {
		t.Fatalf("events = %q; the pause and the resume go to events.jsonl", got)
	}
}
