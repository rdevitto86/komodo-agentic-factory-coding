// Package changelog reads CHANGELOG.md, which only a release writes, and orders the versions it names.
package changelog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// File is the changelog a release writes and every reader sees.
const File = "CHANGELOG.md"

// Read returns root's CHANGELOG.md, or the empty string when it has none.
func Read(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, File))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return string(data), nil
}

// SemVer matches x.y.z with an optional prerelease such as -alpha.1.
const SemVer = `\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?`

// Heading matches a whole version heading line, an "Unreleased —" prefix allowed; its first group is the version.
var Heading = regexp.MustCompile(`(?m)^##\s+(?:Unreleased\s+—\s+)?\[?v?(` + SemVer + `)\]?.*$`)

// versionHeading is the heading pattern this package reads.
var versionHeading = Heading

// validVersion is one whole version, with an optional leading v.
var validVersion = regexp.MustCompile(`^v?` + SemVer + `$`)

// Valid reports whether version is a whole semantic version; Compare sorts anything else first.
func Valid(version string) bool { return validVersion.MatchString(version) }

// unreleasedHeading matches a heading for the version in progress, which no installer or tag can point at yet.
var unreleasedHeading = regexp.MustCompile(`^##\s+Unreleased\s`)

// InProgress reports whether a heading line names the version still marked Unreleased.
func InProgress(heading string) bool { return unreleasedHeading.MatchString(heading) }

// Latest is the highest released version text names, or the empty string when it names none.
func Latest(text string) string {
	latest := ""
	for _, match := range versionHeading.FindAllStringSubmatch(text, -1) {
		if unreleasedHeading.MatchString(match[0]) {
			continue
		}
		if latest == "" || Compare(match[1], latest) > 0 {
			latest = match[1]
		}
	}
	return latest
}

// Section renders one version's changelog section: its dated heading, the summary, then a bullet per item.
func Section(version, date, summary string, items []string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "## %s — %s\n\n", version, date)
	if summary != "" {
		out.WriteString(summary + "\n\n")
	}
	for _, item := range items {
		out.WriteString("- " + item + "\n")
	}
	return out.String()
}

// FirstSentence is text up to and including its first full stop, or the whole text trimmed.
func FirstSentence(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if end := strings.Index(text, ". "); end >= 0 {
		return text[:end+1]
	}
	return text
}

// Insert puts section above text's first version heading, or at its end with none; a version already
// named leaves text unchanged.
func Insert(text, version, section string) string {
	for _, match := range versionHeading.FindAllStringSubmatch(text, -1) {
		if match[1] == version {
			return text
		}
	}
	if loc := versionHeading.FindStringIndex(text); loc != nil {
		return text[:loc[0]] + section + "\n" + text[loc[0]:]
	}
	if text != "" && !strings.HasSuffix(text, "\n\n") {
		text = strings.TrimRight(text, "\n") + "\n\n"
	}
	return text + section
}

// Compare orders two semantic versions, returning -1, 0, or 1; a prerelease sorts before its release,
// and a malformed version sorts before every valid one.
func Compare(left, right string) int {
	leftCore, leftPre, _ := strings.Cut(left, "-")
	rightCore, rightPre, _ := strings.Cut(right, "-")
	a, leftOK := parts(leftCore)
	b, rightOK := parts(rightCore)
	switch {
	case !leftOK && !rightOK:
		return strings.Compare(left, right)
	case !leftOK:
		return -1
	case !rightOK:
		return 1
	}
	for index := 0; index < 3; index++ {
		if a[index] != b[index] {
			if a[index] < b[index] {
				return -1
			}
			return 1
		}
	}
	switch {
	case leftPre == rightPre:
		return 0
	case leftPre == "":
		return 1
	case rightPre == "":
		return -1
	}
	return comparePrerelease(leftPre, rightPre)
}

// comparePrerelease orders two prerelease strings field by field, numbers numerically.
func comparePrerelease(left, right string) int {
	a, b := strings.Split(left, "."), strings.Split(right, ".")
	for index := 0; index < len(a) && index < len(b); index++ {
		x, xErr := strconv.Atoi(a[index])
		y, yErr := strconv.Atoi(b[index])
		switch {
		case xErr == nil && yErr == nil && x != y:
			if x < y {
				return -1
			}
			return 1
		case (xErr == nil) != (yErr == nil):
			if xErr == nil {
				return -1
			}
			return 1
		case a[index] != b[index]:
			return strings.Compare(a[index], b[index])
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

// parts splits a version core into its three numbers, reporting false unless it is exactly three of them.
func parts(version string) ([3]int, bool) {
	var out [3]int
	fields := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(fields) != 3 {
		return out, false
	}
	for index, field := range fields {
		number, err := strconv.Atoi(field)
		if err != nil || field[0] == '+' {
			return [3]int{}, false
		}
		out[index] = number
	}
	return out, true
}
