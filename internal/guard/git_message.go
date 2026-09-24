package guard

import (
	"regexp"
	"strings"
)

// commitMessage composes the text of a commit or merge's own message: every -m paragraph, an
// -F file's content, and a --trailer's raw key=value line, in the order git reads them.
func commitMessage(rest []string, source *messageSource) string {
	var parts []string
	for index := 0; index < len(rest); index++ {
		arg := rest[index]
		switch {
		case arg == "-m" || arg == "--message":
			if index+1 < len(rest) {
				index++
				parts = append(parts, source.literal(rest[index], rest[index]))
			}
		case strings.HasPrefix(arg, "--message="):
			parts = append(parts, source.literal(arg, strings.TrimPrefix(arg, "--message=")))
		case strings.HasPrefix(arg, "-m") && arg != "-m":
			parts = append(parts, source.literal(arg, strings.TrimPrefix(arg, "-m")))
		case arg == "-F" || arg == "--file":
			if index+1 < len(rest) {
				index++
				parts = append(parts, source.file(rest[index], rest[index]))
			}
		case strings.HasPrefix(arg, "--file="):
			parts = append(parts, source.file(arg, strings.TrimPrefix(arg, "--file=")))
		case arg == "--trailer":
			if index+1 < len(rest) {
				index++
				parts = append(parts, source.literal(rest[index], rest[index]))
			}
		case strings.HasPrefix(arg, "--trailer="):
			parts = append(parts, source.literal(arg, strings.TrimPrefix(arg, "--trailer=")))
		}
	}
	return strings.Join(parts, "\n\n")
}

// messageFindings refuses a commit or forge text that carries a trailer, a private pattern, or text the guard cannot see.
func messageFindings(text string, source *messageSource, policy Policy) []string {
	var findings []string
	if policy.HasTrailer(normalizeMessage(text)) {
		findings = append(findings, "commit message carries a co-author or generated-by trailer")
	}
	if hasPrivate(text) {
		findings = append(findings, leakFinding)
	}
	if source.hidden {
		findings = append(findings, scriptNotVisible)
	}
	return findings
}

// messageBreakRe is a ; or && a message smuggles a trailer past, read as a line break instead.
var messageBreakRe = regexp.MustCompile(`\s*(?:&&|;)\s*`)

// normalizeMessage turns a message's own ; and && into line breaks before the trailer check.
func normalizeMessage(text string) string {
	return messageBreakRe.ReplaceAllString(text, "\n")
}

// messageSource reads a commit or forge call's outgoing text, noting when any of it is hidden.
type messageSource struct {
	s         *scanner
	cwd       string
	stdin     string
	expanding map[string]bool
	hidden    bool
}

// literal returns a message argument, marking it hidden when it came from a substitution or variable left unresolved.
func (m *messageSource) literal(arg, value string) string {
	if expandingIn(m.expanding, arg, value) && unresolvedText(value) {
		m.hidden = true
	}
	return value
}

// file reads a message file: the heredoc a - or /dev/stdin names, this line's recorded write, or
// disk, marking it hidden when the line may have written it unseen.
func (m *messageSource) file(arg, path string) string {
	if path == "-" || path == "/dev/stdin" {
		if strings.Contains(m.stdin, unknownInput) {
			m.hidden = true
		}
		return m.stdin
	}
	if expandingIn(m.expanding, arg, path) && unresolvedText(path) {
		m.hidden = true
		return ""
	}
	if write, found := m.s.recordedWrite(path, m.cwd); found {
		if !write.known {
			m.hidden = true
		}
		return write.text
	}
	text, exists, ok := readScript(path, m.cwd)
	switch {
	case exists && ok && !m.s.blind:
		return text
	case exists || m.s.createdEarlier():
		m.hidden = true
	}
	return ""
}
