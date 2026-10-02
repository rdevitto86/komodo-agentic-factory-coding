package repo

import (
	"encoding/json"
	"os"
	"path/filepath"

	"komodo/internal/glob"
)

// LabelsFile is where a repo maps its paths to the scope label a pull request earns.
const LabelsFile = ".komodo/labels.json"

// ScopeRule gives a pull request Label when any changed file matches one of Paths.
type ScopeRule struct {
	Label string   `json:"label"`
	Paths []string `json:"paths"`
}

// Labels is a repo's scope mapping: rules checked in order, and the label when none matches.
type Labels struct {
	Scopes  []ScopeRule `json:"scopes"`
	Default string      `json:"default"`
}

// LoadLabels reads a repo's labels file; found is false when it is absent or unreadable.
func LoadLabels(root string) (labels Labels, found bool) {
	data, err := os.ReadFile(filepath.Join(root, LabelsFile))
	if err != nil || json.Unmarshal(data, &labels) != nil {
		return Labels{}, false
	}
	return labels, true
}

// Scope is the first rule's label whose paths match a changed file, else the default.
func (l Labels) Scope(files []string) string {
	for _, rule := range l.Scopes {
		for _, pattern := range rule.Paths {
			for _, file := range files {
				if glob.Match(pattern, file) {
					return rule.Label
				}
			}
		}
	}
	return l.Default
}
