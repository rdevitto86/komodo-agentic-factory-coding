package line

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/detect"
	"komodo/internal/facet"
	repopkg "komodo/internal/repo"
	"komodo/internal/toolkit"
)

// SkillsDir is where the shipped skills live, relative to the repo root.
const SkillsDir = "komodo/skills"

// CapRepoProfile bounds the repo profile slot, about 200 characters.
const CapRepoProfile = 200

// CapFacet bounds a facet's appendix, its own cap distinct from a standard's.
const CapFacet = 2000

var placeholder = regexp.MustCompile(`\{\{\s*([a-z_]+)\s*\}\}`)

// Brief is one filled role template with the paths it was written to.
type Brief struct {
	Task     string         `json:"task"`
	Role     string         `json:"role"`
	Path     string         `json:"brief"`
	Worktree string         `json:"worktree"`
	Result   string         `json:"result"`
	Slots    map[string]int `json:"slots,omitempty"`
	Tokens   int            `json:"tokens"`
	Text     string         `json:"-"`
}

// Standard is one standards skill: its name, what triggers it, and its body.
type Standard struct {
	Name  string
	Globs []string
	Roles []string
	Body  string
}

// LoadStandards reads every standards skill the repo ships.
func LoadStandards(root string) ([]Standard, error) {
	entries, err := fs.ReadDir(toolkit.FS(root), "skills")
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Standard
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "standards-") {
			continue
		}
		data, err := fs.ReadFile(toolkit.FS(root), path.Join("skills", entry.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		match := frontmatter.FindStringSubmatch(string(data))
		if match == nil {
			continue
		}
		standard := Standard{Name: strings.TrimPrefix(entry.Name(), "standards-"), Body: match[2]}
		for _, line := range strings.Split(match[1], "\n") {
			key, value, found := strings.Cut(line, ":")
			if !found {
				continue
			}
			switch strings.TrimSpace(key) {
			case "globs":
				standard.Globs = trimQuotes(splitList(strings.TrimSpace(value)))
			case "roles":
				standard.Roles = splitList(strings.TrimSpace(value))
			}
		}
		out = append(out, standard)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// trimQuotes strips the quotes a frontmatter list puts around each glob.
func trimQuotes(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, strings.Trim(item, `"'`))
	}
	return out
}

// StandardsFor picks the standards a task's files and the role pull in.
func StandardsFor(all []Standard, files []string, role string) []Standard {
	var out []Standard
	for _, standard := range all {
		hit := contains(standard.Roles, role)
		for _, glob := range standard.Globs {
			if hit {
				break
			}
			for _, file := range files {
				if MatchGlob(glob, file) {
					hit = true
					break
				}
			}
		}
		if hit {
			out = append(out, standard)
		}
	}
	return out
}

// BuildBrief fills a role's template for one task from the worktree it will be built in.
func BuildBrief(root, cwd, taskID, role string, failure string) (*Brief, error) {
	path, err := backlog.Find(root)
	if err != nil {
		return nil, err
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return nil, err
	}
	task, ok := parsed.Task(taskID)
	if !ok {
		return nil, fmt.Errorf("no task %s in %s", taskID, path)
	}
	definition, err := LoadRole(root, role)
	if err != nil {
		return nil, err
	}
	standards, err := LoadStandards(root)
	if err != nil {
		return nil, err
	}
	result := filepath.Join(StateDir, "results", taskID+".json")
	tree, _ := detect.Detect(cwd)
	slots := map[string]string{
		"task_id":      task.ID,
		"title":        task.Title,
		"task_block":   blockText(parsed, task),
		"repo_rules":   repoRules(cwd),
		"repo_context": repoContextSlot(cwd, task),
		"context":      contextSlot(cwd, task),
		"files":        filesSlot(cwd, task),
		"repo_profile": repoProfileSlot(tree),
		"standards":    standardsSlot(StandardsFor(standards, task.Files(), role)) + facetAppendixSlot(root, tree, task, role),
		"done_when":    doneWhenSlot(task),
		"failure":      failureSlot(failure),
		"result_path":  result,
		"schema":       SchemaText(root, role),
	}
	text, err := Fill(definition.Body, slots)
	if err != nil {
		return nil, err
	}
	text = strings.TrimSpace(text) + resultLine(result, slots["schema"])
	brief := &Brief{
		Task: taskID, Role: role, Result: result, Text: text, Tokens: Tokens(text),
		Path:     filepath.Join(StateDir, "briefs", taskID+".md"),
		Worktree: filepath.Join(StateDir, "wt", taskID),
		Slots:    map[string]int{},
	}
	for name, value := range slots {
		brief.Slots[name] = len(value)
	}
	return brief, nil
}

// resultLine tells the machine where its result JSON belongs and the exact shape it must have.
func resultLine(result, schema string) string {
	line := "\n\n# Your result\nWrite this JSON to `" + result + "`.\n"
	if schema == "" {
		return line
	}
	return line + "\nIt must satisfy this schema. Every key under `required` is mandatory, at every level.\n\n```json\n" + schema + "\n```\n"
}

// Fill substitutes every placeholder; an unsupplied slot fails so nothing ships half-filled.
func Fill(template string, slots map[string]string) (string, error) {
	var missing []string
	out := placeholder.ReplaceAllStringFunc(template, func(match string) string {
		key := placeholder.FindStringSubmatch(match)[1]
		value, ok := slots[key]
		if !ok {
			missing = append(missing, key)
			return match
		}
		return value
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("brief slot(s) not supplied: %s", strings.Join(missing, ", "))
	}
	return out, nil
}

// blockText is the task's own yaml block, verbatim.
func blockText(parsed backlog.Backlog, task backlog.Task) string {
	if task.BlockStart < 0 || task.BlockEnd <= task.BlockStart {
		return ""
	}
	return strings.Join(parsed.Lines[task.BlockStart+1:task.BlockEnd], "\n")
}

// repoRules is the repo's own AGENTS.md, clipped, or a one-line default.
func repoRules(cwd string) string {
	data, err := os.ReadFile(filepath.Join(cwd, "AGENTS.md"))
	if err == nil {
		return Clip(string(data), CapRepoRules, "AGENTS.md")
	}
	return "No repo-level rules file. Follow the standards below and the code's existing idioms."
}

// repoContextSlot renders every repo context file whose paths match the task's files, clipped.
func repoContextSlot(cwd string, task backlog.Task) string {
	contexts, _ := repopkg.LoadContext(cwd)
	var parts []string
	for _, context := range contexts {
		if context.Matches(task.Files()) {
			parts = append(parts, Clip(context.Body, CapRepoContext, context.Name))
		}
	}
	if len(parts) == 0 {
		return "None declared for this repo."
	}
	return strings.Join(parts, "\n\n---\n\n")
}

// contextSlot resolves each context anchor to its section, clipped.
func contextSlot(cwd string, task backlog.Task) string {
	var parts []string
	for _, ref := range task.Context() {
		path, anchor, _ := strings.Cut(ref, "#")
		data, err := os.ReadFile(filepath.Join(cwd, path))
		if err != nil {
			parts = append(parts, fmt.Sprintf("### %s\n[%s does not exist yet]", ref, path))
			continue
		}
		text := string(data)
		if anchor != "" {
			if section := Section(text, anchor); section != "" {
				text = section
			}
		}
		parts = append(parts, fmt.Sprintf("### %s\n%s", ref, Clip(text, CapPerFile, ref)))
	}
	if len(parts) == 0 {
		return "None beyond the files below."
	}
	return strings.Join(parts, "\n\n")
}

// filesSlot reads every listed file, names a binary one by size, and never reads bin/.
func filesSlot(cwd string, task backlog.Task) string {
	files := task.Files()
	if len(files) == 0 {
		return "No files listed."
	}
	perFile := CapPerFile
	if perFile*len(files) > CapFilesTotal {
		perFile = CapFilesTotal / len(files)
		if perFile < 2000 {
			perFile = 2000
		}
	}
	var parts []string
	for _, path := range files {
		full := filepath.Join(cwd, path)
		info, err := os.Stat(full)
		switch {
		case err != nil:
			parts = append(parts, fmt.Sprintf("### %s\n[does not exist yet]", path))
		case info.IsDir():
			parts = append(parts, fmt.Sprintf("### %s\n[a directory, %s]", path, path))
		case strings.HasPrefix(strings.ReplaceAll(path, "\\", "/"), "bin/"):
			parts = append(parts, fmt.Sprintf("### %s\n[a prebuilt binary, %d bytes, never read]", path, info.Size()))
		case !IsText(full):
			parts = append(parts, fmt.Sprintf("### %s\n[not text, %d bytes, never read]", path, info.Size()))
		default:
			data, err := os.ReadFile(full)
			if err != nil {
				parts = append(parts, fmt.Sprintf("### %s\n[unreadable: %v]", path, err))
				continue
			}
			parts = append(parts, fmt.Sprintf("### %s\n```\n%s\n```", path, Clip(string(data), perFile, path)))
		}
	}
	return strings.Join(parts, "\n\n")
}

// standardsSlot renders each selected standard, clipped by its own cap.
func standardsSlot(selected []Standard) string {
	if len(selected) == 0 {
		return "No language standard matches these files."
	}
	var parts []string
	for _, standard := range selected {
		parts = append(parts, Clip(strings.TrimSpace(standard.Body), CapStandard, standard.Name))
	}
	return strings.Join(parts, "\n\n---\n\n")
}

// repoProfileSlot summarises the repo's own detected profile: languages, cloud, data, CI, and verify.
func repoProfileSlot(profile detect.Profile) string {
	fields := []string{
		"languages: " + joinOrNone(profile.Languages),
		"cloud: " + joinOrNone(profile.Cloud),
		"data: " + joinOrNone(profile.Data),
		"ci: " + joinOrNone(profile.CI),
		"verify: " + joinOrNone(nonEmpty(profile.Verify)),
	}
	return Clip(strings.Join(fields, "; "), CapRepoProfile, "repo profile")
}

// joinOrNone joins a list with commas, or names it none when empty.
func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ",")
}

// nonEmpty wraps a single string into a one-item list, dropping it when empty.
func nonEmpty(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

// facetAppendixSlot renders the appendix each selected facet carries for this role, its own cap.
func facetAppendixSlot(root string, profile detect.Profile, task backlog.Task, role string) string {
	names, err := facet.Select(root, profile, task.Facets())
	if err != nil {
		return ""
	}
	var parts []string
	for _, name := range names {
		loaded, err := facet.Load(root, name)
		if err != nil {
			continue
		}
		appendix := loaded.BuilderAppendix()
		if role == "reviewer" {
			appendix = loaded.ReviewerAppendix()
		}
		if appendix == "" {
			continue
		}
		parts = append(parts, Clip(appendix, CapFacet, loaded.Name))
	}
	if len(parts) == 0 {
		return ""
	}
	return "\n\n---\n\n" + strings.Join(parts, "\n\n---\n\n")
}

// doneWhenSlot lists the commands whose zero exit proves the task done.
func doneWhenSlot(task backlog.Task) string {
	var lines []string
	for _, command := range task.DoneWhen() {
		lines = append(lines, "- `"+command+"`")
	}
	return strings.Join(lines, "\n")
}

// failureSlot carries the previous attempt into a repair brief, clipped.
func failureSlot(failure string) string {
	if strings.TrimSpace(failure) == "" {
		return ""
	}
	return "\n# Previous attempt failed\nFix the cause. Never weaken the check.\n```\n" +
		Clip(failure, CapFailure, "failure") + "\n```"
}

// WriteBrief creates the task worktree from the group branch and writes the brief into it.
func WriteBrief(root string, brief *Brief, groupBranch string) error {
	worktree := filepath.Join(root, brief.Worktree)
	if _, err := os.Stat(worktree); err != nil {
		branch := "task/" + strings.ToLower(brief.Task)
		if err := AddWorktree(root, branch, groupBranch, worktree); err != nil {
			return err
		}
	}
	for _, base := range []string{root, worktree} {
		path := filepath.Join(base, brief.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(brief.Text), 0o644); err != nil {
			return err
		}
	}
	return nil
}
