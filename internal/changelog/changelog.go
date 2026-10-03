// Package changelog reads CHANGELOG.md, which only a release writes, and orders the versions it names.
package changelog

import (
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

// versionHeading matches a version heading with or without brackets, a leading v, or a date.
var versionHeading = regexp.MustCompile(`(?m)^##\s+\[?v?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)\]?.*$`)

// Latest is the highest version text names, or the empty string when it names none.
func Latest(text string) string {
	latest := ""
	for _, match := range versionHeading.FindAllStringSubmatch(text, -1) {
		if latest == "" || Compare(match[1], latest) > 0 {
			latest = match[1]
		}
	}
	return latest
}

// Compare orders two semantic versions, returning -1, 0, or 1; a prerelease sorts before its release.
func Compare(left, right string) int {
	leftCore, leftPre, _ := strings.Cut(left, "-")
	rightCore, rightPre, _ := strings.Cut(right, "-")
	a, b := parts(leftCore), parts(rightCore)
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

// parts splits a version core into its three numbers.
func parts(version string) [3]int {
	var out [3]int
	for index, field := range strings.SplitN(version, ".", 3) {
		if index > 2 {
			break
		}
		number, err := strconv.Atoi(strings.TrimSpace(field))
		if err == nil {
			out[index] = number
		}
	}
	return out
}
