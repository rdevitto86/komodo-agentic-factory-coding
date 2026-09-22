// Package comments is the mechanical comment lint the output device and the gate both run.
package comments

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Comment families, one per marker syntax.
const (
	familyC    = "c"
	familyHash = "hash"
	familyDash = "dash"
)

// Caps a comment may not exceed.
const (
	MaxWords         = 20
	MaxChars         = 140
	funcBlockMaxLine = 2
	blockMaxLines    = 1
	headerMaxLines   = 30
	trivialLines     = 8
)

var extensionFamily = map[string]string{
	".c": familyC, ".cc": familyC, ".cpp": familyC, ".cs": familyC, ".dart": familyC,
	".go": familyC, ".h": familyC, ".hpp": familyC, ".java": familyC, ".js": familyC,
	".jsx": familyC, ".kt": familyC, ".rs": familyC, ".swift": familyC, ".ts": familyC,
	".tsx": familyC, ".vue": familyC, ".svelte": familyC, ".mjs": familyC, ".cjs": familyC,
	".sh": familyHash, ".bash": familyHash, ".zsh": familyHash, ".py": familyHash, ".pyi": familyHash,
	".rb": familyHash, ".yaml": familyHash, ".yml": familyHash, ".toml": familyHash, ".tf": familyHash,
	".sql": familyDash, ".lua": familyDash,
}

var filenameFamily = map[string]string{"Dockerfile": familyHash, "Makefile": familyHash, "Justfile": familyHash}

var lineMarker = map[string]string{familyC: "//", familyHash: "#", familyDash: "--"}

var quotes = map[string]string{familyC: "\"'`", familyHash: "\"'", familyDash: "'\""}

var exemptPrefixes = []string{
	"!", "+build", "-*- coding", "biome-ignore", "cgo", "clang-format", "eslint-disable", "eslint-enable",
	"fmt:", "go:", "golangci", "isort:", "istanbul ignore", "lint:", "mypy:", "nolint", "noqa", "nosec",
	"pragma", "prettier-ignore", "pylint:", "pyright:", "ruff:", "shellcheck", "ts-expect-error", "ts-ignore",
	"ts-nocheck", "type:", "use client", "use server", "@ts-", "eslint", "region", "endregion",
}

type namedPattern struct {
	pattern *regexp.Regexp
	subject string
}

var externalRefs = []namedPattern{
	{regexp.MustCompile(`\bv\d+\.\d+`), "a version number"},
	{regexp.MustCompile(`\b\d+\.\d+\.\d+\b`), "a version number"},
	{regexp.MustCompile(`\b(?:PRD|SDD|ADR|TSK|EPIC|JIRA|TG)-?\d*\b`), "a spec or ticket reference"},
	{regexp.MustCompile(`(?i)\bper (?:the |our )?(?:spec|prd|sdd|design|ticket|backlog|story)`), "a document reference"},
	{regexp.MustCompile(`(?i)\bthe (?:spec|design doc|backlog|ticket|story)\b`), "a document reference"},
	{regexp.MustCompile(`(?i)\bas (?:discussed|requested|agreed)\b`), "session context"},
	{regexp.MustCompile(`(?i)\bthis (?:band|task|PR|story|sprint|session)\b`), "session context"},
	{regexp.MustCompile(`(?i)\bthe user (?:asked|wants|requested|said)\b`), "session context"},
}

var narrativePatterns = []namedPattern{
	{regexp.MustCompile(`(?i)(?:(?:^|\s)(?:we're|we've|we|let's|our)\b|(?:^|\s)i(?:[\s']|$))`), "first person"},
	{regexp.MustCompile(`(?i)\b(?:probably|maybe|i think|should work|hopefully|seems? to|might be|kind of|sort of)\b`), "a hedge"},
	{regexp.MustCompile(`(?i)\b(?:previously|used to|no longer|now uses|was changed|refactored|moved from|instead of the old)\b`), "history"},
}

var (
	funcDecl = regexp.MustCompile(`^(?:(?:export|default|public|private|protected|internal|static|final|abstract|async|inline|open|override|suspend|pub)(?:\([^)]*\))?\s+)*` +
		`(?:func|function|def|fn)\s+(?:\([^)]*\)\s*)?(\w+)`)
	arrowDecl    = regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?(?:const|let|var)\s+(\w+)\s*(?::[^=]+)?=\s*(?:async\s+)?(?:\([^)]*\)|\w+)\s*=>`)
	goMethod     = regexp.MustCompile(`^func\s*\([^)]*\)\s*(\w+)`)
	goFunc       = regexp.MustCompile(`^func\s+(\w+)`)
	markerShape  = regexp.MustCompile(`^(NOTE|FIXME|TODO):\s+\S`)
	annotation   = regexp.MustCompile(`^(?:@\w+(?:\(.*\))?|#!?\[.*\])$`)
	dunder       = regexp.MustCompile(`^__\w+__$`)
	testFileName = []*regexp.Regexp{
		regexp.MustCompile(`_test\.[^.]+$`), regexp.MustCompile(`^test_`),
		regexp.MustCompile(`\.(?:test|spec)\.[^.]+$`), regexp.MustCompile(`^conftest\.`),
	}
)

var (
	nonDecl        = []string{"if", "for", "while", "switch", "catch", "else", "do", "try", "select", "case", "return", "new"}
	testDirs       = []string{"test", "tests", "__tests__", "spec", "specs", "testdata"}
	generatedDirs  = []string{"vendor", "node_modules", "generated", "third_party", "dist", "build"}
	generatedSuffs = []string{".gen.go", ".pb.go", "_pb2.py", ".generated.ts", ".d.ts", ".min.js"}
	markers        = []string{"NOTE", "FIXME", "TODO"}
)

// Finding is one comment breaking one mechanical rule.
type Finding struct {
	Line   int    `json:"line"`
	Rule   string `json:"rule"`
	Detail string `json:"detail"`
}

// Undocumented is one declaration that owes a comment and has none.
type Undocumented struct {
	Line int    `json:"line"`
	Name string `json:"name"`
}

// ResolveFamily returns the comment family for a path, or the empty string when the lint has no opinion.
func ResolveFamily(path string) string {
	base := filepath.Base(path)
	if family, ok := filenameFamily[base]; ok {
		return family
	}
	return extensionFamily[strings.ToLower(filepath.Ext(base))]
}

// IsTestPath reports whether a path is a test file or sits under a test directory.
func IsTestPath(path string) bool {
	base := filepath.Base(path)
	for _, pattern := range testFileName {
		if pattern.MatchString(base) {
			return true
		}
	}
	parts := strings.Split(strings.ReplaceAll(filepath.Dir(path), "\\", "/"), "/")
	for _, part := range parts {
		if has(testDirs, strings.ToLower(part)) {
			return true
		}
	}
	return false
}

// IsGenerated reports whether a file is machine-written, by header, suffix, or directory.
func IsGenerated(lines []string, path string) bool {
	for index, line := range lines {
		if index >= 5 {
			break
		}
		lowered := strings.ToLower(line)
		if strings.Contains(lowered, "do not edit") || strings.Contains(lowered, "@generated") {
			return true
		}
	}
	base := strings.ToLower(filepath.Base(path))
	for _, suffix := range generatedSuffs {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	for _, part := range strings.Split(strings.ReplaceAll(filepath.Dir(path), "\\", "/"), "/") {
		if has(generatedDirs, strings.ToLower(part)) {
			return true
		}
	}
	return false
}

// CommentStart returns where a comment begins on a line, ignoring markers inside quotes, or -1.
func CommentStart(line, family string) int {
	marker := lineMarker[family]
	quoteSet := quotes[family]
	var active byte
	for index := 0; index < len(line); index++ {
		char := line[index]
		switch {
		case active != 0:
			if char == '\\' {
				index++
				continue
			}
			if char == active {
				active = 0
			}
		case strings.IndexByte(quoteSet, char) >= 0:
			active = char
		case strings.HasPrefix(line[index:], marker):
			return index
		}
	}
	return -1
}

// CommentBody strips a comment's marker and leading doc punctuation.
func CommentBody(raw, family string) string {
	text := strings.TrimSpace(raw)
	marker := lineMarker[family]
	for strings.HasPrefix(text, marker) {
		text = text[len(marker):]
	}
	return strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(text), "/!*"))
}

// IsDirective reports whether a comment body is a machine directive the lint leaves alone.
func IsDirective(body string) bool {
	if body == "" {
		return true
	}
	lowered := strings.ToLower(body)
	for _, prefix := range exemptPrefixes {
		if strings.HasPrefix(lowered, prefix) {
			return true
		}
	}
	return false
}

// ExternalReference names the first banned citation a body carries.
func ExternalReference(body string) string { return firstMatch(externalRefs, body) }

// Narrative names the first narrative tell a body carries.
func Narrative(body string) string { return firstMatch(narrativePatterns, body) }

// firstMatch returns the subject of the first pattern a body matches.
func firstMatch(patterns []namedPattern, body string) string {
	for _, item := range patterns {
		if item.pattern.MatchString(body) {
			return item.subject
		}
	}
	return ""
}

// FunctionName returns the function a line declares, or the empty string.
func FunctionName(line, ext string) string {
	stripped := strings.TrimSpace(line)
	if ext == ".go" {
		if match := goMethod.FindStringSubmatch(stripped); match != nil {
			return match[1]
		}
		if match := goFunc.FindStringSubmatch(stripped); match != nil {
			return match[1]
		}
		return ""
	}
	for _, pattern := range []*regexp.Regexp{funcDecl, arrowDecl} {
		if match := pattern.FindStringSubmatch(stripped); match != nil {
			return match[1]
		}
	}
	braced := []string{".java", ".cs", ".kt", ".c", ".cc", ".cpp", ".h", ".hpp", ".swift", ".rs", ".dart"}
	if has(braced, ext) && strings.HasSuffix(stripped, "{") && strings.Contains(stripped, "(") {
		head := strings.Fields(strings.SplitN(stripped, "(", 2)[0])
		if len(head) > 0 && !has(nonDecl, head[0]) && !hasPrefixAny(head[0], "class", "struct", "enum", "interface", "record") {
			return head[len(head)-1]
		}
	}
	return ""
}

// IsAnnotation reports whether a line is an attribute or annotation, not a comment or declaration.
func IsAnnotation(line string) bool {
	return annotation.MatchString(strings.TrimSpace(line))
}

// IsExported reports whether a declaration is public under its language's convention.
func IsExported(name, line, ext string) bool {
	stripped := strings.TrimSpace(line)
	switch ext {
	case ".go":
		return name != "" && strings.ToUpper(name[:1]) == name[:1]
	case ".py", ".pyi":
		return !strings.HasPrefix(name, "_")
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".vue", ".svelte":
		return strings.HasPrefix(stripped, "export")
	case ".rs":
		return strings.HasPrefix(stripped, "pub")
	}
	head := strings.SplitN(stripped, "(", 2)[0]
	return strings.Contains(head, "public") || strings.Contains(head, "protected")
}

// has reports whether the slice holds the value.
func has(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// hasPrefixAny reports whether text starts with any of the prefixes.
func hasPrefixAny(text string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}
