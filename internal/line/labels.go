package line

import (
	"strings"

	"komodo/internal/changelog"
	repopkg "komodo/internal/repo"
)

// toolkitScopes are the scope rules a repo gets when it maps none of its own in repopkg.LabelsFile.
var toolkitScopes = repopkg.Labels{
	Scopes: []repopkg.ScopeRule{
		{Label: "scope/guard", Paths: []string{"internal/guard/**"}},
		{Label: "scope/mount", Paths: []string{"internal/mount/**"}},
		{Label: "scope/skills", Paths: []string{"komodo/skills/**", "komodo/roles/**"}},
		{Label: "scope/agents", Paths: []string{"internal/profile/**"}},
	},
	Default: "scope/harness",
}

// scopeLabel is the one scope label files earn under the toolkit's own rules.
func scopeLabel(files []string) string {
	return toolkitScopes.Scope(files)
}

// ScopeLabel is the one scope label files earn under the repo's labels file, else the toolkit's rules.
func ScopeLabel(root string, files []string) string {
	if labels, found := repopkg.LoadLabels(root); found {
		return labels.Scope(files)
	}
	return scopeLabel(files)
}

// OptionalLabels are the stage of the repo's newest changelog version, and branch/feature when base is not the default branch.
func OptionalLabels(root, base string) []string {
	var out []string
	if text, err := changelog.Read(root); err == nil {
		if stage := StageLabel(changelog.Latest(text)); stage != "" {
			out = append(out, stage)
		}
	}
	if base != "" && base != DefaultBase(root) {
		out = append(out, "branch/feature")
	}
	return out
}

// StageLabel is the stage label a version's phase earns: alpha, beta, rc, or ga for a stable version.
func StageLabel(version string) string {
	if version == "" {
		return ""
	}
	_, pre, _ := strings.Cut(version, "-")
	for _, phase := range []string{"alpha", "beta", "rc"} {
		if strings.HasPrefix(pre, phase) {
			return "stage/" + phase
		}
	}
	if pre != "" {
		return ""
	}
	return "stage/ga"
}
