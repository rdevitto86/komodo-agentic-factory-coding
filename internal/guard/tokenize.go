package guard

import "strings"

// call is one simple command between control operators: its words and its redirect targets.
type call struct {
	words  []string
	writes []string
}

// controlOperators end one call and start the next in a command line.
var controlOperators = map[string]bool{";": true, "&&": true, "||": true, "&": true, "|": true, "(": true, ")": true}

// tokenize splits a command line into simple calls, stripping quotes from each word and pulling
// every > and >> target out as a write, the only two things the guard still looks for.
func tokenize(command string) []call {
	var calls []call
	var current call
	flush := func() {
		if len(current.words) > 0 || len(current.writes) > 0 {
			calls = append(calls, current)
		}
		current = call{}
	}
	tokens := rawTokens(command)
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		switch {
		case controlOperators[token]:
			flush()
		case token == ">" || token == ">>":
			if index+1 < len(tokens) {
				index++
				current.writes = append(current.writes, tokens[index])
			}
		default:
			current.words = append(current.words, token)
		}
	}
	flush()
	return calls
}

// rawTokens splits a command line into words and operators, honouring single and double quotes
// and a backslash escape, the way a shell's own word splitting does before anything else runs.
func rawTokens(command string) []string {
	var tokens []string
	var buf strings.Builder
	open := false
	inSingle, inDouble := false, false
	flushWord := func() {
		if open {
			tokens = append(tokens, buf.String())
			buf.Reset()
			open = false
		}
	}
	addOp := func(op string) {
		flushWord()
		tokens = append(tokens, op)
	}
	runes := []rune(command)
	for index := 0; index < len(runes); index++ {
		char := runes[index]
		switch {
		case inSingle:
			open = true
			if char == '\'' {
				inSingle = false
			} else {
				buf.WriteRune(char)
			}
		case inDouble:
			open = true
			switch {
			case char == '"':
				inDouble = false
			case char == '\\' && index+1 < len(runes) && strings.ContainsRune("\"\\$`", runes[index+1]):
				index++
				buf.WriteRune(runes[index])
			default:
				buf.WriteRune(char)
			}
		case char == '\'':
			inSingle, open = true, true
		case char == '"':
			inDouble, open = true, true
		case char == '\\' && index+1 < len(runes):
			index++
			open = true
			buf.WriteRune(runes[index])
		case char == '\n':
			addOp(";")
		case char == ' ' || char == '\t' || char == '\r':
			flushWord()
		case char == '&' && index+1 < len(runes) && runes[index+1] == '&':
			index++
			addOp("&&")
		case char == '|' && index+1 < len(runes) && runes[index+1] == '|':
			index++
			addOp("||")
		case char == '>' && index+1 < len(runes) && runes[index+1] == '>':
			index++
			addOp(">>")
		case char == ';' || char == '&' || char == '|' || char == '>' || char == '(' || char == ')':
			addOp(string(char))
		default:
			open = true
			buf.WriteRune(char)
		}
	}
	flushWord()
	return tokens
}
