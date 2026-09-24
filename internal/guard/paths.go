package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// isConditionalWriter reports whether sed, perl, or find writes a path in this invocation.
func isConditionalWriter(name string, kept []string) bool {
	switch name {
	case "sed", "perl":
		return hasPrefixToken(kept[1:], "-i")
	case "find":
		return containsToken(kept[1:], "-delete")
	}
	return false
}

// hasPrefixToken reports whether any token carries the given prefix.
func hasPrefixToken(tokens []string, prefix string) bool {
	for _, token := range tokens {
		if strings.HasPrefix(token, prefix) {
			return true
		}
	}
	return false
}

// containsToken reports whether a token equals the given value.
func containsToken(tokens []string, value string) bool {
	for _, token := range tokens {
		if token == value {
			return true
		}
	}
	return false
}

// changeDir resolves a cd's target against the current directory, so a later write judges
// correctly against wherever the chain now stands, even outside the worktree root.
func changeDir(kept []string, cwd string) string {
	var operands []string
	for _, arg := range kept[1:] {
		if len(operands) == 0 && strings.HasPrefix(arg, "-") && arg != "-" {
			continue
		}
		operands = append(operands, arg)
	}
	if len(operands) > 1 {
		return unresolvedDir
	}
	target := ""
	if len(operands) == 1 {
		target = operands[0]
	}
	if target == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return unresolvedDir
		}
		return home
	}
	if unresolvedVarRe.MatchString(target) || otherHomeRe.MatchString(target) || target == "-" {
		return unresolvedDir
	}
	resolved := expandHome(target)
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(cwd, resolved)
	}
	return filepath.Clean(resolved)
}

// writerPaths checks the paths a path-writing command actually writes to: every argument,
// unless the command only writes its last, since cp, mv, ln, and install read every other one.
func writerPaths(name string, kept []string, cwd, root string, policy Policy) []string {
	var findings []string
	for _, token := range writerTargets(name, kept) {
		findings = append(findings, pathFindings(token, cwd, root, policy)...)
	}
	return findings
}

// writerTargets lists the paths a writer command writes, past sed's and perl's script argument.
func writerTargets(name string, kept []string) []string {
	var targets []string
	editor := name == "sed" || name == "perl"
	script := false
	for index := 1; index < len(kept); index++ {
		token := kept[index]
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "-") {
			// sed -e and -f, and perl's -e in any cluster, take the script as their value.
			if editor && scriptFlag(name, token) {
				script = true
				index++
			}
			continue
		}
		if editor && !script {
			script = true
			continue
		}
		targets = append(targets, token)
	}
	if destOnlyWriters[name] && len(targets) > 1 {
		targets = targets[len(targets)-1:]
	}
	return targets
}

// ddPaths checks dd's of= target, the only argument dd writes to.
func ddPaths(kept []string, cwd, root string, policy Policy) []string {
	var findings []string
	for _, token := range kept[1:] {
		if value, ok := strings.CutPrefix(token, "of="); ok {
			findings = append(findings, pathFindings(value, cwd, root, policy)...)
		}
	}
	return findings
}

// isAllowedWrite reports whether a write may always land here: the null device, or a scratch
// file placed directly in a system temp directory, which cost real time to deny.
func isAllowedWrite(path string) bool {
	compare := path
	if foldsCase() {
		compare = strings.ToLower(compare)
	}
	if compare == os.DevNull {
		return true
	}
	for _, dir := range []string{os.TempDir(), "/tmp", "/private/tmp"} {
		dir = strings.TrimRight(dir, string(filepath.Separator))
		if dir == "" {
			continue
		}
		if foldsCase() {
			dir = strings.ToLower(dir)
		}
		if compare == dir || strings.HasPrefix(compare, dir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// unresolvedDir stands for the working directory after a cd whose target the guard cannot resolve.
const unresolvedDir = "\x00unresolved"

// pathFindings refuses a write that leaves the worktree, lands on a config the hosts own, or
// holds a variable or another user's home the guard cannot resolve, relative to cwd.
func pathFindings(path, cwd, root string, policy Policy) []string {
	if path == "" {
		return nil
	}
	if unresolvedVarRe.MatchString(path) || otherHomeRe.MatchString(path) {
		return []string{fmt.Sprintf("%s holds an unresolved variable or another user's home; the guard cannot judge it", path)}
	}
	resolved := expandHome(path)
	if !filepath.IsAbs(resolved) {
		if cwd == unresolvedDir {
			return []string{fmt.Sprintf("%s follows a cd the guard cannot resolve; use an absolute path", path)}
		}
		resolved = filepath.Join(cwd, resolved)
	}
	resolved = filepath.Clean(resolved)
	var findings []string
	if policy.IsConfigPath(resolved, root) {
		findings = append(findings, fmt.Sprintf("%s is a host or toolkit config; the guard owns it", path))
		return findings
	}
	if isAllowedWrite(resolved) {
		return nil
	}
	if root == "" {
		return findings
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || strings.HasPrefix(relative, "..") {
		findings = append(findings, fmt.Sprintf("%s is outside the worktree root %s", path, root))
	}
	return findings
}

// scriptFlag reports whether a sed or perl option's next word is the script rather than a file.
func scriptFlag(name, flag string) bool {
	if name == "sed" {
		return flag == "-e" || flag == "-f" || flag == "--expression" || flag == "--file"
	}
	return !strings.HasPrefix(flag, "--") && strings.ContainsAny(flag[1:], "eE")
}
