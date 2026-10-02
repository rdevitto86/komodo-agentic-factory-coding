package mount

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

// hookGOOS is the platform Publish swaps for; tests set it to prove the Windows path.
var hookGOOS = runtime.GOOS

// HookPath is the one binary every rendered hook runs: ~/.komodo/bin/komodo, with .exe on Windows.
func HookPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	name := "komodo"
	if hookGOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(home, ".komodo", "bin", name), nil
}

// Publish makes HookPath hold binary's bytes and returns HookPath, or binary itself when it cannot.
func Publish(binary string) string {
	data, err := os.ReadFile(binary)
	if err != nil {
		return binary
	}
	target, err := HookPath()
	if err != nil {
		return binary
	}
	if current, err := os.ReadFile(target); err == nil && bytes.Equal(current, data) {
		return target
	}
	if err := replaceHook(target, data); err != nil {
		return binary
	}
	return target
}

// replaceHook writes data beside target and renames it into place, so a running hook never reads half a file.
// Windows refuses to replace a running executable but lets it be renamed, so the old one moves aside first.
func replaceHook(target string, data []byte) error {
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".komodo-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temp.Name(), 0o755); err != nil {
		return err
	}
	if hookGOOS == "windows" {
		aside := filepath.Join(dir, "komodo.old.exe")
		_ = os.Remove(aside)
		if err := os.Rename(target, aside); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(temp.Name(), target)
}

// hookCopy names a content-hashed hook copy an earlier render wrote, or a binary Publish moved aside.
var hookCopy = regexp.MustCompile(`^komodo-[0-9a-f]{12}$|^komodo\.old\.exe$`)

// PruneHookCopies deletes every hashed hook copy and moved-aside binary under ~/.komodo/bin that keep does not
// name, returning what it removed.
func PruneHookCopies(keep map[string]bool) ([]string, error) {
	target, err := HookPath()
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(target)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, entry := range entries {
		if entry.IsDir() || !hookCopy.MatchString(entry.Name()) || keep[filepath.Join(dir, entry.Name())] {
			continue
		}
		// A copy Windows still holds open stays until a later sweep.
		if path := filepath.Join(dir, entry.Name()); os.Remove(path) == nil {
			removed = append(removed, path)
		}
	}
	return removed, nil
}

// LatestBinary is the newest komodo this machine has for root: the toolkit checkout's own build, else
// the installed release, else the running binary.
func LatestBinary(root string) string {
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	main := MainCheckout(root)
	if isFile(filepath.Join(main, "cmd", "komodo", "main.go")) {
		if built := filepath.Join(main, "bin", "komodo"+exe); isFile(built) {
			return built
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		release := filepath.Join(home, ".komodo", "bin", "komodo-"+runtime.GOOS+"-"+runtime.GOARCH+exe)
		if isFile(release) {
			return release
		}
	}
	binary := BinaryPath()
	if !filepath.IsAbs(binary) {
		binary = filepath.Join(main, binary)
	}
	return binary
}

// isFile reports whether path names a regular file.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
