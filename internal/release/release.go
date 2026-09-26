// Package release reads the changelog, tags the versions it names, and builds the release binaries.
package release

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"komodo/internal/changelog"
	"komodo/internal/gate"
)

// Targets are the platforms a release ships a binary for.
var Targets = []gate.Target{
	{Name: "komodo-darwin-arm64", GOOS: "darwin", Arch: "arm64"},
	{Name: "komodo-windows-amd64.exe", GOOS: "windows", Arch: "amd64"},
	{Name: "komodo-linux-amd64", GOOS: "linux", Arch: "amd64"},
}

// BuildAssets builds every release target into dir and returns the paths it wrote.
func BuildAssets(root, dir string, out io.Writer) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var paths []string
	for _, target := range Targets {
		path, err := gate.Build(root, dir, target)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(out, "built %s\n", target.Name)
		paths = append(paths, path)
	}
	return paths, nil
}

// semver matches x.y.z with an optional prerelease such as -alpha.1.
const semver = `\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?`

var headingRe = regexp.MustCompile(`(?m)^##\s+\[?v?(` + semver + `)\]?`)

// Version is one changelog heading and the lines under it.
type Version struct {
	Number string
	Body   string
}

// Versions lists every version the changelog names, newest first as the file orders them.
func Versions(text string) []Version {
	matches := headingRe.FindAllStringSubmatchIndex(text, -1)
	var out []Version
	for index, match := range matches {
		end := len(text)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		out = append(out, Version{
			Number: text[match[2]:match[3]],
			Body:   strings.TrimSpace(text[match[1]:end]),
		})
	}
	return out
}

// Latest is the highest version the changelog names.
func Latest(text string) string {
	versions := Versions(text)
	if len(versions) == 0 {
		return ""
	}
	numbers := make([]string, 0, len(versions))
	for _, version := range versions {
		numbers = append(numbers, version.Number)
	}
	sort.Slice(numbers, func(i, j int) bool { return Compare(numbers[i], numbers[j]) > 0 })
	return numbers[0]
}

// Compare orders two semantic versions, returning -1, 0, or 1; a prerelease sorts before its release.
func Compare(left, right string) int { return changelog.Compare(left, right) }

// Taggable lists the versions the changelog names that no tag points at.
func Taggable(text string, tags []string) []string {
	has := map[string]bool{}
	for _, tag := range tags {
		has[strings.TrimPrefix(tag, "v")] = true
	}
	var out []string
	for _, version := range Versions(text) {
		if !has[version.Number] && !contains(out, version.Number) {
			out = append(out, version.Number)
		}
	}
	sort.Slice(out, func(i, j int) bool { return Compare(out[i], out[j]) < 0 })
	return out
}

// Unreleased lists the untagged versions newer than every version tag, oldest first; an older gap is history.
func Unreleased(text string, tags []string) []string {
	newest := ""
	for _, tag := range tags {
		if number := strings.TrimPrefix(tag, "v"); versionTag.MatchString(tag) && (newest == "" || Compare(number, newest) > 0) {
			newest = number
		}
	}
	var out []string
	for _, version := range Taggable(text, tags) {
		if newest == "" || Compare(version, newest) > 0 {
			out = append(out, version)
		}
	}
	return out
}

// Drift is one disagreement between the changelog, the tags, and BACKLOG.md.
type Drift struct {
	Subject string `json:"subject"`
	Detail  string `json:"detail"`
}

var versionTag = regexp.MustCompile(`^v?` + semver + `$`)

// Check audits the changelog against the tags and the versions each group declares.
func Check(changelog string, tags, groupVersions []string) []Drift {
	var drift []Drift
	named := map[string]bool{}
	counts := map[string]int{}
	versions := Versions(changelog)
	for index, version := range versions {
		named[version.Number] = true
		if version.Body == "" {
			drift = append(drift, Drift{version.Number, "the changelog heading has no body"})
		}
		counts[version.Number]++
		if counts[version.Number] == 2 {
			drift = append(drift, Drift{version.Number, "more than one heading names this version"})
		}
		if index > 0 && Compare(version.Number, versions[index-1].Number) > 0 {
			drift = append(drift, Drift{version.Number, "this heading is out of order, newer than the heading above it"})
		}
	}
	for _, tag := range tags {
		if !versionTag.MatchString(tag) {
			continue
		}
		if number := strings.TrimPrefix(tag, "v"); !named[number] {
			drift = append(drift, Drift{tag, "a tag no changelog heading names"})
		}
	}
	seen := map[string]bool{}
	for _, version := range groupVersions {
		if version == "" || named[version] || seen[version] {
			continue
		}
		seen[version] = true
		drift = append(drift, Drift{version, "a group ships this version and the changelog does not name it"})
	}
	return drift
}

// ReadChangelog reads a changelog file with the fragments beside it folded in, or the empty string when there is none.
func ReadChangelog(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	fragments, err := changelog.Fragments(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return changelog.Fold(string(data), fragments), nil
}

// TagName is the annotated tag name for one version.
func TagName(version string) string { return "v" + version }

// TagMessage is the annotation one release tag carries.
func TagMessage(version string) string { return fmt.Sprintf("release %s", version) }

// contains reports whether the slice holds the value.
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
