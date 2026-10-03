package install

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// OnWindowsDrive reports whether root sits on the Windows filesystem under WSL, where git and builds
// crawl and file modes break; procVersion is the contents of /proc/version.
func OnWindowsDrive(root, procVersion string) bool {
	return strings.Contains(strings.ToLower(procVersion), "microsoft") && strings.HasPrefix(filepath.ToSlash(root), "/mnt/")
}

// CheckFilesystem refuses a repo on the Windows filesystem under WSL, naming where to clone it instead.
func CheckFilesystem(root string) error {
	if runtime.GOOS != "linux" || root == "" {
		return nil
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil || !OnWindowsDrive(root, string(data)) {
		return nil
	}
	return fmt.Errorf("%s is on the Windows filesystem; clone it under your Linux home, such as ~/src, and run this again", root)
}

// LinkOnPath links komodo in dir to binary, so a person runs komodo by name, and returns a hint when dir
// is not on PATH. Windows has no unprivileged symlink, so there the binary's own directory goes on PATH.
func LinkOnPath(binary, dir, path string) (hint string, err error) {
	if runtime.GOOS == "windows" {
		dir = filepath.Dir(binary)
	} else {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		link := filepath.Join(dir, "komodo")
		if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err := os.Symlink(binary, link); err != nil {
			return "", err
		}
	}
	for _, entry := range filepath.SplitList(path) {
		if filepath.Clean(entry) == filepath.Clean(dir) {
			return "", nil
		}
	}
	return "add " + dir + " to PATH to run komodo by name", nil
}

// PathDir is where komodo is linked for a person to run: KOMODO_BIN_DIR, else ~/.local/bin.
func PathDir(home string) string {
	if dir := os.Getenv("KOMODO_BIN_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".local", "bin")
}
