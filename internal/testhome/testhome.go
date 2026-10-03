// Package testhome points a test binary's home directory at a temp dir, so no test writes the real one.
package testhome

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// goKeys are the Go settings that default under the home directory; they keep their real values.
var goKeys = []string{"GOCACHE", "GOMODCACHE", "GOPATH", "GOENV"}

// Isolate sets HOME and USERPROFILE to a fresh temp dir, pins Go's caches where they were, and
// returns the func that removes the temp dir.
func Isolate() func() {
	kept := map[string]string{}
	for _, key := range goKeys {
		if out, err := exec.Command("go", "env", key).Output(); err == nil {
			kept[key] = strings.TrimSpace(string(out))
		}
	}
	home, err := os.MkdirTemp("", "komodo-test-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testhome:", err)
		os.Exit(1)
	}
	for key, value := range kept {
		if value != "" && value != "off" {
			_ = os.Setenv(key, value)
		}
	}
	_ = os.Setenv("HOME", home)
	_ = os.Setenv("USERPROFILE", home)
	return func() { _ = os.RemoveAll(home) }
}
