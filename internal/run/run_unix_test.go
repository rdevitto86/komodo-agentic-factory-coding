//go:build unix

package run

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLaunchKillsTheWholeProcessGroupNotJustTheHost(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh on this machine")
	}
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	script := "sleep 30 & echo $! > " + pidFile + "; wait"
	var out bytes.Buffer
	code, err := launch(Options{
		Root: dir, Budget: 50 * time.Millisecond,
		Stdout: &out, Stderr: &out, Env: []string{"PATH=/usr/bin:/bin"},
	}, "/bin/sh", []string{"-c", script})
	if code != 124 || err == nil {
		t.Fatalf("code = %d, err = %v; want the budget kill", code, err)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("child pid %d outlived its parent, the shell the budget killed", pid)
}
