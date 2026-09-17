package main

import "errors"

var errNoClose = errors.New("no closing quotation")

// splitWords splits a command into POSIX shell words, erroring on an unterminated quote or escape.
func splitWords(s string) ([]string, error) {
	var words []string
	var cur []rune
	started := false
	quote := rune(0)
	escape := false
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case escape:
			// Inside double quotes only these four stay escapable; otherwise the backslash is literal.
			if quote == '"' && c != '\\' && c != '"' && c != '$' && c != '`' {
				cur = append(cur, '\\')
			}
			cur = append(cur, c)
			escape = false
		case c == '\\' && quote != '\'':
			escape = true
			started = true
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				cur = append(cur, c)
			}
		case c == '\'' || c == '"':
			quote = c
			started = true
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if started {
				words = append(words, string(cur))
				cur = cur[:0]
				started = false
			}
		default:
			cur = append(cur, c)
			started = true
		}
	}
	if quote != 0 || escape {
		return nil, errNoClose
	}
	if started {
		words = append(words, string(cur))
	}
	return words, nil
}
