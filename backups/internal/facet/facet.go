// Package facet reads Komodo's setup skills and role appendices, keyed by detection.
package facet

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"komodo/internal/detect"
	"komodo/internal/toolkit"
)

// FacetsDir is where the shipped facets live, relative to the repo root.
const FacetsDir = "komodo/facets"

// OverrideFile lists facets detection missed, one name per line; it can add, never remove.
const OverrideFile = ".komodo/facets"

// Detect is the markers that select a facet, matched against a detect.Profile.
type Detect struct {
	Cloud []string `json:"cloud"`
	Data  []string `json:"data"`
	CI    []string `json:"ci"`
}

// Facet is one platform's setup skill, role appendices, and detection markers, keyed by name.
type Facet struct {
	Name    string
	Skill   string
	FacetMD string
	Detect  Detect
}

// builderHeading and reviewerHeading find a facet's appendix under its own heading.
var builderHeading = regexp.MustCompile(`(?is)##\s*Builder appendix\s*\n(.*?)(\n##\s|\z)`)
var reviewerHeading = regexp.MustCompile(`(?is)##\s*Reviewer appendix\s*\n(.*?)(\n##\s|\z)`)

// BuilderAppendix returns the text under a facet's "Builder appendix" heading.
func (f Facet) BuilderAppendix() string {
	return heading(builderHeading, f.FacetMD)
}

// ReviewerAppendix returns the text under a facet's "Reviewer appendix" heading.
func (f Facet) ReviewerAppendix() string {
	return heading(reviewerHeading, f.FacetMD)
}

// heading returns the trimmed text a heading pattern captures, or empty when it is absent.
func heading(pattern *regexp.Regexp, body string) string {
	match := pattern.FindStringSubmatch(body)
	if match == nil {
		return ""
	}
	return strings.TrimSpace(match[1])
}

// matches reports whether a facet's markers overlap the profile detection found.
func (d Detect) matches(profile detect.Profile) bool {
	return overlaps(d.Cloud, profile.Cloud) || overlaps(d.Data, profile.Data) || overlaps(d.CI, profile.CI)
}

// overlaps reports whether any value in want appears in have.
func overlaps(want, have []string) bool {
	for _, value := range want {
		for _, candidate := range have {
			if value == candidate {
				return true
			}
		}
	}
	return false
}

// ValidName reports whether a facet name is a single plain path segment, safe to join under a root.
func ValidName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	return !strings.ContainsAny(name, `/\`)
}

// Load reads one shipped facet by name.
func Load(root, name string) (Facet, error) {
	if !ValidName(name) {
		return Facet{}, fmt.Errorf("facet: %q is not a valid facet name", name)
	}
	tree := toolkit.FS(root)
	dir := path.Join("facets", name)

	facetMD, err := fs.ReadFile(tree, path.Join(dir, "facet.md"))
	if err != nil {
		return Facet{}, err
	}
	skill, err := fs.ReadFile(tree, path.Join(dir, "skill", "SKILL.md"))
	if err != nil {
		return Facet{}, err
	}

	var detectMarkers Detect
	if data, err := fs.ReadFile(tree, path.Join(dir, "detect.json")); err == nil {
		_ = json.Unmarshal(data, &detectMarkers)
	}

	return Facet{
		Name:    name,
		Skill:   strings.TrimSpace(string(skill)),
		FacetMD: strings.TrimSpace(string(facetMD)),
		Detect:  detectMarkers,
	}, nil
}

// LoadAll reads every shipped facet, sorted by name.
func LoadAll(root string) ([]Facet, error) {
	entries, err := fs.ReadDir(toolkit.FS(root), "facets")
	if err != nil {
		return nil, err
	}
	var facets []Facet
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		facet, err := Load(root, entry.Name())
		if err != nil {
			return nil, err
		}
		facets = append(facets, facet)
	}
	sort.Slice(facets, func(i, j int) bool { return facets[i].Name < facets[j].Name })
	return facets, nil
}

// overrides reads the facet names a repo adds beyond what detection found.
func overrides(root string) []string {
	data, err := os.ReadFile(filepath.Join(root, OverrideFile))
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		names = append(names, line)
	}
	return names
}

// Select names the facets a repo carries: detection first, then the repo's own additions, then a task's.
func Select(root string, profile detect.Profile, taskFacets []string) ([]string, error) {
	facets, err := LoadAll(root)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var selected []string
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			selected = append(selected, name)
		}
	}

	for _, facet := range facets {
		if facet.Detect.matches(profile) {
			add(facet.Name)
		}
	}
	for _, name := range overrides(root) {
		add(name)
	}
	for _, name := range taskFacets {
		add(name)
	}
	return selected, nil
}
