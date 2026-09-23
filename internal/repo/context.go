// Package repo reads what a repo adds beyond komodo's shipped rules.
package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"komodo/internal/glob"
)

// ContextDir is where a repo declares brief context, relative to the repo root.
const ContextDir = ".komodo/context"

// Context is one repo context file: the paths its content applies to and its body.
type Context struct {
	Name  string
	Paths []string
	Body  string
}

var frontmatter = regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n(.*)\z`)

// LoadContext reads every context file the repo declares, skipping a malformed one with a note.
func LoadContext(root string) ([]Context, []string) {
	entries, err := os.ReadDir(filepath.Join(root, ContextDir))
	if err != nil {
		return nil, nil
	}
	var contexts []Context
	var skipped []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, ContextDir, entry.Name()))
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", entry.Name(), err))
			continue
		}
		match := frontmatter.FindStringSubmatch(normalizeNewlines(data))
		if match == nil {
			skipped = append(skipped, entry.Name()+": no frontmatter block")
			continue
		}
		context := Context{Name: entry.Name(), Body: strings.TrimSpace(match[2])}
		lines := strings.Split(match[1], "\n")
		for i := 0; i < len(lines); i++ {
			key, value, found := strings.Cut(lines[i], ":")
			if !found || strings.TrimSpace(key) != "paths" {
				continue
			}
			value = strings.TrimSpace(value)
			if value != "" {
				context.Paths = trimQuotes(splitList(value))
				continue
			}
			var items []string
			for i+1 < len(lines) {
				next := strings.TrimSpace(lines[i+1])
				if !strings.HasPrefix(next, "- ") {
					break
				}
				items = append(items, strings.TrimPrefix(next, "- "))
				i++
			}
			context.Paths = trimQuotes(items)
		}
		contexts = append(contexts, context)
	}
	sort.Slice(contexts, func(i, j int) bool { return contexts[i].Name < contexts[j].Name })
	return contexts, skipped
}

// Matches reports whether a task with these files pulls in this context; no paths means every task.
func (c Context) Matches(files []string) bool {
	if len(c.Paths) == 0 {
		return true
	}
	for _, pattern := range c.Paths {
		for _, file := range files {
			if glob.Match(pattern, file) {
				return true
			}
		}
	}
	return false
}

// normalizeNewlines collapses CRLF to LF so a frontmatter block parses either way.
func normalizeNewlines(data []byte) string {
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// splitList reads an inline [a, b] frontmatter list into its items.
func splitList(value string) []string {
	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	if value == "" {
		return []string{}
	}
	items := strings.Split(value, ",")
	out := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// trimQuotes strips the quotes a frontmatter list puts around each glob.
func trimQuotes(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, strings.Trim(item, `"'`))
	}
	return out
}
