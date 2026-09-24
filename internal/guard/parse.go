package guard

import (
	"strings"
)

// simpleCommand is one command between control operators: its words, what it writes, and its stdin.
type simpleCommand struct {
	words   []word
	writes  []string
	appends map[string]bool
	stdin   []string
	pipesTo bool
}

// parse groups tokens into simple commands, attaching each redirect to the command it follows.
func parse(tokens []token) []simpleCommand {
	var commands []simpleCommand
	var current simpleCommand
	flush := func(pipes bool) {
		if len(current.words) > 0 || len(current.writes) > 0 || len(current.stdin) > 0 {
			current.pipesTo = pipes
			commands = append(commands, current)
		}
		current = simpleCommand{}
	}
	for index := 0; index < len(tokens); index++ {
		t := tokens[index]
		switch t.kind {
		case tokenControl:
			// name=( ... ) is an array literal, not a subshell, so its words never run.
			if t.text == "(" && isArrayOpen(current.words) {
				for index < len(tokens) && !(tokens[index].kind == tokenControl && tokens[index].text == ")") {
					index++
				}
				continue
			}
			flush(t.text == "|" || t.text == "|&")
		case tokenRedirect:
			var target word
			if index+1 < len(tokens) && tokens[index+1].kind == tokenWord && t.doc == nil {
				index++
				target = tokens[index].word
			}
			switch {
			case t.doc != nil:
				current.stdin = append(current.stdin, t.doc.body)
			case t.text == "<<<":
				current.stdin = append(current.stdin, target.value)
			case t.text == "<&" || t.text == "<":
			case t.text == ">&" && isDescriptor(target.value):
			default:
				current.writes = append(current.writes, target.value)
				if t.text == ">>" || t.text == "&>>" {
					if current.appends == nil {
						current.appends = map[string]bool{}
					}
					current.appends[target.value] = true
				}
			}
		case tokenWord:
			current.words = append(current.words, expandBraces(t.word)...)
		}
	}
	flush(false)
	return commands
}

// isArrayOpen reports whether the last word is a bare name= that an array literal follows.
func isArrayOpen(words []word) bool {
	if len(words) == 0 {
		return false
	}
	last := words[len(words)-1].value
	return strings.HasSuffix(last, "=") && assignRe.MatchString(last)
}

// isDescriptor reports whether a >& target is a descriptor number or -, not a file.
func isDescriptor(target string) bool {
	if target == "-" {
		return true
	}
	trimmed := strings.TrimSuffix(target, "-")
	if trimmed == "" {
		return false
	}
	for _, char := range trimmed {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// maxBraceWords caps how many words one brace expansion may yield, so a hostile pattern cannot explode.
const maxBraceWords = 256

// expandBraces expands every unquoted {a,b} list in a word, as bash does before it runs a command.
func expandBraces(w word) []word {
	out := []word{w}
	for round := 0; round < 16; round++ {
		var next []word
		grew := false
		for _, item := range out {
			parts, ok := expandOne(item)
			if !ok {
				next = append(next, item)
				continue
			}
			grew = true
			next = append(next, parts...)
		}
		if len(next) > maxBraceWords {
			return out
		}
		out = next
		if !grew {
			break
		}
	}
	return out
}

// expandOne expands the first unquoted brace list in a word, reporting whether there was one.
func expandOne(w word) ([]word, bool) {
	runes := []rune(w.value)
	quoted := w.quoted
	for open := 0; open < len(runes); open++ {
		if runes[open] != '{' || quoted[open] {
			continue
		}
		depth := 0
		var commas []int
		for close := open; close < len(runes); close++ {
			if quoted[close] {
				continue
			}
			switch runes[close] {
			case '{':
				depth++
			case '}':
				depth--
			case ',':
				if depth == 1 {
					commas = append(commas, close)
				}
			}
			if depth == 0 {
				if len(commas) == 0 {
					break
				}
				bounds := append(append([]int{open}, commas...), close)
				var out []word
				for part := 0; part+1 < len(bounds); part++ {
					value := append(append(append([]rune{}, runes[:open]...), runes[bounds[part]+1:bounds[part+1]]...), runes[close+1:]...)
					marks := append(append(append([]bool{}, quoted[:open]...), quoted[bounds[part]+1:bounds[part+1]]...), quoted[close+1:]...)
					out = append(out, word{value: string(value), quoted: marks})
				}
				return out, true
			}
		}
	}
	return nil, false
}
