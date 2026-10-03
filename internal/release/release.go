// Package release reads the changelog, tags the versions it names, and builds the release binaries.
package release

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"komodo/internal/changelog"
	"komodo/internal/gate"
)

// Targets are the platforms a release ships a binary for.
var Targets = []gate.Target{
	{Name: "komodo-darwin-arm64", GOOS: "darwin", Arch: "arm64"},
	{Name: "komodo-darwin-amd64", GOOS: "darwin", Arch: "amd64"},
	{Name: "komodo-linux-amd64", GOOS: "linux", Arch: "amd64"},
	{Name: "komodo-linux-arm64", GOOS: "linux", Arch: "arm64"},
	{Name: "komodo-windows-amd64.exe", GOOS: "windows", Arch: "amd64"},
}

// BuildAssets builds every release target into dir and returns the paths it wrote.
func BuildAssets(root, dir string, out io.Writer) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var paths []string
	for _, target := range Targets {
		path, err := build(root, dir, target)
		if err != nil {
			return nil, err
		}
		if err := reproduces(root, path, target); err != nil {
			return nil, err
		}
		fmt.Fprintf(out, "built %s, and a second build matched it byte for byte\n", target.Name)
		paths = append(paths, path)
	}
	return paths, nil
}

// build compiles one target; tests swap it.
var build = gate.Build

// reproduces builds target a second time in a scratch dir and fails unless the binary matches path exactly.
func reproduces(root, path string, target gate.Target) error {
	scratch, err := os.MkdirTemp("", "komodo-reproduce-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	again, err := build(root, scratch, target)
	if err != nil {
		return err
	}
	first, err := gate.Sum(path)
	if err != nil {
		return err
	}
	second, err := gate.Sum(again)
	if err != nil {
		return err
	}
	if first != second {
		return fmt.Errorf("%s does not reproduce: two builds of one commit differ (%s, %s); the release stops", target.Name, first, second)
	}
	return nil
}

// Version is one changelog heading and the lines under it.
type Version struct {
	Number string
	Body   string
}

// Versions lists every version the changelog names, newest first as the file orders them.
func Versions(text string) []Version {
	matches := changelog.Heading.FindAllStringSubmatchIndex(text, -1)
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
func Latest(text string) string { return changelog.Latest(text) }

// versionShaped matches a tag meant as a version, a digit after an optional v, so a malformed one is reported.
var versionShaped = regexp.MustCompile(`^v?[0-9]`)

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
		if number := strings.TrimPrefix(tag, "v"); changelog.Valid(tag) && (newest == "" || Compare(number, newest) > 0) {
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

// Drift is one disagreement between the changelog, the tags, and each group's declared version.
type Drift struct {
	Subject string `json:"subject"`
	Detail  string `json:"detail"`
}

// Check audits the changelog against the tags and the versions each group declares.
func Check(text string, tags, groupVersions []string) []Drift {
	var drift []Drift
	named := map[string]bool{}
	counts := map[string]int{}
	versions := Versions(text)
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
		if !changelog.Valid(tag) {
			if versionShaped.MatchString(tag) {
				drift = append(drift, Drift{tag, "a version tag that is not x.y.z"})
			}
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

// ReadChangelog reads a changelog file, or the empty string when there is none.
func ReadChangelog(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return string(data), nil
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
