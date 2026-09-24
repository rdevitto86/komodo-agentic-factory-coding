package guard

import (
	"os"
	"path/filepath"
	"strings"
)

// xargsInput stands in for the words xargs feeds its command at run time, whatever its placeholder.
const xargsInput = "{}"

// resolveWritePath resolves a redirect or tee target against cwd the same way pathFindings does,
// reporting false when the guard cannot resolve it.
func resolveWritePath(target, cwd string) (string, bool) {
	if target == "" || unresolvedVarRe.MatchString(target) || otherHomeRe.MatchString(target) {
		return "", false
	}
	resolved := expandHome(target)
	if !filepath.IsAbs(resolved) {
		if cwd == unresolvedDir {
			return "", false
		}
		resolved = filepath.Join(cwd, resolved)
	}
	return filepath.Clean(resolved), true
}

// writeContent is the text a redirect puts into its target: what echo or printf prints, or the
// stdin a bare cat copies; any other command's output is unknown, since it may transform stdin.
func writeContent(cmd simpleCommand, stdin string) (string, bool) {
	if text, printed, known := echoedText(cmd); printed {
		return text, known
	}
	if len(cmd.words) == 0 {
		return "", true
	}
	if !passesLiterally(cmd) || strings.Contains(stdin, unknownInput) {
		return "", false
	}
	return stdin, true
}

// recordWrites remembers what this line put into each redirect target, so a later read judges that
// content, not disk; an append adds to what the file already held.
func (s *scanner) recordWrites(cmd simpleCommand, cwd, stdin string) {
	text, known := writeContent(cmd, stdin)
	for _, target := range cmd.writes {
		resolved, ok := resolveWritePath(target, cwd)
		if !ok {
			continue
		}
		s.put(resolved, cwd, scriptWrite{text: text, known: known}, cmd.appends[target])
	}
}

// put records one write to a resolved path, adding to what the line or disk held when it appends.
func (s *scanner) put(resolved, cwd string, write scriptWrite, appends bool) {
	if appends {
		before, found := s.writes[resolved]
		if !found {
			disk, exists, readable := readScript(resolved, cwd)
			before = scriptWrite{text: disk, known: !exists || readable}
		}
		write = scriptWrite{text: before.text + "\n" + write.text, known: before.known && write.known}
	}
	s.writes[resolved] = write
}

// recordOutputWrites marks files a command writes that the guard cannot read: curl -o, wget -O,
// dd of=, sed -i and perl -i edits, and the destination of cp, mv, install, and ln.
func (s *scanner) recordOutputWrites(name string, kept []string, cwd string) {
	var targets []string
	switch name {
	case "curl", "wget":
		short, long := "-o", "--output"
		if name == "wget" {
			short, long = "-O", "--output-document"
		}
		for index := 1; index < len(kept); index++ {
			arg := kept[index]
			switch {
			case arg == short || arg == long:
				if index+1 < len(kept) {
					targets = append(targets, kept[index+1])
					index++
				}
			case strings.HasPrefix(arg, long+"="):
				targets = append(targets, strings.TrimPrefix(arg, long+"="))
			case strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--"):
				// curl -sSLo x and wget -qO x carry the output letter inside a cluster.
				if at := strings.IndexByte(arg[1:], short[1]); at >= 0 {
					if rest := arg[at+2:]; rest != "" {
						targets = append(targets, rest)
					} else if index+1 < len(kept) {
						targets = append(targets, kept[index+1])
						index++
					}
				}
			}
		}
	case "sed", "perl":
		if isConditionalWriter(name, kept) {
			targets = writerTargets(name, kept)
		}
	case "dd":
		for _, arg := range kept[1:] {
			if value, ok := strings.CutPrefix(arg, "of="); ok {
				targets = append(targets, value)
			}
		}
	case "cp", "mv", "install", "ln":
		var operands []string
		for _, arg := range kept[1:] {
			if !strings.HasPrefix(arg, "-") {
				operands = append(operands, arg)
			}
		}
		if len(operands) >= 2 {
			targets = append(targets, operands[len(operands)-1])
		}
	}
	for _, target := range targets {
		if resolved, ok := resolveWritePath(target, cwd); ok {
			s.writes[resolved] = scriptWrite{}
		}
	}
	s.blind = s.blind || writesBlind(name, kept)
}

// blindGitVerbs are git subcommands that rewrite worktree files the guard never sees.
var blindGitVerbs = set("checkout", "restore", "switch", "reset", "apply", "am", "pull", "merge",
	"rebase", "cherry-pick", "stash", "clone", "revert")

// onlyCreatesBranch reports whether a checkout or switch makes a branch at HEAD, which changes no file:
// a create flag and a branch name with no start point.
func onlyCreatesBranch(sub string, rest []string) bool {
	if sub != "checkout" && sub != "switch" {
		return false
	}
	create := false
	var names []string
	for _, arg := range rest {
		switch arg {
		case "-b", "-B", "-c", "-C", "--create", "--force-create":
			create = true
		default:
			if !strings.HasPrefix(arg, "-") {
				names = append(names, arg)
			}
		}
	}
	return create && len(names) == 1
}

// writesBlind reports whether a command rewrites files under names the guard cannot list: curl -O,
// an archive extract, a patch, or a git call that changes the worktree.
func writesBlind(name string, kept []string) bool {
	switch name {
	case "curl":
		for _, arg := range kept[1:] {
			if arg == "--remote-name" || arg == "--remote-name-all" ||
				(strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(arg, "O")) {
				return true
			}
		}
	case "tar", "bsdtar":
		for index, arg := range kept[1:] {
			short := strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--")
			// tar's mode is a dash cluster, or its first word in the old form tar xf a.tgz.
			if arg == "--extract" || arg == "--get" || ((short || index == 0) && strings.Contains(arg, "x")) {
				return true
			}
		}
	case "unzip", "patch", "rsync", "scp", "gunzip", "bunzip2", "unxz":
		return true
	case "7z", "7za", "7zz":
		return len(kept) > 1 && (kept[1] == "x" || kept[1] == "e")
	case "git":
		for index := 1; index < len(kept); index++ {
			arg := kept[index]
			if arg == "-C" || arg == "-c" {
				index++
				continue
			}
			if !strings.HasPrefix(arg, "-") {
				return blindGitVerbs[arg] && !onlyCreatesBranch(arg, kept[index+1:])
			}
		}
	}
	return false
}

// recordTeeWrites remembers what tee's stdin put into each file it names.
func (s *scanner) recordTeeWrites(kept []string, cwd, stdin string) {
	known := stdin != "" && !strings.Contains(stdin, unknownInput)
	appends := false
	var files []string
	for _, token := range kept[1:] {
		switch {
		case token == "--append" || (strings.HasPrefix(token, "-") && !strings.HasPrefix(token, "--") && strings.Contains(token, "a")):
			appends = true
		case token != "" && !strings.HasPrefix(token, "-"):
			files = append(files, token)
		}
	}
	for _, file := range files {
		if resolved, ok := resolveWritePath(file, cwd); ok {
			s.put(resolved, cwd, scriptWrite{text: stdin, known: known}, appends)
		}
	}
}

// recordedWrite returns what this command line already wrote to target, when the guard saw it happen.
func (s *scanner) recordedWrite(target, cwd string) (scriptWrite, bool) {
	resolved, ok := resolveWritePath(target, cwd)
	if !ok {
		return scriptWrite{}, false
	}
	write, found := s.writes[resolved]
	return write, found
}

// createdEarlier reports whether an earlier command in this line could have written a script
// the guard now finds missing on disk, as tar xf a.tgz && sh install.sh would.
func (s *scanner) createdEarlier() bool {
	return s.seen > 1
}

// looksLikeScriptPath reports whether a command names itself by a path, as ./x.sh or bin/x.sh do,
// rather than a bare name a shell resolves through PATH.
func looksLikeScriptPath(target string) bool {
	return strings.Contains(target, "/")
}

// scriptCommandFindings checks a command run by path: a recorded write, or a text file scanned as sh
// under a shell shebang or none, or read for hidden git or gh under an interpreter's.
func (s *scanner) scriptCommandFindings(target, cwd, branch string) ([]string, string) {
	text, exists, ok := readScript(target, cwd)
	write, recorded := s.recordedWrite(target, cwd)
	if recorded {
		if !write.known {
			return []string{scriptNotVisible}, branch
		}
		text, exists, ok = write.text, true, true
	}
	if !exists && (s.blind || (scriptExtensions[strings.ToLower(filepath.Ext(target))] && s.createdEarlier())) {
		return []string{scriptNotVisible}, branch
	}
	if !recorded && ((ok && s.blind) || (exists && !ok && !binaryFile(target, cwd))) {
		return []string{scriptNotVisible}, branch
	}
	if !ok {
		return nil, branch
	}
	switch interp := shebang(text); {
	case interp == "" || shells[interp]:
		return s.scan(text, cwd, branch)
	case interpreters[interpName(interp)] && hidesGitOrGhInFile(text):
		return []string{interpreterHidesGit}, branch
	}
	return nil, branch
}

// binaryFile reports whether a path run as a command is a compiled program, with an early NUL byte.
func binaryFile(target, cwd string) bool {
	file := expandHome(target)
	if !filepath.IsAbs(file) {
		file = filepath.Join(cwd, file)
	}
	handle, err := os.Open(file)
	if err != nil {
		return false
	}
	defer handle.Close()
	head := make([]byte, 8192)
	count, _ := handle.Read(head)
	return strings.ContainsRune(string(head[:count]), 0)
}

// shebang names the interpreter a script's first line asks for, through env when it uses it.
func shebang(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	rest, ok := strings.CutPrefix(line, "#!")
	if !ok {
		return ""
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	interp := filepath.Base(fields[0])
	if interp == "env" {
		for _, field := range fields[1:] {
			if !strings.HasPrefix(field, "-") {
				return filepath.Base(field)
			}
		}
	}
	return interp
}
