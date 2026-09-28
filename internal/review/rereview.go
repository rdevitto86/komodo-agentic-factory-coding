package review

import "komodo/internal/check"

// spot is one file and line a finding points at.
type spot struct {
	file string
	line int
}

// Rereview keeps what a resumed lens may return: a finding on an open finding's line, or on a line the
// repair's diff added. Every other finding sits on a line already reviewed, and is dropped.
func Rereview(open, returned []Finding, repair string) []Finding {
	allowed := make(map[spot]bool, len(open))
	for _, finding := range open {
		allowed[spot{finding.File, finding.Line}] = true
	}
	for _, added := range check.ParseAddedLines(repair) {
		allowed[spot{added.File, added.Line}] = true
	}
	kept := make([]Finding, 0, len(returned))
	for _, finding := range returned {
		if allowed[spot{finding.File, finding.Line}] {
			kept = append(kept, finding)
		}
	}
	return kept
}
