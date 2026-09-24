package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// gitFindings refuses the git operations that touch a critical ref, and reports the branch after the call.
func gitFindings(tokens []string, branch, cwd, root string, policy Policy, source *messageSource) ([]string, string) {
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
		if redirectURLConfigRe.MatchString(name) && normalizeMode(policy.Mode) != ModeUnsafe {
			findings = append(findings, fmt.Sprintf("git -c %s: a config write sends git to another URL; push to origin", entry))
		}
		if hooksPathConfigRe.MatchString(name) {
			findings = append(findings, fmt.Sprintf("git -c %s: a config write points git at other hooks; the guard owns .git/config", entry))
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
	if noVerifyCommands[sub] && normalizeMode(policy.Mode) != ModeUnsafe && hasNoVerify(sub, rest) {
		findings = append(findings, fmt.Sprintf("git %s --no-verify skips the gate; fix what it reports instead", sub))
	}
	switch sub {
	case "push":
		var positional []string
		deletes := false
		mirrorFlag := ""
		repoFlag := false
		for index := 0; index < len(rest); index++ {
			arg := rest[index]
			if arg == "--repo" || strings.HasPrefix(arg, "--repo=") {
				repoFlag = true
				if arg == "--repo" {
					index++
				}
				continue
			}
			if pushValueFlags[arg] {
				index++
				continue
			}
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
		if normalizeMode(policy.Mode) != ModeUnsafe && pushesToURL(rest, positional, cwd, root, repoFlag) {
			findings = append(findings, pushURLFinding)
		}
		targets := positional
		switch {
		case repoFlag && len(positional) > 0:
			// --repo names the repository, so every positional is a refspec.
		case len(positional) > 1:
			targets = positional[1:]
		default:
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
			if hasAnyCritical(policy) && unresolvedTarget(spec) {
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
		findings = append(findings, messageFindings(commitMessage(rest, source), source, policy)...)
	case "merge":
		if policy.IsCritical(branch) && normalizeMode(policy.Mode) != ModeUnsafe {
			findings = append(findings, fmt.Sprintf("git merge on %s: landing is the human's merge button", branch))
		}
		findings = append(findings, messageFindings(commitMessage(rest, source), source, policy)...)
	case "branch":
		deleting, forcing, moving, renaming := false, false, false, false
		for _, arg := range rest {
			switch {
			case arg == "--delete" || longFlagPrefix(arg, "delete"):
				deleting = true
			case arg == "--force" || longFlagPrefix(arg, "force"):
				forcing = true
			case arg == "--move" || longFlagPrefix(arg, "move"):
				moving, renaming = true, true
			case arg == "--copy" || longFlagPrefix(arg, "copy"):
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
					case 'm', 'M':
						moving, renaming = true, true
					case 'c', 'C':
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
			// A rename moves a critical ref either way; one positional renames the current branch, a copy does not.
			if renaming && len(positional) == 1 && policy.IsCritical(branch) {
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
			if hasAnyCritical(policy) && unresolvedTarget(target) {
				findings = append(findings, fmt.Sprintf("git %s %s: the target is only known when it runs; name the branch", sub, target))
				break
			}
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

// redirectURLConfigRe matches a -c key that points a remote, or every matching URL, somewhere else.
var redirectURLConfigRe = regexp.MustCompile(`(?i)^(remote\.[^.]+\.url|url\..+\.(insteadof|pushinsteadof))$`)

// pushValueFlags are push's options whose value is the next word, never the repository or a refspec.
var pushValueFlags = map[string]bool{
	"-o": true, "--push-option": true, "--receive-pack": true, "--exec": true,
}

// hooksPathConfigRe matches a -c or --config-env key that points git at another hooks directory.
var hooksPathConfigRe = regexp.MustCompile(`(?i)^core\.hookspath$`)

// noVerifyCommands are the git subcommands whose --no-verify skips the gate's own hook.
var noVerifyCommands = map[string]bool{
	"commit": true, "merge": true, "push": true, "rebase": true, "am": true, "cherry-pick": true,
}

// hasNoVerify reports whether rest skips hooks: --no-verify or a prefix git resolves to it, or
// commit's -n alone or in a short cluster before any option that takes a value.
func hasNoVerify(sub string, rest []string) bool {
	for index := 0; index < len(rest); index++ {
		arg := rest[index]
		if len(arg) >= len("--no-veri") && strings.HasPrefix("--no-verify", arg) {
			return true
		}
		if sub != "commit" || !strings.HasPrefix(arg, "-") {
			continue
		}
		if strings.HasPrefix(arg, "--") {
			if commitLongValues[arg] {
				index++
			}
			continue
		}
		for position, letter := range arg[1:] {
			if letter == 'n' {
				return true
			}
			if commitValueLetters[letter] {
				// a bare -m or -F takes the next word, which is never a flag cluster.
				if position == len(arg)-2 && commitNextWordLetters[letter] {
					index++
				}
				break
			}
		}
	}
	return false
}

// commitLongValues are commit's long options whose value is the next word.
var commitLongValues = map[string]bool{
	"--message": true, "--file": true, "--reuse-message": true, "--reedit-message": true, "--template": true,
	"--author": true, "--date": true, "--cleanup": true, "--fixup": true, "--squash": true, "--trailer": true,
}

// commitNextWordLetters take the next word when bare; -u and -S take only a glued value.
var commitNextWordLetters = map[rune]bool{'m': true, 'F': true, 'C': true, 'c': true, 't': true}

// commitValueLetters are commit's short options that take the rest of the cluster as their value.
var commitValueLetters = map[rune]bool{'m': true, 'F': true, 'C': true, 'c': true, 't': true, 'u': true, 'S': true}

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

// pushURLFinding is reported when a push destination bypasses the remote the line configured.
const pushURLFinding = "git push to a URL skips the remote the line configured; push to origin"

// scpForm reports whether git reads a destination as scp form: its first colon comes before any
// slash. A single letter, a colon, and a slash or backslash is a Windows drive instead.
func scpForm(dest string) bool {
	colon := strings.IndexByte(dest, ':')
	if colon <= 0 {
		return false
	}
	if slash := strings.IndexByte(dest, '/'); slash >= 0 && slash < colon {
		return false
	}
	if colon == 1 && len(dest) > 2 && (dest[2] == '/' || dest[2] == '\\') {
		return false
	}
	return true
}

// pushesToURL reports whether a push names its repository as a URL, through --repo or the first
// positional, or through a substitution or variable whose value is only known when it runs.
func pushesToURL(rest, positional []string, cwd, root string, repoFlag bool) bool {
	for index, arg := range rest {
		if value, ok := strings.CutPrefix(arg, "--repo="); ok && isPushURL(value, cwd, root) {
			return true
		}
		if arg == "--repo" && index+1 < len(rest) && isPushURL(rest[index+1], cwd, root) {
			return true
		}
	}
	if len(positional) > 1 && unresolvedTarget(positional[0]) {
		return true
	}
	return len(positional) > 0 && !repoFlag && isPushURL(positional[0], cwd, root)
}

// isPushURL reports whether a push destination is a URL, scp-style remote, or a path to a repository
// outside the worktree root: one with a slash or a .git suffix, or a bare name that is a directory.
func isPushURL(dest, cwd, root string) bool {
	if strings.Contains(dest, "://") || scpForm(dest) {
		return true
	}
	resolved := dest
	if !filepath.IsAbs(resolved) {
		if cwd == unresolvedDir {
			return strings.Contains(dest, "/")
		}
		resolved = filepath.Join(cwd, resolved)
	}
	resolved = filepath.Clean(resolved)
	if !strings.Contains(dest, "/") && !strings.HasSuffix(dest, ".git") {
		if info, err := os.Stat(resolved); err != nil || !info.IsDir() {
			return false
		}
	}
	if root == "" {
		root = cwd
	}
	root = filepath.Clean(root)
	return resolved != root && !strings.HasPrefix(resolved, root+string(filepath.Separator))
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
