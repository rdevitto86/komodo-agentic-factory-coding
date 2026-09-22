package line

import (
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"komodo/internal/toolkit"
)

// Role is one role file's frontmatter: what a machine is at a station or in a session.
type Role struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tier        string   `json:"tier"`
	Tools       []string `json:"tools"`
	Session     bool     `json:"session"`
	Returns     string   `json:"returns,omitempty"`
	Machine     string   `json:"machine,omitempty"`
	Body        string   `json:"-"`
}

var frontmatter = regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n(.*)\z`)

// RolesDir is where the shipped role files live, relative to the repo root.
const RolesDir = "komodo/roles"

// LoadRole reads one role file into its frontmatter and body.
func LoadRole(root, name string) (Role, error) {
	data, err := fs.ReadFile(toolkit.FS(root), path.Join("roles", name+".md"))
	if err != nil {
		return Role{}, err
	}
	return parseRole(string(data))
}

// LoadRoles reads every shipped role, sorted by name.
func LoadRoles(root string) ([]Role, error) {
	entries, err := fs.ReadDir(toolkit.FS(root), "roles")
	if err != nil {
		return nil, err
	}
	var roles []Role
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		role, err := LoadRole(root, strings.TrimSuffix(entry.Name(), ".md"))
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].Name < roles[j].Name })
	return roles, nil
}

// parseRole reads the frontmatter keys a role declares.
func parseRole(text string) (Role, error) {
	match := frontmatter.FindStringSubmatch(text)
	if match == nil {
		return Role{}, errNoFrontmatter
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
	return role, nil
}

// splitList reads an inline [a, b] list into its items.
func splitList(value string) []string {
	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	if value == "" {
		return []string{}
	}
	items := strings.Split(value, ",")
	out := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// errNoFrontmatter is returned when a role file opens with no --- block.
var errNoFrontmatter = errorString("role file has no frontmatter")

// errorString is a constant error the package returns without allocating.
type errorString string

// Error returns the message.
func (e errorString) Error() string { return string(e) }
