package ingest

import (
	"path/filepath"
	"sort"
	"strings"

	"komodo/internal/backlog"
)

// allChecks combines hand-written checks with per-language derived checks, adding derived checks
// alongside hand-written ones without replacing them, deduped and in order.
func allChecks(group backlog.Group, files []string) []string {
	handWritten := handWrittenChecks(group)
	derived := derivedChecks(files)

	seen := map[string]bool{}
	var out []string
	for _, check := range handWritten {
		seen[check] = true
		out = append(out, check)
	}
	for _, check := range derived {
		if !seen[check] {
			seen[check] = true
			out = append(out, check)
		}
	}
	return out
}

// derivedChecks generates per-language checks from a card's files: Go build/vet/test per package,
// and TypeScript type check and test script.
func derivedChecks(files []string) []string {
	goPackages := extractGoPackages(files)
	hasTS := hasTypeScriptFiles(files)

	var checks []string

	for _, pkg := range goPackages {
		pattern := goPattern(pkg)
		checks = append(checks, "go build "+pattern)
		checks = append(checks, "go vet "+pattern)
		checks = append(checks, "go test "+pattern)
	}

	if hasTS {
		checks = append(checks, "tsc --noEmit")
		checks = append(checks, "npm test")
	}

	return checks
}

// goPattern is the go command target for a package directory, the repo root a bare dot.
func goPattern(pkg string) string {
	if pkg == "." {
		return "."
	}
	return "./" + pkg + "/..."
}

// extractGoPackages returns the distinct directories containing Go files, including the repo
// root, sorted.
func extractGoPackages(files []string) []string {
	pkgs := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, ".go") {
			pkgs[filepath.Dir(file)] = true
		}
	}
	var out []string
	for pkg := range pkgs {
		out = append(out, pkg)
	}
	sort.Strings(out)
	return out
}

// hasTypeScriptFiles reports whether files contains any TypeScript source files.
func hasTypeScriptFiles(files []string) bool {
	for _, file := range files {
		if strings.HasSuffix(file, ".ts") || strings.HasSuffix(file, ".tsx") {
			return true
		}
	}
	return false
}
