package repo

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// StandardsDir is where a repo appends to a shipped standard or adds a new one.
const StandardsDir = ".komodo/standards"

// StandardOverride is one repo standard: appended text for a shipped one, or a new one with its own globs.
type StandardOverride struct {
	Name string
	New  bool
	Body string
}

// LoadStandards reads every repo standard override the repo declares.
func LoadStandards(root string) ([]StandardOverride, []string) {
	entries, err := os.ReadDir(filepath.Join(root, StandardsDir))
	if err != nil {
		return nil, nil
	}
	var out []StandardOverride
	var skipped []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, StandardsDir, entry.Name()))
		if err != nil {
			skipped = append(skipped, entry.Name()+": "+err.Error())
			continue
		}
		text := normalizeNewlines(data)
		name := strings.TrimSuffix(entry.Name(), ".md")
		out = append(out, StandardOverride{Name: name, New: frontmatter.MatchString(text), Body: strings.TrimSpace(text)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, skipped
}
