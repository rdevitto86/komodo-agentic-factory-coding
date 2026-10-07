package conductor

import (
	"errors"
	"fmt"
	"strings"

	"komodo/internal/review"
)

// errNoProgress stops a group's loop once a round leaves it where it was, sending it on with its open findings.
var errNoProgress = errors.New("no progress")

// findingSpot is where a finding sits; a round that raises none there has closed it.
type findingSpot struct {
	file string
	line int
}

// openFindings returns the verified findings the lenses hold, in lens order.
func openFindings(s State, lenses []review.Lens) []Finding {
	var open []Finding
	for _, lens := range lenses {
		for _, finding := range s.Findings[lens] {
			if finding.Verified {
				open = append(open, finding)
			}
		}
	}
	return open
}

// closesNothing reports whether a round left every finding open before it still open.
func closesNothing(before, after []Finding) bool {
	if len(before) == 0 {
		return false
	}
	still := make(map[findingSpot]bool, len(after))
	for _, finding := range after {
		still[findingSpot{finding.File, finding.Line}] = true
	}
	for _, finding := range before {
		if !still[findingSpot{finding.File, finding.Line}] {
			return false
		}
	}
	return true
}

// stalledReview is the stop for a review round that closed nothing, listing the findings it left open.
func stalledReview(open []Finding) error {
	listed := make([]string, 0, len(open))
	for _, finding := range open {
		listed = append(listed, fmt.Sprintf("%s:%d %s %s", finding.File, finding.Line, finding.Severity, finding.Title))
	}
	return fmt.Errorf(
		"%w: the review round closed none of its open findings: %s", errNoProgress, strings.Join(listed, "; "),
	)
}
