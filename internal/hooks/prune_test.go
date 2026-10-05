package hooks

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sweepRepo is a git repo whose state dir holds the sweep's log and lock.
func sweepRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// swapPrune replaces the sweep's work for one test, counting its calls.
func swapPrune(t *testing.T, work func() ([]string, error)) *int {
	t.Helper()
	calls := 0
	old := prune
	prune = func(string, string, bool) ([]string, error) { calls++; return work() }
	t.Cleanup(func() { prune = old })
	return &calls
}

// readLog returns the sweep's log under root, empty when none was written.
func readLog(root string) string {
	data, _ := os.ReadFile(filepath.Join(root, ".komodo", sweepLog))
	return string(data)
}

func TestThePruneHookLaunchesTheSweepAtTheMainCheckoutAndAllows(t *testing.T) {
	root := sweepRepo(t)
	var launched []string
	old := launch
	launch = func(at string) error { launched = append(launched, at); return nil }
	t.Cleanup(func() { launch = old })

	out, err := startSweep(context.Background(), Input{Root: root})
	if err != nil || out.Verdict != Allow {
		t.Fatalf("startSweep = %+v, %v; want a silent allow", out, err)
	}
	if len(launched) != 1 || launched[0] != root {
		t.Fatalf("launched = %v, want one sweep at %s", launched, root)
	}
	hook, ok := Lookup("prune")
	if !ok || hook.Event != SessionStart || hook.OnFailure != Skip || len(hook.Sessions) != 1 || hook.Sessions[0] != SessionPrimary {
		t.Fatalf("prune row = %+v, want a primary SessionStart hook that skips on failure", hook)
	}
}

func TestThePruneHookLeavesARepoWithoutKomodoStateAlone(t *testing.T) {
	root := sweepRepo(t)
	if err := os.Remove(filepath.Join(root, ".komodo")); err != nil {
		t.Fatal(err)
	}
	launched := false
	old := launch
	launch = func(string) error { launched = true; return nil }
	t.Cleanup(func() { launch = old })
	if out, err := startSweep(context.Background(), Input{Root: root}); err != nil || out.Verdict != Allow || launched {
		t.Fatalf("startSweep = %+v, %v, launched = %v; want a repo komodo never set up left alone", out, err, launched)
	}
}

func TestThePruneHookNamesTheLastSweepsFailure(t *testing.T) {
	root := sweepRepo(t)
	old := launch
	launch = func(string) error { return nil }
	t.Cleanup(func() { launch = old })
	if err := os.WriteFile(filepath.Join(root, ".komodo", sweepLog), []byte("failed: boom\nmore\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := startSweep(context.Background(), Input{Root: root})
	if err != nil || out.Verdict != Inform || !strings.Contains(out.Message, "failed: boom") || strings.Contains(out.Message, "more") {
		t.Fatalf("startSweep = %+v, %v; want the failure's first line", out, err)
	}
}

func TestSweepWritesWhatPruneDidAndFreesTheLock(t *testing.T) {
	root := sweepRepo(t)
	swapPrune(t, func() ([]string, error) { return []string{"removed worktree .komodo/wt/TG-1"}, nil })
	Sweep(root)
	if got := readLog(root); !strings.HasPrefix(got, "swept at ") || !strings.Contains(got, "removed worktree .komodo/wt/TG-1") {
		t.Fatalf("log = %q, want the sweep's outcome", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".komodo", sweepLock)); !os.IsNotExist(err) {
		t.Fatalf("the lock survived the sweep: %v", err)
	}
}

func TestSweepLogsAPruneError(t *testing.T) {
	root := sweepRepo(t)
	swapPrune(t, func() ([]string, error) { return nil, errors.New("no worktree list") })
	Sweep(root)
	if got := readLog(root); !strings.HasPrefix(got, "failed: no worktree list") {
		t.Fatalf("log = %q, want the failure first", got)
	}
}

func TestSweepSkipsWhileAnotherSweepHoldsTheLock(t *testing.T) {
	root := sweepRepo(t)
	calls := swapPrune(t, func() ([]string, error) { return nil, nil })
	lock := filepath.Join(root, ".komodo", sweepLock)
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	Sweep(root)
	if *calls != 0 || readLog(root) != "" {
		t.Fatalf("calls = %d, log = %q; want a held lock to skip the sweep", *calls, readLog(root))
	}
	if _, err := os.Stat(lock); err != nil {
		t.Fatalf("the skipped sweep removed another's lock: %v", err)
	}
}

func TestSweepClearsALockACrashedSweepLeft(t *testing.T) {
	root := sweepRepo(t)
	calls := swapPrune(t, func() ([]string, error) { return nil, nil })
	lock := filepath.Join(root, ".komodo", sweepLock)
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-3 * sweepLimit)
	if err := os.Chtimes(lock, stale, stale); err != nil {
		t.Fatal(err)
	}
	Sweep(root)
	if *calls != 1 {
		t.Fatalf("calls = %d, want the stale lock cleared and one sweep", *calls)
	}
}

func TestSweepStopsAtItsLimitAndLogsTheFailure(t *testing.T) {
	root := sweepRepo(t)
	release, closed := make(chan struct{}), false
	closeRelease := func() {
		if !closed {
			closed = true
			close(release)
		}
	}
	t.Cleanup(closeRelease)
	swapPrune(t, func() ([]string, error) { <-release; return nil, nil })
	old := sweepLimit
	sweepLimit = 50 * time.Millisecond
	t.Cleanup(func() { sweepLimit = old })

	start := time.Now()
	Sweep(root)
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("Sweep took %s past a %s limit", took, sweepLimit)
	}
	if got := readLog(root); !strings.HasPrefix(got, "failed: it ran past") {
		t.Fatalf("log = %q, want the overrun named", got)
	}

	// The first sweep's goroutine is still blocked on prune, so the lock must still record it as going.
	lock := filepath.Join(root, ".komodo", sweepLock)
	if _, err := os.Stat(lock); err != nil {
		t.Fatalf("stat = %v; the lock must survive a timed-out select while the sweep still works", err)
	}
	if takeLock(lock) {
		t.Fatal("a second sweep took the lock while the first's goroutine still holds it")
	}

	closeRelease()
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(lock); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the lock was never freed once the overrun goroutine finished")
		}
	}
}

func TestSweepRefreshesTheMachineBeforeItPrunes(t *testing.T) {
	root := sweepRepo(t)
	var order []string
	swapPrune(t, func() ([]string, error) { order = append(order, "prune"); return []string{"removed worktree x"}, nil })
	old := Refresh
	Refresh = func(at string) ([]string, error) {
		order = append(order, "refresh")
		return []string{"binary /home/.komodo/bin/komodo"}, nil
	}
	t.Cleanup(func() { Refresh = old })
	Sweep(root)
	if strings.Join(order, ",") != "refresh,prune" {
		t.Fatalf("order = %v, want the refresh before the prune", order)
	}
	log, err := os.ReadFile(filepath.Join(root, ".komodo", sweepLog))
	if err != nil || !strings.Contains(string(log), "binary /home/.komodo/bin/komodo") || !strings.Contains(string(log), "removed worktree x") {
		t.Fatalf("log = %q, %v; want both the refresh and the prune recorded", log, err)
	}
}

func TestAFailedRefreshFailsTheSweepAndSkipsThePrune(t *testing.T) {
	root := sweepRepo(t)
	pruned := false
	swapPrune(t, func() ([]string, error) { pruned = true; return nil, nil })
	old := Refresh
	Refresh = func(string) ([]string, error) { return nil, errors.New("render broke") }
	t.Cleanup(func() { Refresh = old })
	Sweep(root)
	log, _ := os.ReadFile(filepath.Join(root, ".komodo", sweepLog))
	if pruned || !strings.HasPrefix(string(log), sweepFailed) || !strings.Contains(string(log), "render broke") {
		t.Fatalf("pruned = %v, log = %q; a failed refresh must stop the sweep and say why", pruned, log)
	}
}
