package eval

import (
	"strings"
	"testing"
)

// outcome builds one shipped run, accepted when its hidden tests passed.
func outcome(platform, group string, accepted bool, tokens int, minutes float64) Outcome {
	return Outcome{
		Repo: "greet", Group: group, Platform: platform, Shipped: true, Passed: accepted,
		Sessions: 2, Turns: 5, Tokens: tokens, Minutes: minutes, ReviewRounds: 1,
	}
}

// TestSummarizeReportsEachPlatformsNumbers proves pass rate, consistency, cost and tokens per accepted group.
func TestSummarizeReportsEachPlatformsNumbers(t *testing.T) {
	report := Summarize([]Outcome{
		outcome("linux", "TG-01.1", true, 100, 10),
		outcome("linux", "TG-01.1", true, 100, 30),
		outcome("linux", "TG-01.2", true, 100, 20),
		outcome("linux", "TG-01.2", false, 100, 70),
		outcome("darwin", "TG-01.1", true, 300, 5),
	})
	if len(report.Platforms) != 2 || report.Platforms[0].Platform != "darwin" {
		t.Fatalf("platforms = %+v, want darwin then linux", report.Platforms)
	}
	linux := report.Platforms[1]
	want := Row{
		Platform: "linux", Groups: 2, Runs: 4, Accepted: 3, PassRate: 0.75, Consistency: 0.5,
		Sessions: 8, Turns: 20, Tokens: 400, MedianMinutes: 25, MaxMinutes: 70, ReviewRounds: 4,
		TokensPerAccepted: 400.0 / 3,
	}
	if linux != want {
		t.Fatalf("linux = %+v\nwant    %+v", linux, want)
	}
	darwin := report.Platforms[0]
	if darwin.PassRate != 1 || darwin.Consistency != 1 || darwin.MedianMinutes != 5 || darwin.TokensPerAccepted != 300 {
		t.Fatalf("darwin = %+v", darwin)
	}
}

// TestSummarizeWithNothingAcceptedReportsZeroTokensPerAccepted proves a run with no accepted group divides by nothing.
func TestSummarizeWithNothingAcceptedReportsZeroTokensPerAccepted(t *testing.T) {
	row := Summarize([]Outcome{outcome("linux", "TG-01.1", false, 100, 1)}).Platforms[0]
	if row.PassRate != 0 || row.TokensPerAccepted != 0 || row.Consistency != 1 {
		t.Fatalf("row = %+v", row)
	}
	if report := Summarize(nil); len(report.Platforms) != 0 {
		t.Fatalf("Summarize(nil) = %+v, want no platforms", report)
	}
}

// TestPrintNamesEveryNumberPerPlatform proves the printed report carries every number the eval reports.
func TestPrintNamesEveryNumberPerPlatform(t *testing.T) {
	var out strings.Builder
	Summarize([]Outcome{outcome("linux", "TG-01.1", true, 100, 12)}).Print(&out)
	for _, want := range []string{
		"linux: 1 groups, 1 runs", "pass rate         100% (1/1)", "consistency       100%", "sessions          2",
		"turns             5", "tokens            100", "minutes           median 12.0, max 12.0",
		"review rounds     1", "tokens/accepted   100",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("report = %q, want %q", out.String(), want)
		}
	}
}
