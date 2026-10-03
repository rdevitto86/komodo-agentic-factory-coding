package hooks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/doctor"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/proc"
)

// SweepEnv marks the detached process a session's start launches to run the prune sweep itself.
const SweepEnv = "KOMODO_SWEEP"

// The sweep's record and its one-at-a-time lock, both under the main checkout's state dir.
const (
	sweepLog  = "prune.log"
	sweepLock = "prune.lock"
)

// sweepFailed opens the log of a sweep that failed, which the next session's start reports.
const sweepFailed = "failed:"

// sweepLimit is how long one background sweep runs before it stops itself; tests shorten it.
var sweepLimit = 2 * time.Minute

// prune is the sweep's work; tests swap it.
var prune = doctor.Prune

// Refresh publishes the newest binary and re-renders stale layers before the prune; nil skips it.
var Refresh func(root string) ([]string, error)

// launch starts the sweep as its own process group at root and returns without waiting; tests swap it.
var launch = func(root string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "hook", "prune")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), SweepEnv+"=1")
	proc.Group(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// startSweep is the prune hook: in a komodo repo, it launches the background sweep and names the last one's failure.
func startSweep(_ context.Context, in Input) (Outcome, error) {
	root := mount.MainCheckout(in.Root)
	out := Outcome{Verdict: Allow}
	if info, err := os.Stat(filepath.Join(root, line.StateDir)); err != nil || !info.IsDir() {
		return out, nil
	}
	if data, err := os.ReadFile(filepath.Join(root, line.StateDir, sweepLog)); err == nil && strings.HasPrefix(string(data), sweepFailed) {
		first, _, _ := strings.Cut(string(data), "\n")
		out = Outcome{Verdict: Inform, Message: "The last background worktree sweep " + first + "; see .komodo/" + sweepLog + "."}
	}
	return out, launch(root)
}

// Sweep runs one capped prune at root under the lock, logging what it did; a held lock skips it.
func Sweep(root string) {
	dir := filepath.Join(root, line.StateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	lock := filepath.Join(dir, sweepLock)
	if !takeLock(lock) {
		return
	}
	result, work := make(chan string, 1), prune
	go func() {
		// The lock outlives a timed-out select below, so a second sweep never starts while this one works.
		defer os.Remove(lock)
		var refreshed []string
		if Refresh != nil {
			var err error
			if refreshed, err = Refresh(root); err != nil {
				result <- sweepFailed + " refreshing the machine: " + err.Error() + "\n"
				return
			}
		}
		done, err := work(root, line.DefaultBase(root), true)
		if err != nil {
			result <- sweepFailed + " " + err.Error() + "\n"
			return
		}
		lines := append(refreshed, done...)
		result <- "swept at " + time.Now().Format(time.RFC3339) + "\n" + strings.Join(lines, "\n") + "\n"
	}()
	var text string
	select {
	case text = <-result:
	case <-time.After(sweepLimit):
		text = sweepFailed + " it ran past " + sweepLimit.String() + " and stopped\n"
	}
	_ = os.WriteFile(filepath.Join(dir, sweepLog), []byte(text), 0o644)
}

// takeLock creates the lock file, first clearing one older than twice sweepLimit; false when another sweep holds it.
func takeLock(path string) bool {
	if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) >= 2*sweepLimit {
		// A rename is atomic, so of two sweeps finding one stale lock only one clears it.
		stale := fmt.Sprintf("%s.%d", path, os.Getpid())
		if os.Rename(path, stale) == nil {
			_ = os.Remove(stale)
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return false
	}
	_ = file.Close()
	return true
}
