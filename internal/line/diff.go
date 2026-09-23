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
	byFile := fileDiffs(body)
	pieces := make([]string, 0, len(input.Files))
	for _, name := range input.Files {
		if strings.HasPrefix(name, "bin/") {
			pieces = append(pieces, fmt.Sprintf("[%s is a prebuilt binary; its bytes are never read]", name))
			continue
		}
		if chunk, ok := byFile[name]; ok {
			pieces = append(pieces, chunk)
		}
	}
	input.Lines = strings.Count(strings.Join(pieces, "\n"), "\n")
	input.Diff = clipDiff(pieces, CapDiff)
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

// fileDiffs splits a multi-file diff into one chunk per file, keyed by its post-change name.
func fileDiffs(body string) map[string]string {
	chunks := map[string]string{}
	var name string
	var current []string
	flush := func() {
		if name != "" {
			chunks[name] = strings.Join(current, "\n")
		}
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "diff --git a/") {
			flush()
			current = nil
			rest := strings.TrimPrefix(line, "diff --git a/")
			if index := strings.Index(rest, " b/"); index >= 0 {
				name = rest[index+len(" b/"):]
			} else {
				name = rest
			}
		}
		current = append(current, line)
	}
	flush()
	return chunks
}

// clipDiff joins per-file diff chunks up to limit, dropping whole files rather than cutting a hunk.
func clipDiff(pieces []string, limit int) string {
	var kept []string
	size := 0
	for index, piece := range pieces {
		size += len(piece) + 1
		if size > limit && len(kept) > 0 {
			omitted := len(pieces) - index
			marker := fmt.Sprintf("\n[... diff clipped at a file boundary: %d of %d files shown, %d files omitted ...]", index, len(pieces), omitted)
			return strings.Join(kept, "\n") + marker
		}
		if size > limit {
			return Clip(piece, limit, "diff")
		}
		kept = append(kept, piece)
	}
	return strings.Join(kept, "\n")
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
