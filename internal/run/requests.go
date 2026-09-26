package run

import (
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/line"
	"komodo/internal/mount"
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

// ReviewerRequest fills the reviewer's start request: the group's review brief, which carries the
// diff, the reviewer role's tools and schema, and the reviewer tier's machine, not profile.Machine.
func ReviewerRequest(root string, plan *line.Plan) (mount.StartRequest, error) {
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
		Brief:  string(text),
		Tools:  definition.Tools,
		Model:  machine.Model,
		Effort: machine.Effort,
		Schema: []byte(line.SchemaText(root, "reviewer")),
	}, nil
}
