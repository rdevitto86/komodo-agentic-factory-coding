// Package mount holds what the line knows about a host, and nothing outside it names one.
package mount

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Verbs are the five tools a role may ask for. A host tool name never appears in a role.
var Verbs = []string{"read", "edit", "write", "shell", "search"}

// Role is one role file as a mount renders it.
type Role struct {
	Name        string
	Description string
	Tier        string
	Tools       []string
	Session     bool
	Returns     string
	Body        string
}

// Skill is one skill directory a mount copies.
type Skill struct {
	Name string
	Body string
}

var frontmatter = regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n(.*)\z`)

// LoadRoles reads every shipped role, sorted by name.
func LoadRoles(root string) ([]Role, error) {
	dir := filepath.Join(root, "komodo", "roles")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var roles []Role
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		match := frontmatter.FindStringSubmatch(string(data))
		if match == nil {
			continue
		}
		role := Role{Body: match[2]}
		for _, line := range strings.Split(match[1], "\n") {
			key, value, found := strings.Cut(line, ":")
			if !found {
				continue
			}
			value = strings.TrimSpace(value)
			switch strings.TrimSpace(key) {
			case "name":
				role.Name = value
			case "description":
				role.Description = value
			case "tier":
				role.Tier = value
			case "session":
				role.Session = value == "true"
			case "returns":
				role.Returns = value
			case "tools":
				role.Tools = splitList(value)
			}
		}
		roles = append(roles, role)
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].Name < roles[j].Name })
	return roles, nil
}

// LoadSkills reads every shipped skill, sorted by name.
func LoadSkills(root string) ([]Skill, error) {
	dir := filepath.Join(root, "komodo", "skills")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var skills []Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		skills = append(skills, Skill{Name: entry.Name(), Body: string(data)})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

// MergeOverride appends a repo skill that is new, or folds one that collides into the shipped skill of that name.
func MergeOverride(skills *[]Skill, byName map[string]int, name string, isNew bool, body string) {
	index, ok := byName[name]
	if !ok {
		if !isNew {
			return
		}
		byName[name] = len(*skills)
		*skills = append(*skills, Skill{Name: name, Body: body})
		return
	}
	(*skills)[index].Body = strings.TrimRight((*skills)[index].Body, "\n") + "\n\n## Repo overrides\n\n" + stripFrontmatter(body) + "\n"
}

// stripFrontmatter removes a leading frontmatter block, so an override never nests one in a body.
func stripFrontmatter(body string) string {
	if match := frontmatter.FindStringSubmatch(body); match != nil {
		return strings.TrimSpace(match[2])
	}
	return body
}

// Instructions is a role's body without the brief template, which is what a session agent reads.
func (r Role) Instructions() string {
	if index := strings.Index(r.Body, "\n# Brief\n"); index >= 0 {
		return strings.TrimSpace(r.Body[:index])
	}
	return strings.TrimSpace(r.Body)
}

// Writes reports whether a role asks for a tool that changes the tree.
func (r Role) Writes() bool {
	for _, tool := range r.Tools {
		if tool == "write" || tool == "edit" || tool == "shell" {
			return true
		}
	}
	return false
}

// Rules is the universal rules file with the accessibility contract filled in.
func Rules(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "komodo", "AGENTS.md"))
	if err != nil {
		return "", err
	}
	text := string(data)
	contract, err := os.ReadFile(filepath.Join(root, "komodo", "rules", "accessibility.md"))
	if err == nil {
		text = strings.ReplaceAll(text, "{{accessibility}}", strings.TrimSpace(string(contract)))
	}
	return text, nil
}

// splitList reads an inline [a, b] list into its items.
func splitList(value string) []string {
	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	if value == "" {
		return nil
	}
	var out []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
