package line

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"komodo/internal/glob"
)

// Slot caps, in characters, as the README's Devices table sets them.
const (
	CapRepoRules   = 8000
	CapRepoContext = 8000
	CapPerFile     = 10000
	CapFilesTotal  = 24000
	CapStandard    = 6000
	CapFailure     = 80000
)

// Clip truncates text at limit with a visible marker so a machine knows it saw a cut.
func Clip(text string, limit int, label string) string {
	if len(text) <= limit {
		return text
	}
	if label == "" {
		label = "content"
	}
	head := text[:int(float64(limit)*0.7)]
	tail := text[len(text)-int(float64(limit)*0.25):]
	marker := fmt.Sprintf("\n\n[... %s truncated: %d of %d chars shown ...]\n\n", label, len(head)+len(tail), len(text))
	return head + marker + tail
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// slug reduces a heading or an anchor to its comparable form.
func slug(text string) string {
	return strings.Trim(slugPattern.ReplaceAllString(strings.ToLower(text), "-"), "-")
}

// Section returns the markdown section whose heading matches the anchor, or the empty string.
func Section(text, anchor string) string {
	want := slug(anchor)
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if !strings.HasPrefix(line, "#") {
			continue
		}
		if slug(strings.TrimSpace(strings.TrimLeft(line, "#"))) != want {
			continue
		}
		level := len(line) - len(strings.TrimLeft(line, "#"))
		end := index + 1
		for end < len(lines) {
			candidate := lines[end]
			if strings.HasPrefix(candidate, "#") && len(candidate)-len(strings.TrimLeft(candidate, "#")) <= level {
				break
			}
			end++
		}
		return strings.Join(lines[index:end], "\n")
	}
	return ""
}

// Tokens is the rough token count of a slot: four characters to a token.
func Tokens(text string) int { return (len(text) + 3) / 4 }

// IsText reports whether a file looks like text: valid UTF-8 with no NUL in its first bytes.
func IsText(path string) bool {
	handle, err := os.Open(path)
	if err != nil {
		return false
	}
	defer handle.Close()
	const probe = 8000
	buffer := make([]byte, probe)
	read, _ := handle.Read(buffer)
	buffer = buffer[:read]
	if len(buffer) == 0 {
		return true
	}
	for _, b := range buffer {
		if b == 0 {
			return false
		}
	}
	// A short read reached EOF, so an invalid tail is real, not a buffer boundary cutting a rune.
	return utf8.Valid(buffer) || read == probe
}

// MatchGlob reports whether a path matches one of the shipped glob shapes.
func MatchGlob(pattern, path string) bool {
	return glob.Match(pattern, path)
}
