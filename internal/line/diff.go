package line

import (
	"fmt"
	"strconv"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/detect"
	"komodo/internal/facet"
	"komodo/internal/git"
	repopkg "komodo/internal/repo"
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
	ref := StartRef(worktree, plan.Base)
	names, err := git.Run(worktree, "diff", "--name-only", ref+"...HEAD")
	if err != nil {
		return nil, err
	}
	input := &ReviewInput{Group: plan.Group, Title: plan.Title, Base: plan.Base, Branch: plan.Branch}
	for _, name := range strings.Split(names, "\n") {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			input.Files = append(input.Files, unquotePath(trimmed))
		}
	}
	body, err := git.Run(worktree, "diff", ref+"...HEAD", "--", ":(exclude)bin")
	if err != nil {
		return nil, err
	}
	pieces := diffPieces(input.Files, fileDiffs(body))
	input.Lines = strings.Count(strings.Join(pieces, "\n"), "\n")
	input.Diff = clipDiff(pieces, CapDiff)
	input.Tasks = taskBlocks(root, plan)
	standards, err := LoadStandards(root)
	if err != nil {
		return nil, err
	}
	input.Standards = standardsSlot(StandardsFor(standards, input.Files, "reviewer")) +
		repoStandardsSlot(root) + facetReviewSlot(root, worktree, plan)
	input.Text = strings.Join([]string{
		fmt.Sprintf("# Review of group %s: %s", plan.Group, plan.Title),
		"\n# Tasks the diff was meant to deliver\n" + input.Tasks,
		"\n# Standards for the languages in the diff\n" + input.Standards,
		fmt.Sprintf("\n# Diff against %s\n```diff\n%s\n```", plan.Base, input.Diff),
	}, "\n")
	return input, nil
}

// diffPieces renders one diff chunk per changed file, naming a binary by size and any file
// whose chunk never matched, so a changed file never silently drops out of the review.
func diffPieces(files []string, byFile map[string]string) []string {
	pieces := make([]string, 0, len(files))
	for _, name := range files {
		chunk, ok := byFile[name]
		switch {
		case strings.HasPrefix(name, "bin/"):
			pieces = append(pieces, fmt.Sprintf("[%s is a prebuilt binary; its bytes are never read]", name))
		case ok:
			pieces = append(pieces, chunk)
		default:
			pieces = append(pieces, fmt.Sprintf("[%s changed but its diff chunk could not be matched; review it directly]", name))
		}
	}
	return pieces
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
		if next := diffGitName(line); next != "" {
			flush()
			current = nil
			name = next
		}
		current = append(current, line)
	}
	flush()
	return chunks
}

// diffGitName is the post-change path a "diff --git" header names, plain or C-quoted; the
// quoted form wraps a/ and b/ in double quotes when either path holds a non-ASCII byte.
func diffGitName(line string) string {
	if rest, ok := strings.CutPrefix(line, "diff --git a/"); ok {
		if index := strings.Index(rest, " b/"); index >= 0 {
			return rest[index+len(" b/"):]
		}
		return rest
	}
	rest, ok := strings.CutPrefix(line, `diff --git "a/`)
	if !ok {
		return ""
	}
	index := strings.Index(rest, `" "b/`)
	if index < 0 {
		return ""
	}
	quoted := strings.TrimSuffix(rest[index+len(`" "b/`):], `"`)
	return unquoteCPath(quoted)
}

// unquotePath strips git's C-quoting from a path, or returns it unchanged when git left it bare.
func unquotePath(name string) string {
	if len(name) >= 2 && name[0] == '"' && name[len(name)-1] == '"' {
		return unquoteCPath(name[1 : len(name)-1])
	}
	return name
}

// unquoteCPath decodes the body of a C-quoted path: octal byte escapes and the usual backslash ones.
func unquoteCPath(quoted string) string {
	var out []byte
	for i := 0; i < len(quoted); i++ {
		if quoted[i] != '\\' || i+1 >= len(quoted) {
			out = append(out, quoted[i])
			continue
		}
		next := quoted[i+1]
		if next >= '0' && next <= '7' && i+3 < len(quoted) {
			if value, err := strconv.ParseUint(quoted[i+1:i+4], 8, 8); err == nil {
				out = append(out, byte(value))
				i += 3
				continue
			}
		}
		out = append(out, unescape(next))
		i++
	}
	return string(out)
}

// unescape is the byte one backslash-letter escape stands for, or the letter itself when it names none.
func unescape(letter byte) byte {
	switch letter {
	case 'a':
		return '\a'
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	case 'v':
		return '\v'
	default:
		return letter
	}
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

// repoStandardsSlot renders every repo standard override, which a shipped standards skill never carries.
func repoStandardsSlot(root string) string {
	overrides, _ := repopkg.LoadStandards(root)
	if len(overrides) == 0 {
		return ""
	}
	var parts []string
	for _, override := range overrides {
		parts = append(parts, Clip(override.Body, CapStandard, override.Name))
	}
	return "\n\n---\n\n" + strings.Join(parts, "\n\n---\n\n")
}

// facetReviewSlot renders each selected facet's reviewer appendix, the way a builder brief
// carries its own, so a reviewer reads the same platform notes a builder did.
func facetReviewSlot(root, worktree string, plan *Plan) string {
	tree, _ := detect.Detect(worktree)
	names, err := facet.Select(root, tree, facetsForPlan(root, plan))
	if err != nil {
		return ""
	}
	var parts []string
	for _, name := range names {
		loaded, err := facet.Load(root, name)
		if err != nil {
			continue
		}
		if appendix := loaded.ReviewerAppendix(); appendix != "" {
			parts = append(parts, Clip(appendix, CapFacet, loaded.Name))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "\n\n---\n\n" + strings.Join(parts, "\n\n---\n\n")
}

// facetsForPlan unions the facets every task in the group declares.
func facetsForPlan(root string, plan *Plan) []string {
	path, err := backlog.Find(root)
	if err != nil {
		return nil
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return nil
	}
	var declared []string
	for _, task := range plan.Tasks {
		if current, ok := parsed.Task(task.ID); ok {
			declared = append(declared, current.Facets()...)
		}
	}
	return declared
}
