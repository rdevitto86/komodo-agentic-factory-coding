package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/check"
	"komodo/internal/line"
	"komodo/internal/review"
)

// blockingFloor is the lowest severity a lens returns for a finding it means to block.
const blockingFloor = "medium"

// lensDiff is the worktree's diff since it forked from base; a test swaps it.
var lensDiff = check.Diff

// checkEvidence refuses a lens's stop while a finding it means to block carries no evidence the binary verifies.
func checkEvidence(_ context.Context, in Input) (Outcome, error) {
	group := filepath.Base(in.Root)
	results, err := filepath.Glob(filepath.Join(in.Root, line.StateDir, "results", group+"-review*.json"))
	if err != nil {
		return Outcome{}, err
	}
	var findings []review.Finding
	for _, path := range results {
		data, err := os.ReadFile(path)
		if err != nil {
			return Outcome{}, err
		}
		var result struct {
			Findings []review.Finding `json:"findings"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return Outcome{}, fmt.Errorf("%s is not a review result: %w", path, err)
		}
		for _, finding := range result.Findings {
			if line.AtOrAbove(finding.Severity, blockingFloor) {
				findings = append(findings, finding)
			}
		}
	}
	if len(findings) == 0 {
		return Outcome{Verdict: Allow}, nil
	}
	tree, err := lensTree(in.Root, group)
	if err != nil {
		return Outcome{}, err
	}
	checked, err := review.Verify(tree, findings)
	if err != nil {
		return Outcome{}, err
	}
	var missing []string
	for _, each := range checked {
		if !each.Blocks {
			missing = append(missing, fmt.Sprintf("- %s:%d %s %s: %s",
				each.Finding.File, each.Finding.Line, each.Finding.RuleID, each.Finding.Title, each.Why))
		}
	}
	if len(missing) == 0 {
		return Outcome{Verdict: Allow}, nil
	}
	return Outcome{
		Verdict: Refuse,
		Message: "These findings carry no evidence the line can verify. Add it, or lower them to low; " +
			"once the refusals run out they become PR notes.\n\n" + strings.Join(missing, "\n"),
	}, nil
}

// lensTree is the worktree, its diff against the group's base, and the validators' report when one was saved.
func lensTree(root, group string) (review.Tree, error) {
	base := line.DefaultBase(root)
	if path, err := backlog.Find(root); err == nil {
		if parsed, err := backlog.Load(path); err == nil {
			if found, ok := parsed.Group(group); ok && found.Base() != "" {
				base = found.Base()
			}
		}
	}
	diff, err := lensDiff(root, base)
	if err != nil {
		return review.Tree{}, err
	}
	tree := review.Tree{Worktree: root, Diff: diff}
	data, err := os.ReadFile(filepath.Join(root, line.StateDir, "results", group+"-validators.json"))
	if errors.Is(err, os.ErrNotExist) {
		return tree, nil
	}
	if err != nil {
		return review.Tree{}, err
	}
	if err := json.Unmarshal(data, &tree.Report); err != nil {
		return review.Tree{}, fmt.Errorf("the validators' report is not JSON: %w", err)
	}
	return tree, nil
}
