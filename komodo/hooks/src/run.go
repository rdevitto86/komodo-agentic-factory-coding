package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitOutput runs git in dir and returns trimmed stdout, or empty when the command fails.
func gitOutput(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitLines runs git in dir and returns its non-empty output lines.
func gitLines(dir string, args ...string) []string {
	var kept []string
	for _, line := range strings.Split(gitOutput(dir, args...), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			kept = append(kept, line)
		}
	}
	return kept
}

// findPython returns the argv prefix for a python interpreter on PATH, or nil when none is installed.
func findPython() []string {
	for _, name := range []string{"python3", "python"} {
		if path, err := exec.LookPath(name); err == nil {
			return []string{path}
		}
	}
	if path, err := exec.LookPath("py"); err == nil {
		return []string{path, "-3"}
	}
	return nil
}

// toolkitRoot walks up from this binary to the checkout holding the komodo package, so the lint can import it.
func toolkitRoot() string {
	if root := os.Getenv("KOMODO_ROOT"); root != "" {
		return root
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	for {
		if _, err := os.Stat(filepath.Join(dir, "komodo", "__main__.py")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// blocked prints every problem under a hook's name and exits non-zero.
func blocked(hook string, problems []string) {
	fmt.Fprintf(os.Stderr, "%s: blocked\n", hook)
	for _, problem := range problems {
		fmt.Fprintf(os.Stderr, "  %s\n", problem)
	}
	os.Exit(1)
}
