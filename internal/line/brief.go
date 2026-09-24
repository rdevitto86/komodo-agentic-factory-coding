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
	profilepkg "komodo/internal/profile"
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
	group, _ := parsed.Group(task.GroupID)
	definition, err := LoadRole(root, role)
	if err != nil {
		return nil, err
	}
	standards, err := LoadStandards(root)
	if err != nil {
		return nil, err
	}
	caps := resolveCaps(root)
	result := filepath.Join(StateDir, "results", taskID+".json")
	tree, _ := detect.Detect(cwd)
	slots := map[string]string{
		"task_id":      task.ID,
		"title":        task.Title,
		"task_block":   blockText(parsed, task),
		"repo_rules":   repoRules(cwd, caps.RepoRules),
		"repo_context": repoContextSlot(cwd, task, caps.RepoContext),
		"context":      contextSlot(cwd, task, caps.PerFile, caps.FilesTotal),
		"files":        filesSlot(cwd, task, caps.PerFile, caps.FilesTotal),
		"repo_profile": repoProfileSlot(tree),
		"standards": standardsSlot(StandardsFor(standards, task.Files(), role)) +
			repoStandardsSlot(root) + facetAppendixSlot(root, tree, task, role),
		"done_when":   doneWhenSlot(task),
		"failure":     failureSlot(failure, caps.Failure),
		"result_path": result,
		"schema":      SchemaText(root, role),
	}
	text, err := Fill(definition.Body, slots)
	if err != nil {
		return nil, err
	}
	text = strings.TrimSpace(text) + resultLine(result, slots["schema"])
	worktree := filepath.Join(StateDir, "wt", taskID)
	if group.Mode() == "single" {
		// A single-mode group shares one builder and one worktree, so its tasks never split at QC.
		worktree = filepath.Join(StateDir, "wt", group.ID)
	}
	brief := &Brief{
		Task: taskID, Role: role, Result: result, Text: text, Tokens: Tokens(text),
		Path:     filepath.Join(StateDir, "briefs", taskID+".md"),
		Worktree: worktree,
		Slots:    map[string]int{},
	}
	for name, value := range slots {
		brief.Slots[name] = len(value)
	}
	return brief, nil
}

// FixBrief fills the builder role with the group's tasks, files, and blocking review findings, and
// writes it to .komodo/briefs/<group>-fix.md in the root and the group worktree.
func FixBrief(root string, plan *Plan) (*Brief, error) {
	path, err := backlog.Find(root)
	if err != nil {
		return nil, err
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return nil, err
	}
	definition, err := LoadRole(root, "builder")
	if err != nil {
		return nil, err
	}
	standards, err := LoadStandards(root)
	if err != nil {
		return nil, err
	}
	blocking, _ := SplitFindings(ReviewFindings(root, plan.Group), plan.Profile.SeverityFloor)
	if len(blocking) == 0 {
		return nil, fmt.Errorf("%s has no review finding at or above %s to fix", plan.Group, plan.Profile.SeverityFloor)
	}
	task, blocks := fixTask(parsed, plan)
	cwd := WorktreePath(root, plan.Worktree)
	caps := resolveCaps(root)
	result := filepath.Join(StateDir, "results", task.ID+".json")
	tree, _ := detect.Detect(cwd)
	failure := findingsText(blocking)
	if previous := RepairText(root, task.ID); previous != "" {
		failure += "\n\n# The last fix round failed its gate\n" + previous
	}
	slots := map[string]string{
		"task_id":      task.ID,
		"title":        task.Title,
		"task_block":   blocks,
		"repo_rules":   repoRules(cwd, caps.RepoRules),
		"repo_context": repoContextSlot(cwd, task, caps.RepoContext),
		"context":      contextSlot(cwd, task, caps.PerFile, caps.FilesTotal),
		"files":        filesSlot(cwd, task, caps.PerFile, caps.FilesTotal),
		"repo_profile": repoProfileSlot(tree),
		"standards": standardsSlot(StandardsFor(standards, task.Files(), "builder")) +
			repoStandardsSlot(root) + facetAppendixSlot(root, tree, task, "builder"),
		"done_when":   doneWhenSlot(task),
		"failure":     failureSlot(failure, caps.Failure),
		"result_path": result,
		"schema":      SchemaText(root, "builder"),
	}
	text, err := Fill(definition.Body, slots)
	if err != nil {
		return nil, err
	}
	text = strings.TrimSpace(text) + resultLine(result, slots["schema"])
	brief := &Brief{
		Task: task.ID, Role: "builder", Result: result, Text: text, Tokens: Tokens(text),
		Path:     filepath.Join(StateDir, "briefs", task.ID+".md"),
		Worktree: plan.Worktree,
		Slots:    map[string]int{},
	}
	for name, value := range slots {
		brief.Slots[name] = len(value)
	}
	for _, base := range []string{root, cwd} {
		full := filepath.Join(base, brief.Path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			return nil, err
		}
	}
	return brief, nil
}

// fixTask is one task standing for the whole group's fix round: every task's files, checks,
// context, and facets, plus each task's own yaml block for the brief.
func fixTask(parsed backlog.Backlog, plan *Plan) (backlog.Task, string) {
	lists := map[string][]any{}
	seen := map[string]bool{}
	var blocks []string
	for _, planned := range plan.Tasks {
		task, ok := parsed.Task(planned.ID)
		if !ok {
			continue
		}
		blocks = append(blocks, "# "+task.ID+": "+task.Title+"\n"+blockText(parsed, task))
		for key, values := range map[string][]string{
			"files": task.Files(), "done_when": task.DoneWhen(), "context": task.Context(), "facets": task.Facets(),
		} {
			for _, value := range values {
				if !seen[key+"\x00"+value] {
					seen[key+"\x00"+value] = true
					lists[key] = append(lists[key], value)
				}
			}
		}
	}
	task := backlog.Task{ID: plan.Group + "-fix", Title: "Fix the review findings on " + plan.Title, GroupID: plan.Group}
	task.Fields.Set("type", "fix")
	for _, key := range []string{"files", "done_when", "context", "facets"} {
		task.Fields.Set(key, lists[key])
	}
	return task, strings.Join(blocks, "\n\n")
}

// findingsText renders each blocking finding as one line a builder can act on.
func findingsText(findings []Finding) string {
	lines := []string{"# Blocking review findings"}
	for _, finding := range findings {
		lines = append(lines, fmt.Sprintf("- [%s] %s %s:%d %s: %s Fix: %s",
			finding.Severity, finding.Class, finding.File, finding.Line, finding.Title, finding.Detail, finding.Fix))
	}
	return strings.Join(lines, "\n")
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

// resolveCaps resolves the profile's slot caps, narrowed by the developer's own overlay, so a
// brief never carries more than the host running it can afford.
func resolveCaps(root string) profilepkg.Caps {
	return resolveProfile(root).Caps
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
