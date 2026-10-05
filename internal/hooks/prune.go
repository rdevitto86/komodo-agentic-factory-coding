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

// sweepMachineEnv marks a launched sweep for a repo with no state dir: it refreshes the machine and prunes nothing.
const sweepMachineEnv = "KOMODO_SWEEP_MACHINE"

// launch starts the sweep as its own process group at root and returns without waiting; tests swap it.
var launch = func(root string, machine bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "hook", "prune")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), SweepEnv+"=1")
	if machine {
		cmd.Env = append(cmd.Env, sweepMachineEnv+"=1")
	}
	proc.Group(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// sweepDir is where a sweep keeps its log and lock: ~/.komodo for a machine-only sweep, else root's state dir.
func sweepDir(root string, machine bool) string {
	if !machine {
		return filepath.Join(root, line.StateDir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, line.StateDir)
}

// startSweep is the prune hook: it launches the background sweep and names the last one's failure.
func startSweep(_ context.Context, in Input) (Outcome, error) {
	root := mount.MainCheckout(in.Root)
	out := Outcome{Verdict: Allow}
	info, err := os.Stat(filepath.Join(root, line.StateDir))
	machine := err != nil || !info.IsDir()
	dir := sweepDir(root, machine)
	if dir == "" {
		return out, nil
	}
	path := filepath.Join(dir, sweepLog)
	if data, err := os.ReadFile(path); err == nil && strings.HasPrefix(string(data), sweepFailed) {
		first, _, _ := strings.Cut(string(data), "\n")
		out = Outcome{Verdict: Inform, Message: "The last background worktree sweep " + first + "; see " + path + "."}
	}
	return out, launch(root, machine)
}

// Sweep refreshes the machine and, unless launched machine-only, prunes root, capped and under the lock; a held
// lock skips it.
func Sweep(root string) {
	repo := os.Getenv(sweepMachineEnv) != "1"
	dir := sweepDir(root, !repo)
	if dir == "" {
		return
	}
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
		var done []string
		if repo {
			var err error
			if done, err = work(root, line.DefaultBase(root), true); err != nil {
				result <- sweepFailed + " " + err.Error() + "\n"
				return
			}
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
