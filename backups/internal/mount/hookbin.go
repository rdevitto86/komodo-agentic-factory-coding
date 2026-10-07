package mount

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"komodo/internal/changelog"
)

// hookGOOS is the platform Publish swaps for; tests set it to prove the Windows path.
var hookGOOS = runtime.GOOS

// versionTimeout bounds how long a binary's own version command may run.
const versionTimeout = 5 * time.Second

// BinaryBuild is the version and commit path's own version command prints, or empty strings when it prints none.
func BinaryBuild(path string) (version, commit string) {
	ctx, cancel := context.WithTimeout(context.Background(), versionTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "version").Output()
	if err != nil {
		return "", ""
	}
	// The output reads komodo <version> (<commit>).
	fields := strings.Fields(string(out))
	if len(fields) < 3 || fields[0] != "komodo" {
		return "", ""
	}
	return fields[1], strings.Trim(fields[2], "()")
}

// binaryVersion is the version path's own version command prints, or "" when it prints none; tests swap it.
var binaryVersion = func(path string) string {
	// A test binary would rerun its own tests when asked for its version.
	if strings.HasSuffix(strings.TrimSuffix(filepath.Base(path), ".exe"), ".test") {
		return ""
	}
	version, _ := BinaryBuild(path)
	return version
}

// refusalOut receives the harness Publish prints when it keeps a newer installed binary; tests swap it.
var refusalOut io.Writer = os.Stderr

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
// An installed stamped release newer than binary, or any stamped release over an unstamped binary, stays.
func Publish(binary string) string {
	data, err := os.ReadFile(binary)
	if err != nil {
		return binary
	}
	target, err := HookPath()
	if err != nil {
		return binary
	}
	if current, err := os.ReadFile(target); err == nil {
		if bytes.Equal(current, data) {
			return target
		}
		installed := binaryVersion(target)
		if candidate := binaryVersion(binary); olderThanInstalled(candidate, installed) {
			if !changelog.Valid(candidate) {
				candidate = "an unstamped build"
			}
			fmt.Fprintf(refusalOut, "komodo: kept the installed %s at %s; %s at %s is older\n",
				installed, target, candidate, binary)
			return target
		}
	}
	if err := replaceHook(target, data); err != nil {
		return binary
	}
	return target
}

// olderThanInstalled reports whether candidate must not replace installed: installed is a stamped version
// and candidate is unstamped or sorts before it.
func olderThanInstalled(candidate, installed string) bool {
	if !changelog.Valid(installed) {
		return false
	}
	return !changelog.Valid(candidate) || changelog.Compare(candidate, installed) < 0
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

// LatestBinary is the highest-versioned komodo among root's toolkit build, the installed release and the
// running binary; a tie or an unstamped version keeps that order.
func LatestBinary(root string) string {
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	main := MainCheckout(root)
	var candidates []string
	if isFile(filepath.Join(main, "cmd", "komodo", "main.go")) {
		if built := filepath.Join(main, "bin", "komodo"+exe); isFile(built) {
			candidates = append(candidates, built)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		release := filepath.Join(home, ".komodo", "bin", "komodo-"+runtime.GOOS+"-"+runtime.GOARCH+exe)
		if isFile(release) {
			candidates = append(candidates, release)
		}
	}
	binary := BinaryPath()
	if !filepath.IsAbs(binary) {
		binary = filepath.Join(main, binary)
	}
	candidates = append(candidates, binary)
	best, bestVersion := "", ""
	for _, path := range candidates {
		version := binaryVersion(path)
		newer := !changelog.Valid(bestVersion) || changelog.Compare(version, bestVersion) > 0
		if best == "" || changelog.Valid(version) && newer {
			best, bestVersion = path, version
		}
	}
	return best
}

// isFile reports whether path names a regular file.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
