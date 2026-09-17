package main

import (
	"regexp"
	"strings"
	"sync"
)

var (
	cacheMu sync.Mutex
	cache   = map[string]*regexp.Regexp{}
)

// globMatch reports whether name matches a shell glob, with * crossing separators as fnmatch does.
func globMatch(name, pattern string) bool {
	cacheMu.Lock()
	re, ok := cache[pattern]
	if !ok {
		re = regexp.MustCompile(translate(pattern))
		cache[pattern] = re
	}
	cacheMu.Unlock()
	return re.MatchString(name)
}

// translate converts an fnmatch pattern into an anchored regular expression.
func translate(pattern string) string {
	var b strings.Builder
	b.WriteString(`\A`)
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch c {
		case '*':
			b.WriteString(`.*`)
		case '?':
			b.WriteString(`.`)
		case '[':
			end := i + 1
			if end < len(runes) && (runes[end] == '!' || runes[end] == '^') {
				end++
			}
			if end < len(runes) && runes[end] == ']' {
				end++
			}
			for end < len(runes) && runes[end] != ']' {
				end++
			}
			if end >= len(runes) {
				b.WriteString(`\[`)
				continue
			}
			body := string(runes[i+1 : end])
			if strings.HasPrefix(body, "!") {
				body = "^" + body[1:]
			}
			b.WriteString("[" + strings.ReplaceAll(body, `\`, `\\`) + "]")
			i = end
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString(`\z`)
	return b.String()
}
