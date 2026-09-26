package claude

import (
	"sort"
	"strings"

	"komodo/internal/detect"
	"komodo/internal/mount"
)

// BuilderPluginSkills returns the skills that should be in the builder's plugin: the build skill
// and standards for the languages the group touches, omitting review, planning and orchestrator skills.
func BuilderPluginSkills(detected detect.Profile, skills []mount.Skill) []string {
	var out []string

	// The builder always gets the build skill if it exists.
	out = append(out, "build")

	// Build a map of available skills for lookup.
	availableSkills := map[string]bool{}
	for _, skill := range skills {
		availableSkills[skill.Name] = true
	}

	// Add standards for each detected language.
	langToStandard := map[string]string{
		"Go":         "standards-go",
		"Python":     "standards-python",
		"TypeScript": "standards-typescript",
		"JavaScript": "standards-javascript",
		"Rust":       "standards-rust",
		"Java":       "standards-java",
		"C#":         "standards-csharp",
		"Ruby":       "standards-ruby",
		"PHP":        "standards-php",
		"C":          "standards-c",
		"C++":        "standards-cpp",
	}

	// Collect standards for languages the group touches, only if they are available.
	seen := map[string]bool{"build": true}
	for _, lang := range detected.Languages {
		if standard, ok := langToStandard[lang]; ok && availableSkills[standard] && !seen[standard] {
			out = append(out, standard)
			seen[standard] = true
		}
	}

	// Also add standards that are forced by repo overrides.
	for _, skill := range skills {
		if strings.HasPrefix(skill.Name, "standards-") && !seen[skill.Name] && isForcedStandard(skill.Name) {
			out = append(out, skill.Name)
			seen[skill.Name] = true
		}
	}

	sort.Strings(out)
	return out
}

// isForcedStandard reports whether a standard is forced by a repo override; this is a placeholder
// that assumes a standard is forced if it exists in the repo's own standards.
func isForcedStandard(name string) bool {
	// No repo override source exists yet.
	return false
}
