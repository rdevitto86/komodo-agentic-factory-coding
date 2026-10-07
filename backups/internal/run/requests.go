package run

import (
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/conductor"
	"komodo/internal/harness"
	"komodo/internal/mount"
	"komodo/internal/profile"
	"komodo/internal/review"
)

// BuilderRequest fills the builder's start request: every task's brief, in wave order, joined into
// one prompt, the builder role's tools and schema, and the model and effort profile.Machine resolves.
func BuilderRequest(root string, plan *harness.Plan) (mount.StartRequest, error) {
	definition, err := harness.LoadRole(root, "builder")
	if err != nil {
		return mount.StartRequest{}, err
	}
	worktree := harness.WorktreePath(root, plan.Worktree)
	var briefs []string
	for _, wave := range plan.Waves {
		for _, taskID := range wave {
			brief, err := harness.BuildBrief(root, worktree, taskID, "builder", "")
			if err != nil {
				return mount.StartRequest{}, err
			}
			briefs = append(briefs, brief.Text)
		}
	}
	machine, _ := plan.Profile.Machine("builder")
	limits := profile.LimitsFor("builder")
	return mount.StartRequest{
		Role:   "builder",
		Brief:  strings.Join(briefs, "\n\n---\n\n"),
		Tools:  definition.Tools,
		Model:  machine.Model,
		Effort: machine.Effort,
		Schema: []byte(harness.SchemaText(root, "builder")),
		Window: limits.Window, Idle: limits.Idle, Grace: limits.Grace,
	}, nil
}

// ReReviewInput is what a lens's resumed reviewer reads each round after its first: the lens's open
// findings and the diff since the HEAD its last review saw.
func ReReviewInput(root string, plan *harness.Plan, s conductor.State, lens review.Lens) (string, error) {
	input, err := harness.ReReviewFor(root, plan, s.Reviewed, s.Open(lens))
	if err != nil {
		return "", err
	}
	return input.Text, nil
}

// ReviewerRequest fills one lens's start request: the review brief with its lens section, bound
// to the reviewer role.
func ReviewerRequest(root string, plan *harness.Plan, lens review.Lens) (mount.StartRequest, error) {
	definition, err := harness.LoadRole(root, "reviewer")
	if err != nil {
		return mount.StartRequest{}, err
	}
	briefPath, err := harness.ReviewBrief(root, plan, string(lens))
	if err != nil {
		return mount.StartRequest{}, err
	}
	text, err := os.ReadFile(filepath.Join(root, briefPath))
	if err != nil {
		return mount.StartRequest{}, err
	}
	machine := plan.Profile.Tiers.Reviewer
	limits := profile.LimitsFor("reviewer")
	return mount.StartRequest{
		Role:   "reviewer",
		Brief:  string(text),
		Tools:  definition.Tools,
		Model:  machine.Model,
		Effort: machine.Effort,
		Schema: []byte(harness.SchemaText(root, "reviewer")),
		Window: limits.Window, Idle: limits.Idle, Grace: limits.Grace,
	}, nil
}
