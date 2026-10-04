package line

import (
	"fmt"
	"strconv"
	"strings"

	"komodo/internal/guard"
	"komodo/internal/ledger"
	"komodo/internal/pr"
)

// MergeResult is what merging a shipped group's pull request into its base did.
type MergeResult struct {
	Group  string `json:"group"`
	Base   string `json:"base"`
	Number int    `json:"number"`
	URL    string `json:"url,omitempty"`
}

// MergeGroup merges a shipped group's pull request into its base with a merge commit, never a
// squash, refusing a critical base, an unshipped group, a blocking finding, or a failed check.
func MergeGroup(root string, plan *Plan, client *pr.Client) (*MergeResult, error) {
	if client == nil {
		return nil, fmt.Errorf("%s has no pull request client; nothing was merged", plan.Group)
	}
	if guard.Load(root, root).IsCritical(plan.Base) {
		return nil, fmt.Errorf("%s targets the critical ref %q; landing is the human's merge button", plan.Group, plan.Base)
	}
	if !shipped(root, plan) {
		return nil, fmt.Errorf("%s has not shipped; ship it, then merge", plan.Group)
	}
	blocking, _ := SplitFindings(ReviewFindings(root, plan.Group), plan.Profile.SeverityFloor)
	if len(blocking) > 0 {
		return nil, fmt.Errorf("the review left %d finding(s) at or above %s on %s; fix them, then merge",
			len(blocking), plan.Profile.SeverityFloor, plan.Branch)
	}
	if checksFailed(root, plan.Group) {
		return nil, fmt.Errorf("%s has a failed check; fix it, then merge", plan.Group)
	}
	pull, err := client.View("")
	if err != nil {
		return nil, err
	}
	if pull.State != "OPEN" {
		return nil, fmt.Errorf("%s's pull request is %s, not open; nothing to merge", plan.Group, strings.ToLower(pull.State))
	}
	number := strconv.Itoa(pull.Number)
	if err := client.Merge(number); err != nil {
		return nil, err
	}
	return &MergeResult{Group: plan.Group, Base: plan.Base, Number: pull.Number, URL: pull.URL}, nil
}

// checksFailed reports whether any QC wave the group stamped did not pass.
func checksFailed(root, group string) bool {
	entries, err := Book(root).Read(ledger.RunFile)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Station == "qc" && entry.Group == group && entry.Outcome != "done" {
			return true
		}
	}
	return false
}
