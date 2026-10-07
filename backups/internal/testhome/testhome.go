// Package testhome points a test binary's home and temp directories at a fresh temp root, so no test
// writes the real ones and nothing a test leaves behind outlives the binary.
package testhome

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// goKeys are the Go settings that default under the home directory; they keep their real values.
var goKeys = []string{"GOCACHE", "GOMODCACHE", "GOPATH", "GOENV"}

// Isolate points HOME, USERPROFILE and the temp variables into a fresh root, pins Go's caches where
// they were, and returns the func that removes the root.
func Isolate() func() {
	kept := map[string]string{}
	for _, key := range goKeys {
		if out, err := exec.Command("go", "env", key).Output(); err == nil {
			kept[key] = strings.TrimSpace(string(out))
		}
	}
	root, err := os.MkdirTemp("", "komodo-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testhome:", err)
		os.Exit(1)
	}
	home, temp := filepath.Join(root, "home"), filepath.Join(root, "tmp")
	for _, dir := range []string{home, temp} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			fmt.Fprintln(os.Stderr, "testhome:", err)
			os.Exit(1)
		}
	}
	for key, value := range kept {
		if value != "" && value != "off" {
			_ = os.Setenv(key, value)
		}
	}
	for _, key := range []string{"HOME", "USERPROFILE"} {
		_ = os.Setenv(key, home)
	}
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		_ = os.Setenv(key, temp)
	}
	return func() { _ = os.RemoveAll(root) }
}
