package eval

import (
	"fmt"
	"io"
	"sort"
)

// Report is an eval's numbers, one row per platform.
type Report struct {
	Platforms []Row `json:"platforms"`
}

// Row is one platform's numbers: how often groups were accepted, how often runs agreed, and what they cost.
type Row struct {
	Platform string `json:"platform"`
	Groups   int    `json:"groups"`
	Runs     int    `json:"runs"`
	Accepted int    `json:"accepted"`
	// PassRate is the accepted runs over every run.
	PassRate float64 `json:"pass_rate"`
	// Consistency is the groups whose every run had the same outcome over every group.
	Consistency   float64 `json:"consistency"`
	Sessions      int     `json:"sessions"`
	Turns         int     `json:"turns"`
	Tokens        int     `json:"tokens"`
	MedianMinutes float64 `json:"median_minutes"`
	MaxMinutes    float64 `json:"max_minutes"`
	ReviewRounds  int     `json:"review_rounds"`
	// TokensPerAccepted is every run's tokens over the accepted runs, zero when none was accepted.
	TokensPerAccepted float64 `json:"tokens_per_accepted"`
}

// Summarize reduces outcomes to one row per platform, sorted by platform.
func Summarize(outcomes []Outcome) Report {
	byPlatform := map[string][]Outcome{}
	for _, outcome := range outcomes {
		byPlatform[outcome.Platform] = append(byPlatform[outcome.Platform], outcome)
	}
	platforms := make([]string, 0, len(byPlatform))
	for platform := range byPlatform {
		platforms = append(platforms, platform)
	}
	sort.Strings(platforms)
	report := Report{Platforms: make([]Row, 0, len(platforms))}
	for _, platform := range platforms {
		report.Platforms = append(report.Platforms, summarizePlatform(platform, byPlatform[platform]))
	}
	return report
}

// summarizePlatform is one platform's row.
func summarizePlatform(platform string, outcomes []Outcome) Row {
	row := Row{Platform: platform, Runs: len(outcomes)}
	verdicts := map[string][]bool{}
	var order []string
	minutes := make([]float64, 0, len(outcomes))
	for _, outcome := range outcomes {
		key := outcome.Repo + "/" + outcome.Group
		if _, seen := verdicts[key]; !seen {
			order = append(order, key)
		}
		verdicts[key] = append(verdicts[key], outcome.Accepted())
		if outcome.Accepted() {
			row.Accepted++
		}
		row.Sessions += outcome.Sessions
		row.Turns += outcome.Turns
		row.Tokens += outcome.Tokens
		row.ReviewRounds += outcome.ReviewRounds
		minutes = append(minutes, outcome.Minutes)
	}
	row.Groups = len(order)
	consistent := 0
	for _, key := range order {
		same := true
		for _, verdict := range verdicts[key] {
			same = same && verdict == verdicts[key][0]
		}
		if same {
			consistent++
		}
	}
	if row.Runs > 0 {
		row.PassRate = float64(row.Accepted) / float64(row.Runs)
	}
	if row.Groups > 0 {
		row.Consistency = float64(consistent) / float64(row.Groups)
	}
	if row.Accepted > 0 {
		row.TokensPerAccepted = float64(row.Tokens) / float64(row.Accepted)
	}
	sort.Float64s(minutes)
	if count := len(minutes); count > 0 {
		row.MaxMinutes = minutes[count-1]
		row.MedianMinutes = minutes[count/2]
		if count%2 == 0 {
			row.MedianMinutes = (minutes[count/2-1] + minutes[count/2]) / 2
		}
	}
	return row
}

// Print writes one block per platform.
func (r Report) Print(w io.Writer) {
	for _, row := range r.Platforms {
		fmt.Fprintf(w, "%s: %d groups, %d runs\n", row.Platform, row.Groups, row.Runs)
		fmt.Fprintf(w, "  pass rate         %.0f%% (%d/%d)\n", row.PassRate*100, row.Accepted, row.Runs)
		fmt.Fprintf(w, "  consistency       %.0f%%\n", row.Consistency*100)
		fmt.Fprintf(w, "  sessions          %d\n", row.Sessions)
		fmt.Fprintf(w, "  turns             %d\n", row.Turns)
		fmt.Fprintf(w, "  tokens            %d\n", row.Tokens)
		fmt.Fprintf(w, "  minutes           median %.1f, max %.1f\n", row.MedianMinutes, row.MaxMinutes)
		fmt.Fprintf(w, "  review rounds     %d\n", row.ReviewRounds)
		fmt.Fprintf(w, "  tokens/accepted   %.0f\n", row.TokensPerAccepted)
	}
}
