package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// pathFindings refuses a write on a config the hosts own, and a line session's write outside its worktree.
func pathFindings(path, cwd, root string, policy Policy) []string {
	if path == "" {
		return nil
	}
	resolved := expandHome(path)
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(cwd, resolved)
	}
	resolved = filepath.Clean(resolved)
	if isHostWritePath(resolved, root) {
		return nil
	}
	if policy.IsConfigPath(resolved, root) {
		return []string{fmt.Sprintf("%s is a host or toolkit config; the guard owns it", path)}
	}
	if isAllowedWrite(resolved) {
		return nil
	}
	if root == "" || !IsLineSession() {
		return nil
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || strings.HasPrefix(relative, "..") {
		return []string{fmt.Sprintf("%s is outside the worktree root %s", path, root)}
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
