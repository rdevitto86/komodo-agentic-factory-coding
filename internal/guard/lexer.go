package guard

import (
	"strconv"
	"strings"
)

// word is one shell word after quote removal, with a mark per rune for whether a quote covered it.
type word struct {
	value   string
	quoted  []bool
	expands bool
}

// anyQuoted reports whether any rune of the word sat inside quotes or behind a backslash.
func (w word) anyQuoted() bool {
	for _, mark := range w.quoted {
		if mark {
			return true
		}
	}
	return false
}

// tokenKind separates a word from a control operator and a redirect operator.
type tokenKind int

const (
	tokenWord tokenKind = iota
	tokenControl
	tokenRedirect
)

// token is one lexeme: a word, or the text of an operator.
type token struct {
	kind tokenKind
	text string
	word word
	doc  *heredoc
}

// heredoc is one << redirect: its terminator, whether its body expands, and the body once read.
type heredoc struct {
	delim  string
	strip  bool
	expand bool
	body   string
}

// lexed is a whole command line: its tokens, and every substitution body found anywhere in it.
type lexed struct {
	tokens        []token
	substitutions []string
}

// lexer walks a command line once, rune by rune, the way a POSIX shell's tokenizer does.
type lexer struct {
	runes   []rune
	pos     int
	out     lexed
	pending []*heredoc
}

// lex splits a command line into shell tokens, reading heredoc bodies and substitution bodies as it goes.
func lex(command string) lexed {
	l := &lexer{runes: []rune(command)}
	l.run()
	return l.out
}

// controlOperators are the multi-rune control operators, longest first so a prefix never wins.
var controlOperators = []string{";;&", ";;", ";&", "&&", "||", "|&", ";", "|", "&", "(", ")", "\n"}

// redirectOperators are the redirect operators, longest first.
var redirectOperators = []string{"<<<", "<<-", "&>>", "<<", "<>", "<&", ">>", ">|", ">&", "&>", "<", ">"}

// run tokenizes the whole input.
func (l *lexer) run() {
	for l.pos < len(l.runes) {
		char := l.runes[l.pos]
		switch {
		case char == ' ' || char == '\t' || char == '\r':
			l.pos++
		case char == '\\' && l.peek(1) == '\n':
			l.pos += 2
		case char == '#':
			for l.pos < len(l.runes) && l.runes[l.pos] != '\n' {
				l.pos++
			}
		case char == '\n':
			l.pos++
			l.emit(token{kind: tokenControl, text: "\n"})
			l.readHeredocs()
		case (char == '<' || char == '>') && l.peek(1) == '(':
			l.emit(token{kind: tokenWord, word: l.readWord()})
		default:
			if l.skipDescriptor() {
				continue
			}
			if op := l.match(redirectOperators); op != "" {
				l.pos += len([]rune(op))
				l.redirect(op)
				continue
			}
			if op := l.match(controlOperators); op != "" {
				l.pos += len([]rune(op))
				l.emit(token{kind: tokenControl, text: op})
				continue
			}
			l.emit(token{kind: tokenWord, word: l.readWord()})
		}
	}
	l.readHeredocs()
}

// emit appends one token.
func (l *lexer) emit(t token) {
	l.out.tokens = append(l.out.tokens, t)
}

// peek returns the rune offset places ahead, or zero past the end.
func (l *lexer) peek(offset int) rune {
	if l.pos+offset < len(l.runes) {
		return l.runes[l.pos+offset]
	}
	return 0
}

// match returns the first operator the input continues with, or the empty string.
func (l *lexer) match(operators []string) string {
	for _, op := range operators {
		opRunes := []rune(op)
		if l.pos+len(opRunes) > len(l.runes) {
			continue
		}
		if string(l.runes[l.pos:l.pos+len(opRunes)]) == op {
			// <( and >( open a process substitution, which is a word, not a redirect.
			if (op == "<" || op == ">") && l.peek(1) == '(' {
				continue
			}
			return op
		}
	}
	return ""
}

// skipDescriptor drops a file descriptor number glued to a redirect, as in 2> or 10<&.
func (l *lexer) skipDescriptor() bool {
	end := l.pos
	for end < len(l.runes) && l.runes[end] >= '0' && l.runes[end] <= '9' {
		end++
	}
	if end == l.pos || end >= len(l.runes) || (l.runes[end] != '<' && l.runes[end] != '>') {
		return false
	}
	if end+1 < len(l.runes) && l.runes[end+1] == '(' {
		return false
	}
	l.pos = end
	return true
}

// redirect emits a redirect operator and, for a heredoc, queues its body for the next newline.
func (l *lexer) redirect(op string) {
	t := token{kind: tokenRedirect, text: op}
	if op == "<<" || op == "<<-" {
		l.skipBlanks()
		delim := l.readWord()
		doc := &heredoc{delim: delim.value, strip: op == "<<-", expand: !delim.anyQuoted()}
		l.pending = append(l.pending, doc)
		t.doc = doc
	}
	l.emit(t)
}

// skipBlanks steps over spaces and tabs.
func (l *lexer) skipBlanks() {
	for l.pos < len(l.runes) && (l.runes[l.pos] == ' ' || l.runes[l.pos] == '\t') {
		l.pos++
	}
}

// readHeredocs reads the body of every queued heredoc, in order, from the lines that follow.
func (l *lexer) readHeredocs() {
	for len(l.pending) > 0 {
		doc := l.pending[0]
		l.pending = l.pending[1:]
		var lines []string
		for l.pos < len(l.runes) {
			end := l.pos
			for end < len(l.runes) && l.runes[end] != '\n' {
				end++
			}
			line := string(l.runes[l.pos:end])
			l.pos = min(end+1, len(l.runes))
			check := line
			if doc.strip {
				check = strings.TrimLeft(check, "\t")
			}
			if strings.TrimRight(check, " \r") == doc.delim {
				break
			}
			lines = append(lines, line)
		}
		doc.body = strings.Join(lines, "\n")
		if doc.expand {
			l.out.substitutions = append(l.out.substitutions, expansionBodies(doc.body)...)
		}
	}
}

// isWordEnd reports whether an unquoted rune ends a word.
func isWordEnd(char rune) bool {
	switch char {
	case ' ', '\t', '\r', '\n', ';', '&', '|', '(', ')', '<', '>':
		return true
	}
	return false
}

// readWord reads one word, removing quotes and recording every substitution body inside it.
func (l *lexer) readWord() word {
	var b wordBuilder
	for l.pos < len(l.runes) {
		char := l.runes[l.pos]
		switch {
		case (char == '<' || char == '>') && l.peek(1) == '(':
			start := l.pos
			l.pos += 2
			body, end := readBalanced(l.runes, l.pos)
			l.out.substitutions = append(l.out.substitutions, body)
			l.pos = end
			b.add(string(l.runes[start:l.pos]), true)
		case isWordEnd(char):
			return b.word()
		case char == '\\':
			switch next := l.peek(1); {
			case next == '\n':
				l.pos += 2
			case next == 0:
				b.add(`\`, true)
				l.pos++
			default:
				b.add(string(next), true)
				l.pos += 2
			}
		case char == '\'':
			l.pos++
			start := l.pos
			for l.pos < len(l.runes) && l.runes[l.pos] != '\'' {
				l.pos++
			}
			b.add(string(l.runes[start:l.pos]), true)
			l.pos++
		case char == '$' && l.peek(1) == '\'':
			l.pos += 2
			b.add(l.readANSI(), true)
		case char == '$' && l.peek(1) == '"':
			l.pos++
		case char == '"':
			l.pos++
			l.readDouble(&b)
		case char == '$' || char == '`':
			l.readExpansion(&b, false)
		default:
			b.add(string(char), false)
			l.pos++
		}
	}
	return b.word()
}

// readDouble reads the inside of a double-quoted string, up to and past its closing quote.
func (l *lexer) readDouble(b *wordBuilder) {
	for l.pos < len(l.runes) {
		char := l.runes[l.pos]
		switch {
		case char == '"':
			l.pos++
			return
		case char == '\\':
			next := l.peek(1)
			switch next {
			case '\n':
				l.pos += 2
			case '$', '`', '"', '\\':
				b.add(string(next), true)
				l.pos += 2
			case 0:
				b.add(`\`, true)
				l.pos++
			default:
				b.add(`\`+string(next), true)
				l.pos += 2
			}
		case char == '$' || char == '`':
			l.readExpansion(b, true)
		default:
			b.add(string(char), true)
			l.pos++
		}
	}
}

// readExpansion reads a $(...), a backtick, a ${...}, or a bare $, keeping its text in the word.
func (l *lexer) readExpansion(b *wordBuilder, inDouble bool) {
	start := l.pos
	switch {
	case l.runes[l.pos] == '`':
		l.pos++
		var body strings.Builder
		for l.pos < len(l.runes) && l.runes[l.pos] != '`' {
			if l.runes[l.pos] == '\\' && l.pos+1 < len(l.runes) {
				l.pos++
			}
			body.WriteRune(l.runes[l.pos])
			l.pos++
		}
		l.pos = min(l.pos+1, len(l.runes))
		l.out.substitutions = append(l.out.substitutions, body.String())
	case l.peek(1) == '(':
		l.pos += 2
		body, end := readBalanced(l.runes, l.pos)
		l.out.substitutions = append(l.out.substitutions, body)
		l.pos = end
	case l.peek(1) == '{':
		l.pos += 2
		depth := 1
		inner := l.pos
		for l.pos < len(l.runes) && depth > 0 {
			switch l.runes[l.pos] {
			case '\\':
				l.pos++
			case '{':
				depth++
			case '}':
				depth--
			}
			l.pos++
		}
		innerEnd := max(inner, min(l.pos-1, len(l.runes)))
		if depth > 0 {
			innerEnd = len(l.runes)
		}
		l.out.substitutions = append(l.out.substitutions, expansionBodies(string(l.runes[inner:innerEnd]))...)
	default:
		b.expands = b.expands || isParamStart(l.peek(1))
		l.pos++
		b.add("$", inDouble)
		return
	}
	b.expands = true
	// A variable or substitution is never a brace or glob pattern, so it is marked quoted.
	b.add(string(l.runes[start:min(l.pos, len(l.runes))]), true)
}

// isParamStart reports whether a rune after $ opens a parameter the shell expands: a name, a digit, or a special.
func isParamStart(char rune) bool {
	return char == '_' || (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9') || strings.ContainsRune("?@*#!$-", char)
}

// readANSI reads a $'...' string's inside and decodes its C escapes.
func (l *lexer) readANSI() string {
	var out strings.Builder
	for l.pos < len(l.runes) {
		char := l.runes[l.pos]
		if char == '\'' {
			l.pos++
			return out.String()
		}
		if char != '\\' || l.pos+1 >= len(l.runes) {
			out.WriteRune(char)
			l.pos++
			continue
		}
		l.pos++
		escape := l.runes[l.pos]
		l.pos++
		switch escape {
		case 'n':
			out.WriteRune('\n')
		case 't':
			out.WriteRune('\t')
		case 'r':
			out.WriteRune('\r')
		case 'a':
			out.WriteRune('\a')
		case 'b':
			out.WriteRune('\b')
		case 'e', 'E':
			out.WriteRune(0x1b)
		case 'f':
			out.WriteRune('\f')
		case 'v':
			out.WriteRune('\v')
		case 'x':
			out.WriteRune(l.readCode(16, 2))
		case 'u':
			out.WriteRune(l.readCode(16, 4))
		case 'U':
			out.WriteRune(l.readCode(16, 8))
		case '0', '1', '2', '3', '4', '5', '6', '7':
			l.pos--
			out.WriteRune(l.readCode(8, 3))
		default:
			out.WriteRune(escape)
		}
	}
	return out.String()
}

// readCode reads up to limit digits in a base and returns the rune they name.
func (l *lexer) readCode(base, limit int) rune {
	start := l.pos
	for l.pos < len(l.runes) && l.pos-start < limit && strings.ContainsRune("0123456789abcdefABCDEF"[:base+max(0, base-10)], l.runes[l.pos]) {
		l.pos++
	}
	value, err := strconv.ParseUint(string(l.runes[start:l.pos]), base, 32)
	if err != nil {
		return 0
	}
	return rune(value)
}

// readBalanced reads to the ) that closes an already opened (, skipping quotes and escapes;
// it returns the body and the index just past the close, or the rest when nothing closes it.
func readBalanced(runes []rune, start int) (string, int) {
	depth := 1
	for index := start; index < len(runes); index++ {
		switch runes[index] {
		case '\\':
			index++
		case '\'':
			for index++; index < len(runes) && runes[index] != '\''; index++ {
			}
		case '"':
			for index++; index < len(runes) && runes[index] != '"'; index++ {
				if runes[index] == '\\' {
					index++
				}
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return string(runes[start:index]), index + 1
			}
		}
	}
	return string(runes[min(start, len(runes)):]), len(runes)
}

// expansionBodies returns every $(...) and backtick body in text where quotes are literal,
// as in an unquoted heredoc's body or inside a ${...}.
func expansionBodies(text string) []string {
	runes := []rune(text)
	var out []string
	for index := 0; index < len(runes); index++ {
		switch {
		case runes[index] == '\\':
			index++
		case runes[index] == '$' && index+1 < len(runes) && runes[index+1] == '(':
			body, end := readBalanced(runes, index+2)
			out = append(out, body)
			index = end - 1
		case runes[index] == '`':
			end := index + 1
			for end < len(runes) && runes[end] != '`' {
				end++
			}
			out = append(out, string(runes[index+1:min(end, len(runes))]))
			index = end
		}
	}
	return out
}

// wordBuilder accumulates a word's runes and their quote marks.
type wordBuilder struct {
	value   []rune
	quoted  []bool
	expands bool
}

// add appends text, every rune carrying the same quote mark.
func (b *wordBuilder) add(text string, quoted bool) {
	for _, char := range text {
		b.value = append(b.value, char)
		b.quoted = append(b.quoted, quoted)
	}
}

// word returns what the builder holds.
func (b *wordBuilder) word() word {
	return word{value: string(b.value), quoted: append([]bool(nil), b.quoted...), expands: b.expands}
}
