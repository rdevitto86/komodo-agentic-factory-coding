package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/conductor"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/review"
)

// BuilderRequest fills the builder's start request: every task's brief, in wave order, joined into
// one prompt, the builder role's tools and schema, and the model and effort profile.Machine resolves.
func BuilderRequest(root string, plan *line.Plan) (mount.StartRequest, error) {
	definition, err := line.LoadRole(root, "builder")
	if err != nil {
		return mount.StartRequest{}, err
	}
	worktree := line.WorktreePath(root, plan.Worktree)
	var briefs []string
	for _, wave := range plan.Waves {
		for _, taskID := range wave {
			brief, err := line.BuildBrief(root, worktree, taskID, "builder", "")
			if err != nil {
				return mount.StartRequest{}, err
			}
			briefs = append(briefs, brief.Text)
		}
	}
	machine, _ := plan.Profile.Machine("builder")
	return mount.StartRequest{
		Role:   "builder",
		Brief:  strings.Join(briefs, "\n\n---\n\n"),
		Tools:  definition.Tools,
		Model:  machine.Model,
		Effort: machine.Effort,
		Schema: []byte(line.SchemaText(root, "builder")),
	}, nil
}

// ReReviewInput is what a lens's resumed reviewer reads each round after its first: the lens's open
// findings and the diff since the HEAD its last review saw.
func ReReviewInput(root string, plan *line.Plan, s conductor.State, lens review.Lens) (string, error) {
	input, err := line.ReReviewFor(root, plan, s.Reviewed, s.Open(lens))
	if err != nil {
		return "", err
	}
	return input.Text, nil
}

// lensLead opens a lens's brief, binding its session to the one checklist skill it reviews through.
func lensLead(lens review.Lens) string {
	return fmt.Sprintf("# Your lens: %s\n\nReview through the `%s` skill alone, and report only the rules it lists.\n\n",
		lens, lens.Skill())
}

// ReviewerRequest fills one lens's start request: the group's review brief, which carries the diff, bound
// to the lens's skill, the reviewer role's tools and schema, and the reviewer tier's machine for every lens.
func ReviewerRequest(root string, plan *line.Plan, lens review.Lens) (mount.StartRequest, error) {
	definition, err := line.LoadRole(root, "reviewer")
	if err != nil {
		return mount.StartRequest{}, err
	}
	briefPath, err := line.ReviewBrief(root, plan)
	if err != nil {
		return mount.StartRequest{}, err
	}
	text, err := os.ReadFile(filepath.Join(root, briefPath))
	if err != nil {
		return mount.StartRequest{}, err
	}
	machine := plan.Profile.Tiers.Reviewer
	return mount.StartRequest{
		Role:   "reviewer",
		Brief:  lensLead(lens) + string(text),
		Tools:  definition.Tools,
		Model:  machine.Model,
		Effort: machine.Effort,
		Schema: []byte(line.SchemaText(root, "reviewer")),
	}, nil
}
