//go:build unix

package run

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"komodo/internal/pr"
	"komodo/internal/proc"
)

func TestAnInterruptStopsTheRunningSessionBeforeTheConductorExits(t *testing.T) {
	root := driveRepo(t)
	setupDriveFakeClaude(t)
	marker := filepath.Join(t.TempDir(), "session-started")
	t.Setenv("FAKE_HANG_MARKER", marker)
	sleep := fmt.Sprintf("61.%d", os.Getpid())
	t.Setenv("FAKE_HANG_SLEEP", sleep)
	client := &pr.Client{Run: func(string, ...string) (string, error) { return "[]", nil }}

	done := make(chan error, 1)
	go func() {
		_, err := Drive(Options{Root: root, Target: "TG-40.1", PR: client})
		done <- err
	}()
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the fake session never started")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the drive never returned after an interrupt")
	}
	for end := time.Now().Add(3 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		// pgrep exits zero only while a process still matches.
		if out := proc.Exec("", 5*time.Second, "pgrep", "-f", "^sleep "+sleep+"$"); !out.OK() {
			return
		}
		if time.Now().After(end) {
			t.Fatal("the session outlived its conductor after an interrupt")
		}
	}
}
