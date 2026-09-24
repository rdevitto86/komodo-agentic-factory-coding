package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// gitFindings refuses the git operations that touch a critical ref, and reports the branch after the call.
func gitFindings(tokens []string, branch, cwd string, policy Policy, stdin string) ([]string, string) {
	args := tokens[1:]
	var findings []string
	var configs []string
	elsewhere := ""
	index := 0
	for index < len(args) && strings.HasPrefix(args[index], "-") {
		arg := args[index]
		switch {
		case arg == "-C":
			// git -C <dir> reads and writes the branch of that checkout, not this one.
			if index+1 < len(args) && args[index+1] != "." {
				elsewhere = args[index+1]
			}
			index += 2
		case arg == "-c":
			if index+1 < len(args) {
				configs = append(configs, args[index+1])
			}
			index += 2
		default:
			name, value, hasValue := strings.Cut(arg, "=")
			index++
			if globalValueFlags[name] && !hasValue && index < len(args) {
				value = args[index]
				index++
			}
			switch name {
			case "--git-dir", "--work-tree":
				// Another repository's branch is not tracked; this checkout's own .git and . are.
				if clean := filepath.Clean(value); clean != "." && clean != ".git" {
					elsewhere = value
				}
			case "--config-env":
				configs = append(configs, value)
			}
		}
	}
	for _, entry := range configs {
		name, _, _ := strings.Cut(entry, "=")
		if remotePushConfigRe.MatchString(name) {
			findings = append(findings, fmt.Sprintf("git -c %s: a config write reaches push; open a pull request instead", entry))
		}
		if credentialConfigRe.MatchString(name) {
			findings = append(findings, fmt.Sprintf("git -c %s: hands git a credential the headless run scrubs", entry))
		}
	}
	if index >= len(args) {
		return findings, branch
	}
	sub, rest := args[index], args[index+1:]
	if alias := aliasValue(configs, sub); strings.HasPrefix(alias, "!") {
		return append(findings, fmt.Sprintf("git -c alias.%s: a shell alias hides its command; run the command directly", sub)), branch
	} else if alias != "" {
		if parts := strings.Fields(alias); len(parts) > 0 {
			sub, rest = parts[0], append(append([]string{}, parts[1:]...), rest...)
		}
	}
	if elsewhere != "" && !readOnlyGit(sub, rest) {
		return append(findings, fmt.Sprintf("git -C %s %s: the branch there is not tracked; open a pull request instead", elsewhere, sub)), branch
	}
	switch sub {
	case "push":
		var positional []string
		deletes := false
		mirrorFlag := ""
		for _, arg := range rest {
			switch arg {
			case "--delete", "-d":
				deletes = true
			case "--mirror", "--all":
				mirrorFlag = arg
			}
			if isForceFlag(arg) {
				findings = append(findings, fmt.Sprintf("git push %s: pushed history is never rewritten; push a new commit instead", arg))
			}
			if !strings.HasPrefix(arg, "-") {
				if strings.HasPrefix(arg, "+") {
					findings = append(findings, fmt.Sprintf("git push %s: a forced refspec rewrites pushed history; push a new commit instead", arg))
				}
				positional = append(positional, arg)
			}
		}
		if mirrorFlag != "" && hasAnyCritical(policy) {
			findings = append(findings, fmt.Sprintf("git push %s: reaches every ref, including a critical one; open a pull request instead", mirrorFlag))
		}
		targets := positional
		if len(positional) > 1 {
			targets = positional[1:]
		} else {
			targets = []string{branch}
			if hasAnyCritical(policy) && len(positional) == 1 && unresolvedTarget(positional[0]) {
				findings = append(findings, fmt.Sprintf("git push %s: the target is only known when it runs; name the branch", positional[0]))
			}
		}
		for _, spec := range targets {
			target := strings.TrimPrefix(spec, "+")
			if _, after, found := strings.Cut(target, ":"); found {
				target = after
			}
			if target == "HEAD" {
				target = branch
			}
			if hasAnyCritical(policy) && unresolvedTarget(target) {
				findings = append(findings, fmt.Sprintf("git push %s: the target is only known when it runs; name the branch", spec))
				continue
			}
			if strings.Contains(target, "*") {
				if hasAnyCritical(policy) {
					findings = append(findings, fmt.Sprintf("git push %s: a wildcard refspec reaches every ref, including a critical one; open a pull request instead", spec))
				}
				continue
			}
			if policy.IsCritical(target) {
				verb := "push to"
				if deletes {
					verb = "delete"
				}
				if deletes || normalizeMode(policy.Mode) != ModeUnsafe {
					findings = append(findings, fmt.Sprintf("git %s %s: open a pull request instead", verb, target))
				}
			}
		}
	case "pull":
		if policy.IsCritical(branch) && normalizeMode(policy.Mode) == ModeSafe {
			findings = append(findings, fmt.Sprintf("git pull on %s: a critical ref is watched too", branch))
		}
	case "commit":
		if policy.IsCritical(branch) && normalizeMode(policy.Mode) != ModeUnsafe {
			findings = append(findings, fmt.Sprintf("git commit on %s: create a branch first", branch))
		}
		if policy.HasTrailer(normalizeMessage(commitMessage(rest, cwd, stdin))) {
			findings = append(findings, "commit message carries a co-author or generated-by trailer")
		}
	case "merge":
		if policy.IsCritical(branch) && normalizeMode(policy.Mode) != ModeUnsafe {
			findings = append(findings, fmt.Sprintf("git merge on %s: landing is the human's merge button", branch))
		}
		if policy.HasTrailer(normalizeMessage(commitMessage(rest, cwd, stdin))) {
			findings = append(findings, "commit message carries a co-author or generated-by trailer")
		}
	case "branch":
		deleting, forcing, moving := false, false, false
		for _, arg := range rest {
			switch {
			case arg == "--delete" || longFlagPrefix(arg, "delete"):
				deleting = true
			case arg == "--force" || longFlagPrefix(arg, "force"):
				forcing = true
			case arg == "--move" || longFlagPrefix(arg, "move") || arg == "--copy" || longFlagPrefix(arg, "copy"):
				moving = true
			case strings.HasPrefix(arg, "--"):
				// a long flag with no bearing on deleting, forcing, or moving.
			case strings.HasPrefix(arg, "-"):
				// git bundles short flags, so -qf and -f are the same force.
				for _, r := range arg[1:] {
					switch r {
					case 'd', 'D':
						deleting = true
					case 'f':
						forcing = true
					case 'm', 'M', 'c', 'C':
						moving = true
					}
				}
			}
		}
		if !deleting && !forcing && !moving {
			return findings, branch
		}
		var positional []string
		for _, arg := range rest {
			if !strings.HasPrefix(arg, "-") {
				positional = append(positional, arg)
			}
		}
		switch {
		case deleting:
			for _, arg := range positional {
				switch {
				case policy.IsCritical(arg):
					findings = append(findings, fmt.Sprintf("git branch --delete %s: a critical ref is never deleted", arg))
				case hasAnyCritical(policy) && unresolvedTarget(arg):
					findings = append(findings, fmt.Sprintf("git branch --delete %s: the target is only known when it runs; name the branch", arg))
				}
			}
		case moving:
			// Renaming a critical ref away or onto one both move it; one positional renames the current branch.
			if len(positional) == 1 && policy.IsCritical(branch) {
				findings = append(findings, fmt.Sprintf("git branch %s: a critical ref is never moved by hand", branch))
			}
			for _, arg := range positional {
				switch {
				case policy.IsCritical(arg):
					findings = append(findings, fmt.Sprintf("git branch %s: a critical ref is never moved by hand", arg))
				case hasAnyCritical(policy) && unresolvedTarget(arg):
					findings = append(findings, fmt.Sprintf("git branch %s: the target is only known when it runs; name the branch", arg))
				}
			}
		case forcing:
			// -f without -m/-c names the ref being moved as its first positional; a later one is only a start point.
			if len(positional) > 0 && policy.IsCritical(positional[0]) {
				findings = append(findings, fmt.Sprintf("git branch -f %s: a critical ref is never moved by hand", positional[0]))
			}
			// An unresolved word anywhere means the real argument positions are not known either.
			for _, arg := range positional {
				if hasAnyCritical(policy) && unresolvedTarget(arg) {
					findings = append(findings, fmt.Sprintf("git branch -f %s: the target is only known when it runs; name the branch", arg))
					break
				}
			}
		}
	case "update-ref":
		for _, arg := range rest {
			if policy.IsCritical(arg) {
				findings = append(findings, fmt.Sprintf("git update-ref %s: a critical ref is never moved by hand", arg))
			}
		}
	case "switch", "checkout":
		if target, create, ok := switchTarget(rest); ok {
			if previousBranch(target) && hasAnyCritical(policy) {
				findings = append(findings, fmt.Sprintf("git %s %s: the previous branch is not tracked; name the branch", sub, target))
			}
			if !create && policy.IsCritical(target) && normalizeMode(policy.Mode) == ModeSafe {
				findings = append(findings, fmt.Sprintf("git %s %s: a switch onto a critical ref is watched too", sub, target))
			}
			branch = target
		}
	case "config":
		if !hasReadFlag(rest) {
			findings = append(findings, ".git/config is a host or toolkit config; the guard owns it")
		}
	case "remote":
		if !readOnlyGitRemote(rest) {
			findings = append(findings, ".git/config is a host or toolkit config; the guard owns it")
		}
	}
	return findings, branch
}

// isForceFlag reports whether a push option rewrites the remote's history.
func isForceFlag(arg string) bool {
	if arg == "-f" || arg == "--force" || arg == "--force-if-includes" {
		return true
	}
	if strings.HasPrefix(arg, "--force-with-lease") {
		return true
	}
	// A short cluster such as -fu carries the same force.
	return strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(arg[1:], "f")
}

// commitMessage composes the text of a commit or merge's own message: every -m paragraph, an
// -F file's content, and a --trailer's raw key=value line, in the order git reads them.
func commitMessage(rest []string, cwd, stdin string) string {
	var parts []string
	for index := 0; index < len(rest); index++ {
		arg := rest[index]
		switch {
		case arg == "-m" || arg == "--message":
			if index+1 < len(rest) {
				index++
				parts = append(parts, rest[index])
			}
		case strings.HasPrefix(arg, "--message="):
			parts = append(parts, strings.TrimPrefix(arg, "--message="))
		case strings.HasPrefix(arg, "-m") && arg != "-m":
			parts = append(parts, strings.TrimPrefix(arg, "-m"))
		case arg == "-F" || arg == "--file":
			if index+1 < len(rest) {
				index++
				parts = append(parts, readMessageFile(rest[index], cwd, stdin))
			}
		case strings.HasPrefix(arg, "--file="):
			parts = append(parts, readMessageFile(strings.TrimPrefix(arg, "--file="), cwd, stdin))
		case arg == "--trailer":
			if index+1 < len(rest) {
				index++
				parts = append(parts, rest[index])
			}
		case strings.HasPrefix(arg, "--trailer="):
			parts = append(parts, strings.TrimPrefix(arg, "--trailer="))
		}
	}
	return strings.Join(parts, "\n\n")
}

// messageBreakRe is a ; or && a message smuggles a trailer past, read as a line break instead.
var messageBreakRe = regexp.MustCompile(`\s*(?:&&|;)\s*`)

// normalizeMessage turns a message's own ; and && into line breaks before the trailer check.
func normalizeMessage(text string) string {
	return messageBreakRe.ReplaceAllString(text, "\n")
}

// readMessageFile reads a commit message file relative to cwd, or the heredoc a - or /dev/stdin names.
func readMessageFile(path, cwd, stdin string) string {
	if path == "-" || path == "/dev/stdin" {
		return stdin
	}
	resolved := expandHome(path)
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(cwd, resolved)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return ""
	}
	return string(data)
}

// aliasValue returns the git alias a -c option set for a subcommand name, or the empty string.
func aliasValue(configs []string, sub string) string {
	for _, entry := range configs {
		if name, value, found := strings.Cut(entry, "="); found && name == "alias."+sub {
			return value
		}
	}
	return ""
}

// hasAnyCritical reports whether the policy protects any ref at all.
func hasAnyCritical(policy Policy) bool {
	return len(policy.CriticalRefs) > 0
}

// longFlagPrefix reports whether arg abbreviates a long flag, as git accepts any unambiguous prefix.
func longFlagPrefix(arg, name string) bool {
	body, ok := strings.CutPrefix(arg, "--")
	return ok && len(body) >= 1 && strings.HasPrefix(name, body)
}

// unresolvedTarget reports whether a push target still holds text a shell only fills in when it
// runs: an xargs placeholder, a command substitution, a backtick, or an unresolved variable.
func unresolvedTarget(target string) bool {
	return strings.Contains(target, "{}") || strings.Contains(target, "$(") ||
		strings.Contains(target, "`") || unresolvedVarRe.MatchString(target)
}

// switchTarget finds the ref a switch or checkout targets, and whether it creates a fresh one;
// a ref before -- and paths is a restore, and -B, -C, and --force-create reset, not create.
func switchTarget(rest []string) (target string, create bool, ok bool) {
	for index, arg := range rest {
		if arg == "--" {
			return "", false, false
		}
		if arg == "-b" || arg == "-c" || arg == "--create" || arg == "--orphan" {
			return switchOperand(rest, index+1, true)
		}
		if arg == "-B" || arg == "-C" || arg == "--force-create" {
			return switchOperand(rest, index+1, false)
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			if restoresPaths(rest[index+1:]) {
				return "", false, false
			}
			return arg, false, true
		}
	}
	return "", false, false
}

// switchOperand is the ref after a create flag, or nothing when the flag ends the command.
func switchOperand(rest []string, index int, create bool) (string, bool, bool) {
	if index < len(rest) {
		return rest[index], create, true
	}
	return "", false, false
}

// restoresPaths reports whether the tokens after a ref are -- and at least one path.
func restoresPaths(rest []string) bool {
	return len(rest) >= 2 && rest[0] == "--"
}

// previousBranch reports whether a switch target names an earlier branch, as - and @{-1} do.
func previousBranch(target string) bool {
	return target == "-" || strings.HasPrefix(target, "@{-")
}

// readOnlyGitCommands never move a ref, so running them in another checkout is harmless.
var readOnlyGitCommands = map[string]bool{
	"status": true, "log": true, "diff": true, "show": true, "rev-parse": true, "rev-list": true,
	"ls-files": true, "ls-tree": true, "cat-file": true, "blame": true, "grep": true,
	"describe": true, "shortlog": true, "merge-base": true, "name-rev": true,
}

// readOnlyGit reports whether a git subcommand only reads, including a branch or worktree listing.
func readOnlyGit(sub string, rest []string) bool {
	switch sub {
	case "branch":
		for _, arg := range rest {
			if !strings.HasPrefix(arg, "-") || !branchListFlags[arg] {
				return false
			}
		}
		return true
	case "worktree":
		return len(rest) > 0 && rest[0] == "list"
	case "config":
		return hasReadFlag(rest)
	case "remote":
		return readOnlyGitRemote(rest)
	}
	return readOnlyGitCommands[sub]
}

// remoteReadCommands are the git remote subcommands that only read.
var remoteReadCommands = map[string]bool{"show": true, "get-url": true}

// readOnlyGitRemote reports whether a git remote call only reads: bare, -v, show, or get-url.
func readOnlyGitRemote(rest []string) bool {
	for _, arg := range rest {
		if arg == "-v" || arg == "--verbose" {
			continue
		}
		return remoteReadCommands[arg]
	}
	return true
}

// branchListFlags are the git branch flags that only list.
var branchListFlags = map[string]bool{
	"--list": true, "-a": true, "--all": true, "-r": true, "--remotes": true, "-v": true, "-vv": true, "--show-current": true,
}

// hasReadFlag reports whether a git config call only reads, never writes.
func hasReadFlag(rest []string) bool {
	for _, arg := range rest {
		if configReadFlags[arg] {
			return true
		}
	}
	return false
}
