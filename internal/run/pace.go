package run

import (
	"fmt"
	"io"
	"time"

	"komodo/internal/ledger"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/profile"
)

// Pace bounds: the longest single wait, and the recheck when a reset passed but the window stays spent.
const (
	longestPause = 5 * time.Hour
	pauseRecheck = time.Minute
)

// Pace ledger values: the station a pause stamps, and the outcomes events.jsonl reads as a pause and a resume.
const (
	stationPace    = "pace"
	outcomePaused  = "paused"
	outcomeResumed = "resumed"
)

// usageWindow reads the plan and its usage window from the plan probe; a test swaps it.
var usageWindow = profile.Select

// ObserveRateLimit records a session's live rate_limit_event, so the next wait paces from it
// instead of the plan probe's stale or empty usage window; the conductor calls this as it streams.
func ObserveRateLimit(limit mount.RateLimit) {
	mount.ObserveRateLimit(limit)
}

// currentWindow is the plan probe's profile, with any live rate-limit event layered over its
// usage window and reset time.
func currentWindow(root string) profile.Profile {
	selected := usageWindow(root)
	if limit, ok := mount.LatestRateLimit(); ok {
		selected.Utilization = limit.FiveHour
		selected.ResetsAt = limit.ResetsAt
	}
	return selected
}

// awaitWindow holds the drain while the plan's usage window is too far spent, stamping a pause and then a
// resume to events.jsonl, and reports false when the reset falls past the deadline.
func awaitWindow(root string, stdout io.Writer, deadline time.Time) bool {
	selected := currentWindow(root)
	if !selected.Paused() {
		return true
	}
	fmt.Fprintf(stdout, "drain paused: the usage window is %.0f%% spent; resuming at %s\n",
		selected.Utilization*100, selected.WaitUntil().Format(time.RFC3339))
	stampPace(root, stdout, outcomePaused)
	for selected.Paused() {
		wait := time.Until(selected.WaitUntil())
		if wait <= 0 || wait > longestPause {
			wait = pauseRecheck
		}
		if time.Now().Add(wait).After(deadline) {
			fmt.Fprintln(stdout, "drain stopped: the usage window resets past the whole budget")
			return false
		}
		time.Sleep(wait)
		selected = currentWindow(root)
	}
	stampPace(root, stdout, outcomeResumed)
	fmt.Fprintln(stdout, "drain resumed: the usage window reset")
	return true
}

// stampPace writes one pause or resume to events.jsonl, printing a failure rather than stopping the drain.
func stampPace(root string, stdout io.Writer, outcome string) {
	entry := ledger.Entry{At: time.Now().UTC(), Station: stationPace, Outcome: outcome}
	if err := line.Book(root).WriteEvents([]ledger.Entry{entry}); err != nil {
		fmt.Fprintf(stdout, "drain could not record the %s: %v\n", outcome, err)
	}
}
