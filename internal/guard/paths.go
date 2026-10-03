package guard

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"komodo/internal/mount"
)

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

// pathFindings refuses a write on a config the hosts own, and a line session's write outside its
// worktree; a resolvable $VAR expands first, an unresolvable one is judged on its literal text.
func pathFindings(path, cwd, root string, policy Policy) []finding {
	if path == "" {
		return nil
	}
	judged := path
	if expanded, ok := expandVars(path); ok {
		judged = expanded
	}
	resolved := expandHome(judged)
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(cwd, resolved)
	}
	resolved = filepath.Clean(resolved)
	if isHostWritePath(resolved, root) {
		return nil
	}
	if policy.IsConfigPath(resolved, root) {
		return []finding{newFinding("%s is a host or toolkit config; the guard owns it", path)}
	}
	if isAllowedWrite(resolved) {
		return nil
	}
	if root == "" || !IsLineSession() {
		return nil
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || strings.HasPrefix(relative, "..") {
		return []finding{newFinding("%s is outside the worktree root %s", path, root)}
	}
	return nil
}

// isHostWritePath reports whether a resolved path is a write a registered mount declares for root,
// such as its own memory store, allowed outside the worktree with no other config path opened.
func isHostWritePath(resolved, root string) bool {
	if root == "" {
		return false
	}
	compare := filepath.ToSlash(resolved)
	if foldsCase() {
		compare = strings.ToLower(compare)
	}
	home, _ := homeDir()
	return matchesAnyPattern(mount.WritePaths(root), home, compare, compare)
}

// expandVars substitutes every $NAME or ${NAME} reference in path with the session's own
// environment, reporting false when a command substitution or an unset name leaves it unresolved.
func expandVars(path string) (string, bool) {
	if strings.Contains(path, "$(") {
		return path, false
	}
	var out strings.Builder
	runes := []rune(path)
	for index := 0; index < len(runes); index++ {
		if runes[index] != '$' {
			out.WriteRune(runes[index])
			continue
		}
		name, consumed := varName(runes[index+1:])
		if name == "" {
			out.WriteRune(runes[index])
			continue
		}
		value, ok := os.LookupEnv(name)
		if !ok {
			return path, false
		}
		out.WriteString(value)
		index += consumed
	}
	return out.String(), true
}

// varName reads a $NAME reference's name right after the $, braced or bare, and how many runes
// of the input it consumed; "" names none.
func varName(runes []rune) (string, int) {
	if len(runes) > 0 && runes[0] == '{' {
		for index := 1; index < len(runes); index++ {
			if runes[index] == '}' {
				return string(runes[1:index]), index + 1
			}
		}
		return "", 0
	}
	index := 0
	for index < len(runes) && (runes[index] == '_' || unicode.IsLetter(runes[index]) ||
		(index > 0 && unicode.IsDigit(runes[index]))) {
		index++
	}
	return string(runes[:index]), index
}

// expandHome replaces a leading tilde with the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := homeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}
