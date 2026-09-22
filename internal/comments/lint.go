package comments

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// maskedLines blanks the interior of multi-line strings so a # inside a docstring is not a comment.
func maskedLines(text, ext string) []string {
	lines := strings.Split(text, "\n")
	if ext != ".py" && ext != ".pyi" {
		return lines
	}
	out := make([]string, 0, len(lines))
	fence := ""
	for _, line := range lines {
		if fence != "" {
			close := strings.Index(line, fence)
			if close == -1 {
				out = append(out, "")
				continue
			}
			line = strings.Repeat(" ", close+3) + line[close+3:]
			fence = ""
		}
		kept := line
		cursor := 0
		for {
			start, opener := -1, ""
			for _, candidate := range []string{`"""`, "'''"} {
				at := strings.Index(kept[cursor:], candidate)
				if at == -1 {
					continue
				}
				at += cursor
				if start == -1 || at < start {
					start, opener = at, candidate
				}
			}
			if start == -1 {
				break
			}
			hashAt := strings.Index(kept[cursor:], "#")
			if hashAt != -1 {
				hashAt += cursor
			}
			if hashAt != -1 && hashAt < start {
				break
			}
			close := strings.Index(kept[start+3:], opener)
			if close == -1 {
				fence = opener
				kept = kept[:start]
				break
			}
			close += start + 3
			kept = kept[:start] + strings.Repeat(" ", close+3-start) + kept[close+3:]
			cursor = close + 3
		}
		out = append(out, kept)
	}
	return out
}

// bodySpan counts statement lines and returns inside a declaration's body, read from indentation.
func bodySpan(lines []string, index int, family string) (int, int) {
	indent := len(lines[index]) - len(strings.TrimLeft(lines[index], " \t"))
	marker := lineMarker[family]
	statements, returns := 0, 0
	for _, line := range lines[index+1:] {
		stripped := strings.TrimSpace(line)
		if stripped == "" {
			continue
		}
		if len(line)-len(strings.TrimLeft(line, " \t")) <= indent && !hasPrefixAny(stripped, "}", ")", "]") {
			break
		}
		if strings.HasPrefix(stripped, marker) || stripped == "}" || stripped == "})" || stripped == "};" {
			continue
		}
		statements++
		if strings.HasPrefix(stripped, "return") {
			returns++
		}
	}
	return statements, returns
}

// documentedAbove reports whether the previous non-blank line is a comment.
func documentedAbove(lines []string, index int, family string) bool {
	marker := lineMarker[family]
	look := index - 1
	for look >= 0 && strings.TrimSpace(lines[look]) == "" {
		look--
	}
	if look < 0 {
		return false
	}
	previous := strings.TrimSpace(lines[look])
	return strings.HasPrefix(previous, marker) || strings.HasSuffix(previous, "*/") ||
		hasPrefixAny(previous, "/**", "/*", "///")
}

// hasDocstring reports whether a declaration's first body line opens a Python docstring.
func hasDocstring(lines []string, index int) bool {
	for _, line := range lines[index+1:] {
		stripped := strings.TrimSpace(line)
		if stripped == "" {
			continue
		}
		return hasPrefixAny(stripped, `"""`, "'''", `r"""`, "r'''")
	}
	return false
}

// UndocumentedFunctions lists declarations that owe a comment under the requirement and lack one.
func UndocumentedFunctions(text, path, require string) []Undocumented {
	if require == "none" {
		return nil
	}
	family := ResolveFamily(path)
	if family == "" || IsTestPath(path) {
		return nil
	}
	ext := strings.ToLower(filepath.Ext(path))
	raw := strings.Split(text, "\n")
	if IsGenerated(raw, path) {
		return nil
	}
	lines := maskedLines(text, ext)
	var found []Undocumented
	for index, line := range lines {
		name := FunctionName(line, ext)
		if name == "" || dunder.MatchString(name) {
			continue
		}
		statements, returns := bodySpan(lines, index, family)
		if statements == 0 {
			continue
		}
		exported := IsExported(name, line, ext)
		nonobvious := statements > trivialLines || returns > 1
		if require == "exported" && !exported {
			continue
		}
		if require == "nonobvious" && !exported && !nonobvious {
			continue
		}
		if (ext == ".py" || ext == ".pyi") && hasDocstring(raw, index) {
			continue
		}
		if documentedAbove(raw, index, family) {
			continue
		}
		found = append(found, Undocumented{Line: index + 1, Name: name})
	}
	return found
}

// InvalidComments returns every comment breaking a mechanical rule.
func InvalidComments(text, path string) []Finding {
	family := ResolveFamily(path)
	if family == "" {
		return nil
	}
	ext := strings.ToLower(filepath.Ext(path))
	marker := lineMarker[family]
	lines := maskedLines(text, ext)
	var findings []Finding

	index := 0
	if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		index = 1
	}
	for index < len(lines) && index < headerMaxLines && strings.HasPrefix(strings.TrimSpace(lines[index]), marker) {
		index++
	}
	headerEnd := index

	type run struct{ start, end int }
	var runs []run
	index = 0
	for index < len(lines) {
		if strings.HasPrefix(strings.TrimSpace(lines[index]), marker) {
			start := index
			for index < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[index]), marker) {
				index++
			}
			runs = append(runs, run{start, index})
			continue
		}
		index++
	}

	previousEnd := -1
	for _, block := range runs {
		if previousEnd >= 0 && block.end > headerEnd {
			blank := true
			for number := previousEnd; number < block.start; number++ {
				if strings.TrimSpace(lines[number]) != "" {
					blank = false
					break
				}
			}
			if blank {
				findings = append(findings, Finding{block.start + 1, "STACKED",
					"two comment blocks with no code between them; merge or drop one"})
			}
		}
		previousEnd = block.end
		if block.end <= headerEnd {
			continue
		}
		var substantive []int
		for number := block.start; number < block.end; number++ {
			if !IsDirective(CommentBody(lines[number], family)) {
				substantive = append(substantive, number)
			}
		}
		target := ""
		if block.end < len(lines) {
			target = lines[block.end]
		}
		cap := blockMaxLines
		where := "above a statement"
		if FunctionName(target, ext) != "" || funcDecl.MatchString(strings.TrimSpace(target)) {
			cap, where = funcBlockMaxLine, "above a function"
		}
		if len(substantive) > cap {
			plural := "s"
			if cap == 1 {
				plural = ""
			}
			findings = append(findings, Finding{substantive[cap] + 1, "OVER_LINES",
				fmt.Sprintf("a comment block %s is capped at %d line%s; this one runs %d", where, cap, plural, len(substantive))})
		}
	}

	for number, line := range lines {
		lineno := number + 1
		if number < headerEnd {
			continue
		}
		start := CommentStart(line, family)
		if start == -1 {
			continue
		}
		body := CommentBody(line[start:], family)
		if IsDirective(body) {
			continue
		}
		words := len(strings.Fields(body))
		if len(body) > MaxChars || words > MaxWords {
			findings = append(findings, Finding{lineno, "OVER_WORDS",
				fmt.Sprintf("comment runs %d words; the cap is %d", words, MaxWords)})
			continue
		}
		if cited := ExternalReference(body); cited != "" {
			findings = append(findings, Finding{lineno, "EXTERNAL_REF",
				"comment cites " + cited + "; describe the code, not a document, version, or conversation"})
			continue
		}
		if tell := Narrative(body); tell != "" {
			findings = append(findings, Finding{lineno, "NARRATIVE",
				"comment carries " + tell + "; state what the code does"})
			continue
		}
		upper := strings.ToUpper(body)
		if hasPrefixAny(upper, markers...) && !markerShape.MatchString(body) {
			findings = append(findings, Finding{lineno, "MALFORMED_MARKER",
				"a marker must read 'NOTE: text', 'TODO: text', or 'FIXME: text'"})
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), marker) && number+1 < len(lines) && ext != ".go" && words <= 3 {
			name := FunctionName(lines[number+1], ext)
			first := ""
			if fields := strings.Fields(body); len(fields) > 0 {
				first = strings.ToLower(strings.Trim(fields[0], "*(),.:;'\"`"))
			}
			if name != "" && first == strings.ToLower(name) {
				findings = append(findings, Finding{lineno, "NAME_ECHO", "comment restates the identifier below it"})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Rule < findings[j].Rule
	})
	return findings
}

// CheckFile lints one file on disk and returns its findings and what it left undocumented.
func CheckFile(path, require string) ([]Finding, []Undocumented, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	text := string(data)
	return InvalidComments(text, path), UndocumentedFunctions(text, path, require), nil
}

// Check lints every path and returns one line per problem.
func Check(root string, paths []string, require string) ([]string, error) {
	var out []string
	for _, path := range paths {
		full := path
		if !filepath.IsAbs(path) {
			full = filepath.Join(root, path)
		}
		if info, err := os.Stat(full); err != nil || info.IsDir() {
			continue
		}
		findings, undocumented, err := CheckFile(full, require)
		if err != nil {
			return nil, err
		}
		for _, finding := range findings {
			out = append(out, fmt.Sprintf("%s:%d %s: %s", path, finding.Line, finding.Rule, finding.Detail))
		}
		for _, item := range undocumented {
			out = append(out, fmt.Sprintf("%s:%d UNDOCUMENTED: %s owes a one-line comment", path, item.Line, item.Name))
		}
	}
	return out, nil
}
