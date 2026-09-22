package repo

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SkillsDir is where a repo adds a new skill or overrides a shipped one.
const SkillsDir = ".komodo/skills"

// protectedSkills are the skills a repo can never append to or replace.
var protectedSkills = []string{"run", "review", "backlog", "respond"}

// SkillOverride is one repo skill: a whole new skill, or the "Repo overrides" text for a shipped one.
type SkillOverride struct {
	Name string
	New  bool
	Body string
}

// LoadSkills reads every repo skill override, skipping a protected name with a note.
func LoadSkills(root string) ([]SkillOverride, []string) {
	entries, err := os.ReadDir(filepath.Join(root, SkillsDir))
	if err != nil {
		return nil, nil
	}
	var out []SkillOverride
	var skipped []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if contains(protectedSkills, name) {
			skipped = append(skipped, name+": run, review, backlog, and respond cannot be appended to")
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, SkillsDir, name, "SKILL.md"))
		if err != nil {
			skipped = append(skipped, name+": "+err.Error())
			continue
		}
		text := string(data)
		out = append(out, SkillOverride{Name: name, New: frontmatter.MatchString(text), Body: strings.TrimSpace(text)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, skipped
}

// contains reports whether the slice holds the value.
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
