// Package glob matches a repo-declared path pattern against a file path.
package glob

import (
	"path"
	"strings"
)

// Match reports whether target matches pattern: dir/** as a path prefix, path.Match otherwise.
func Match(pattern, target string) bool {
	target = strings.ReplaceAll(target, "\\", "/")
	return matchSegments(strings.Split(pattern, "/"), strings.Split(target, "/"))
}

// matchSegments matches pattern segments against path segments; "**" spans zero or more.
func matchSegments(pattern, target []string) bool {
	if len(pattern) == 0 {
		return len(target) == 0
	}
	if pattern[0] == "**" {
		if matchSegments(pattern[1:], target) {
			return true
		}
		if len(target) == 0 {
			return false
		}
		return matchSegments(pattern, target[1:])
	}
	if len(target) == 0 {
		return false
	}
	ok, err := path.Match(pattern[0], target[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(pattern[1:], target[1:])
}
