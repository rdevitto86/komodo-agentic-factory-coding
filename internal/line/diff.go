package line

import (
	"fmt"
	"strings"

	"komodo/internal/backlog"
)

// CapDiff is how much diff the reviewer's brief carries.
const CapDiff = CapFailure

// ReviewInput is the whole input a reviewer gets: the tasks, the standards, and the diff.
type ReviewInput struct {
	Group     string   `json:"group"`
	Title     string   `json:"title"`
	Base      string   `json:"base"`
	Branch    string   `json:"branch"`
	Tasks     string   `json:"-"`
	Standards string   `json:"-"`
	Diff      string   `json:"-"`
	Files     []string `json:"files"`
	Lines     int      `json:"lines"`
	Text      string   `json:"-"`
}

// DiffFor renders the group's diff against its base with the task blocks and the standards it touches.
func DiffFor(root string, plan *Plan) (*ReviewInput, error) {
	worktree := WorktreePath(root, plan.Worktree)
	names, err := git(worktree, "diff", "--name-only", plan.Base+"...HEAD")
	if err != nil {
		return nil, err
	}
	input := &ReviewInput{Group: plan.Group, Title: plan.Title, Base: plan.Base, Branch: plan.Branch}
	for _, name := range strings.Split(names, "\n") {
		if strings.TrimSpace(name) != "" {
			input.Files = append(input.Files, name)
		}
	}
	body, err := git(worktree, "diff", plan.Base+"...HEAD", "--", ":(exclude)bin")
	if err != nil {
		return nil, err
	}
	for _, name := range input.Files {
		if strings.HasPrefix(name, "bin/") {
			body += fmt.Sprintf("\n[%s is a prebuilt binary; its bytes are never read]\n", name)
		}
	}
	input.Lines = strings.Count(body, "\n")
	input.Diff = Clip(body, CapDiff, "diff")
	input.Tasks = taskBlocks(root, plan)
	standards, err := LoadStandards(root)
	if err != nil {
		return nil, err
	}
	input.Standards = standardsSlot(StandardsFor(standards, input.Files, "reviewer"))
	input.Text = strings.Join([]string{
		fmt.Sprintf("# Review of group %s: %s", plan.Group, plan.Title),
		"\n# Tasks the diff was meant to deliver\n" + input.Tasks,
		"\n# Standards for the languages in the diff\n" + input.Standards,
		fmt.Sprintf("\n# Diff against %s\n```diff\n%s\n```", plan.Base, input.Diff),
	}, "\n")
	return input, nil
}

// taskBlocks renders every task block in the group, which is what the diff was meant to deliver.
func taskBlocks(root string, plan *Plan) string {
	path, err := backlog.Find(root)
	if err != nil {
		return ""
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return ""
	}
	var out []string
	for _, task := range plan.Tasks {
		current, ok := parsed.Task(task.ID)
		if !ok {
			continue
		}
		out = append(out, fmt.Sprintf("## %s %s\n```yaml\n%s\n```", current.ID, current.Title, blockText(parsed, current)))
	}
	return strings.Join(out, "\n\n")
}
