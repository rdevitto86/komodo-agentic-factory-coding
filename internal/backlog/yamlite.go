package backlog

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// The YAML subset a task block uses: flat keys, scalars, inline lists, dashed lists.

var (
	keyLine  = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*:\s*(.*)$`)
	dashLine = regexp.MustCompile(`^\s*-\s+(.*)$`)
	intOnly  = regexp.MustCompile(`^-?\d+$`)
)

// Fields is one parsed block: keys in file order, values as string, int, bool, nil, or a slice.
type Fields struct {
	keys   []string
	values map[string]any
}

// Get returns the value stored under key, and whether the key was present.
func (f Fields) Get(key string) (any, bool) {
	value, ok := f.values[key]
	return value, ok
}

// Keys returns the block's keys in the order they were written.
func (f Fields) Keys() []string { return append([]string(nil), f.keys...) }

// Set stores a value, appending the key when it is new.
func (f *Fields) Set(key string, value any) {
	if f.values == nil {
		f.values = map[string]any{}
	}
	if _, seen := f.values[key]; !seen {
		f.keys = append(f.keys, key)
	}
	f.values[key] = value
}

// String returns the value under key as text, or the empty string.
func (f Fields) String(key string) string {
	value, ok := f.values[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

// List returns the value under key as a slice of strings, or nil.
func (f Fields) List(key string) []string {
	value, ok := f.values[key]
	if !ok {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, fmt.Sprint(item))
	}
	return out
}

// stripComment drops a trailing # comment unless the # sits inside quotes.
func stripComment(text string) string {
	var quote rune
	for index, char := range text {
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == '#' && (index == 0 || isSpaceByte(text[index-1])) {
			return strings.TrimRight(text[:index], " \t")
		}
	}
	return strings.TrimRight(text, " \t")
}

// isSpaceByte reports whether the byte is an ASCII space or tab.
func isSpaceByte(b byte) bool { return b == ' ' || b == '\t' }

// scalar turns one token into a string, int, bool, or nil, honouring quotes.
func scalar(raw string) any {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	if len(text) >= 2 && text[0] == text[len(text)-1] && text[0] == '\'' {
		// Inside single quotes YAML writes a single quote twice.
		return strings.ReplaceAll(text[1:len(text)-1], "''", "'")
	}
	if len(text) >= 2 && text[0] == text[len(text)-1] && text[0] == '"' {
		return text[1 : len(text)-1]
	}
	switch strings.ToLower(text) {
	case "true", "yes":
		return true
	case "false", "no":
		return false
	case "null", "~":
		return nil
	}
	if intOnly.MatchString(text) {
		number, err := strconv.Atoi(text)
		if err == nil {
			return number
		}
	}
	return text
}

// splitInline splits [a, b, "c, d"] on the commas that sit outside quotes.
func splitInline(body string) []string {
	var items []string
	var current strings.Builder
	var quote rune
	for _, char := range body {
		if quote != 0 {
			current.WriteRune(char)
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			current.WriteRune(char)
			continue
		}
		if char == ',' {
			items = append(items, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(char)
	}
	if tail := current.String(); strings.TrimSpace(tail) != "" || len(items) > 0 {
		items = append(items, tail)
	}
	out := items[:0]
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			out = append(out, item)
		}
	}
	return out
}

// ParseFields reads a block into Fields, failing on anything outside the subset.
func ParseFields(text string) (Fields, error) {
	fields := Fields{values: map[string]any{}}
	lines := strings.Split(text, "\n")
	index := 0
	for index < len(lines) {
		line := stripComment(lines[index])
		index++
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isSpaceByte(line[0]) {
			return fields, fmt.Errorf("unexpected indentation at: %q", line)
		}
		match := keyLine.FindStringSubmatch(line)
		if match == nil {
			return fields, fmt.Errorf("expected 'key: value' at: %q", line)
		}
		key, value := match[1], strings.TrimSpace(match[2])
		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			var items []any
			for _, item := range splitInline(value[1 : len(value)-1]) {
				items = append(items, scalar(item))
			}
			if items == nil {
				items = []any{}
			}
			fields.Set(key, items)
			continue
		}
		if value != "" {
			fields.Set(key, scalar(value))
			continue
		}
		items := []any{}
		for index < len(lines) {
			candidate := stripComment(lines[index])
			if strings.TrimSpace(candidate) == "" {
				index++
				continue
			}
			dash := dashLine.FindStringSubmatch(candidate)
			if dash == nil {
				break
			}
			items = append(items, scalar(dash[1]))
			index++
		}
		fields.Set(key, items)
	}
	return fields, nil
}

// newlineRe matches any line break so quote can collapse one into a space.
var newlineRe = regexp.MustCompile(`\r\n|\r|\n`)

// quote renders a scalar, quoting text the parser would otherwise misread.
// A newline is collapsed to a space, since a raw one would break the fenced block.
func quote(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(typed)
	}
	text := newlineRe.ReplaceAllString(fmt.Sprint(value), " ")
	needs := text == "" || text != strings.TrimSpace(text) ||
		strings.ContainsAny(text, ":#[]{},\"'") || intOnly.MatchString(text)
	switch strings.ToLower(text) {
	case "true", "false", "yes", "no", "null", "~":
		needs = true
	}
	if !needs {
		return text
	}
	// A value holding a double quote takes single quotes, each single quote inside written twice.
	if strings.Contains(text, "\"") {
		return "'" + strings.ReplaceAll(text, "'", "''") + "'"
	}
	return `"` + text + `"`
}

// DumpFields renders Fields back into the subset, lists dashed one item per line.
func DumpFields(fields Fields) string {
	var out []string
	for _, key := range fields.keys {
		value := fields.values[key]
		items, isList := value.([]any)
		if !isList {
			out = append(out, key+": "+quote(value))
			continue
		}
		if len(items) == 0 {
			out = append(out, key+": []")
			continue
		}
		out = append(out, key+":")
		for _, item := range items {
			out = append(out, "  - "+quote(item))
		}
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}
