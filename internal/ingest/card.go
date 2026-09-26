// Package ingest compiles each READY backlog group into a card: its task list, files, checks,
// context references and a stable hash, with zero model calls (REQ-7).
package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/glob"
	"komodo/internal/line"
)

// QueueDir is where a compiled card lands, one file per group, gitignored, never committed.
const QueueDir = ".komodo/queue"

// skipDir names directories file expansion never descends into.
var skipDir = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, ".komodo": true, "bin": true,
}

// CardTask is one checkbox the group's task list carries, in file order.
type CardTask struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// Size counts what a card compiled, so a group's scope is visible before any session spends a token.
type Size struct {
	Tasks      int `json:"tasks"`
	Files      int `json:"files"`
	Packages   int `json:"packages"`
	BriefBytes int `json:"brief_bytes"`
}

// Card is one READY group compiled for a builder session, with a hash stable over its content.
type Card struct {
	Group   string     `json:"group"`
	Title   string     `json:"title"`
	Type    string     `json:"type"`
	Version string     `json:"version"`
	Base    string     `json:"base"`
	Tier    string     `json:"tier"`
	Tasks   []CardTask `json:"tasks"`
	Files   []string   `json:"files"`
	Checks  []string   `json:"checks"`
	Context []string   `json:"context"`
	Size    Size       `json:"size"`
	Hash    string     `json:"hash"`
}

// Path is where a group's card lands, relative to the repo root.
func Path(groupID string) string {
	return filepath.Join(QueueDir, groupID+".json")
}

// ReadyGroups are the groups in parsed holding at least one ready agent task, in file order.
func ReadyGroups(parsed backlog.Backlog) []backlog.Group {
	var out []backlog.Group
	for _, group := range parsed.Groups {
		if group.HasReadyTask() {
			out = append(out, group)
		}
	}
	return out
}

// Build compiles one group into a card from its parsed backlog and the tree at root.
func Build(root string, parsed backlog.Backlog, group backlog.Group) (Card, error) {
	files, err := expandFiles(root, taskFiles(group))
	if err != nil {
		return Card{}, err
	}
	tier, err := builderTier(root, group)
	if err != nil {
		return Card{}, err
	}
	card := Card{
		Group:   group.ID,
		Title:   group.Title,
		Type:    group.Type(),
		Version: group.Version(),
		Base:    resolveBase(root, parsed, group),
		Tier:    tier,
		Tasks:   cardTasks(group),
		Files:   files,
		Checks:  handWrittenChecks(group),
		Context: taskContext(group),
	}
	card.Size = Size{
		Tasks:      len(card.Tasks),
		Files:      len(card.Files),
		Packages:   countPackages(card.Files),
		BriefBytes: briefBytes(parsed, group),
	}
	card.Hash = hash(card)
	return card, nil
}

// Write compiles a group's card and saves it to .komodo/queue/<group>.json under root.
func Write(root string, parsed backlog.Backlog, group backlog.Group) (Card, error) {
	card, err := Build(root, parsed, group)
	if err != nil {
		return Card{}, err
	}
	path := filepath.Join(root, Path(group.ID))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Card{}, err
	}
	data, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		return Card{}, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return Card{}, err
	}
	return card, nil
}

// cardTasks renders the group's checkboxes in order.
func cardTasks(group backlog.Group) []CardTask {
	tasks := make([]CardTask, 0, len(group.Tasks))
	for _, task := range group.Tasks {
		tasks = append(tasks, CardTask{ID: task.ID, Title: task.Title, Done: task.Status == "DONE"})
	}
	return tasks
}

// taskFiles collects every file pattern the group's tasks declare, in first-seen order.
func taskFiles(group backlog.Group) []string {
	var out []string
	seen := map[string]bool{}
	for _, task := range group.Tasks {
		for _, file := range task.Files() {
			if !seen[file] {
				seen[file] = true
				out = append(out, file)
			}
		}
	}
	return out
}

// handWrittenChecks collects every done_when command the group's tasks declare, in first-seen
// order; per-language derived checks are added alongside these, never replacing them.
func handWrittenChecks(group backlog.Group) []string {
	var out []string
	seen := map[string]bool{}
	for _, task := range group.Tasks {
		for _, check := range task.DoneWhen() {
			if !seen[check] {
				seen[check] = true
				out = append(out, check)
			}
		}
	}
	return out
}

// taskContext collects every context reference the group's tasks cite, in first-seen order; the
// context pack later renders each into its section, file bodies, and callers.
func taskContext(group backlog.Group) []string {
	var out []string
	seen := map[string]bool{}
	for _, task := range group.Tasks {
		for _, ref := range task.Context() {
			if !seen[ref] {
				seen[ref] = true
				out = append(out, ref)
			}
		}
	}
	return out
}

// expandFiles turns each glob or directory into the files it names on disk; a plain path that
// doesn't exist is kept only when its parent directory does.
func expandFiles(root string, patterns []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(path string) {
		if !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	for _, pattern := range patterns {
		clean := strings.TrimSuffix(filepath.ToSlash(pattern), "/")
		info, err := os.Stat(filepath.Join(root, clean))
		switch {
		case err == nil && info.IsDir():
			matches, err := filesUnder(root, clean)
			if err != nil {
				return nil, err
			}
			for _, match := range matches {
				add(match)
			}
		case err == nil:
			add(clean)
		case strings.ContainsAny(pattern, "*?["):
			matches, err := filesMatching(root, pattern)
			if err != nil {
				return nil, err
			}
			for _, match := range matches {
				add(match)
			}
		case parentExists(root, clean):
			add(clean)
		}
	}
	sort.Strings(out)
	return out, nil
}

// filesUnder lists every file under a declared directory, relative to root.
func filesUnder(root, dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skipDir[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	return out, err
}

// filesMatching walks the whole tree for every file a glob pattern matches.
func filesMatching(root, pattern string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if skipDir[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if glob.Match(pattern, rel) {
			out = append(out, rel)
		}
		return nil
	})
	return out, err
}

// parentExists reports whether path's parent directory exists under root, allowing a new file
// only where the directory it belongs in is already real.
func parentExists(root, path string) bool {
	dir := filepath.Dir(path)
	if dir == "." {
		return true
	}
	info, err := os.Stat(filepath.Join(root, dir))
	return err == nil && info.IsDir()
}

// countPackages counts the distinct directories a card's Go files sit in.
func countPackages(files []string) int {
	dirs := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, ".go") {
			dirs[filepath.Dir(file)] = true
		}
	}
	return len(dirs)
}

// briefBytes sums the raw byte size of each task's title and its own yaml block, an estimate of
// what the card's task list costs a brief before the context pack adds its own bytes.
func briefBytes(parsed backlog.Backlog, group backlog.Group) int {
	total := 0
	for _, task := range group.Tasks {
		total += len(task.Title)
		if task.BlockStart >= 0 && task.BlockEnd > task.BlockStart {
			for _, line := range parsed.Lines[task.BlockStart+1 : task.BlockEnd] {
				total += len(line) + 1
			}
		}
	}
	return total
}

// resolveBase is the branch a group's work targets: an explicit base, the branch of a group named
// in depends_on, or the repo's default.
func resolveBase(root string, parsed backlog.Backlog, group backlog.Group) string {
	if base := group.Base(); base != "" {
		return base
	}
	for _, id := range group.DependsOn() {
		if other, ok := parsed.Group(id); ok {
			return other.Branch()
		}
	}
	return line.DefaultBase(root)
}

// builderTier is the builder role's tier, overridden to heavy only when a task in the group asks for it.
func builderTier(root string, group backlog.Group) (string, error) {
	role, err := line.LoadRole(root, "builder")
	if err != nil {
		return "", err
	}
	for _, task := range group.Tasks {
		if task.Tier() == "heavy" {
			return "heavy", nil
		}
	}
	return role.Tier, nil
}

// hash returns the sha256 of the card's own JSON, computed with the hash field blank, so the same
// content always gives the same card hash (REQ-7).
func hash(card Card) string {
	card.Hash = ""
	data, err := json.Marshal(card)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
