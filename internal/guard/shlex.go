package guard

import "errors"

var errNoClose = errors.New("no closing quotation")

// splitWords splits a command into POSIX shell words, erroring on an unterminated quote or escape.
func splitWords(text string) ([]string, error) {
	var words []string
	var current []rune
	started := false
	quote := rune(0)
	escape := false
	runes := []rune(text)

	for index := 0; index < len(runes); index++ {
		char := runes[index]
		switch {
		case escape:
			if quote == '"' && char != '\\' && char != '"' && char != '$' && char != '`' {
				current = append(current, '\\')
			}
			current = append(current, char)
			escape = false
		case char == '\\' && quote != '\'':
			escape = true
			started = true
		case quote != 0:
			if char == quote {
				quote = 0
			} else {
				current = append(current, char)
			}
		case char == '\'' || char == '"':
			quote = char
			started = true
		case char == ' ' || char == '\t' || char == '\n' || char == '\r':
			if started {
				words = append(words, string(current))
				current = current[:0]
				started = false
			}
		default:
			current = append(current, char)
			started = true
		}
	}
	if quote != 0 || escape {
		return nil, errNoClose
	}
	if started {
		words = append(words, string(current))
	}
	return words, nil
}
