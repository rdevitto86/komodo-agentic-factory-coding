package lease

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"komodo/internal/proc"
)

// gitRepo returns a fresh git repo root, so Dir can resolve its common dir.
func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return dir
}

// self is this test process, a holder proc.Alive reports as alive.
func self(t *testing.T) proc.Process {
	t.Helper()
	holder, ok := proc.Of(os.Getpid())
	if !ok {
		t.Skip("this platform cannot name a process by its start time")
	}
	return holder
}

func TestTakeThenHeldRoundTrips(t *testing.T) {
	dir := gitRepo(t)
	now := time.Now()
	holder := self(t)
	if err := Take(dir, "TG-1", "feat/a", holder, now); err != nil {
		t.Fatal(err)
	}
	held, ok := Held(dir, "feat/a", now)
	if !ok || held.Group != "TG-1" || held.Branch != "feat/a" || held.Holder != holder {
		t.Fatalf("Held = %+v, %v; want TG-1's lease on feat/a", held, ok)
	}
}

func TestHeldRefusesAfterTTL(t *testing.T) {
	dir := gitRepo(t)
	taken := time.Now()
	if err := Take(dir, "TG-1", "feat/a", self(t), taken); err != nil {
		t.Fatal(err)
	}
	if _, ok := Held(dir, "feat/a", taken.Add(TTL+time.Minute)); ok {
		t.Fatal("a lease past its TTL still holds")
	}
}

func TestHeldRefusesADeadHolder(t *testing.T) {
	dir := gitRepo(t)
	taken := time.Now()
	dead := proc.Process{PID: 2147483000, Start: "x"}
	if err := Take(dir, "TG-1", "feat/a", dead, taken); err != nil {
		t.Fatal(err)
	}
	if _, ok := Held(dir, "feat/a", taken); ok {
		t.Fatal("a lease whose holder is gone still holds")
	}
}

func TestDropRemovesTheLease(t *testing.T) {
	dir := gitRepo(t)
	now := time.Now()
	if err := Take(dir, "TG-1", "feat/a", self(t), now); err != nil {
		t.Fatal(err)
	}
	Drop(dir, "feat/a")
	if _, ok := Held(dir, "feat/a", now); ok {
		t.Fatal("Drop left the lease held")
	}
}

func TestDropOnABranchWithNoLeaseIsANoop(t *testing.T) {
	Drop(gitRepo(t), "feat/never-taken")
}

func TestOwnReportsThisProcessAsTheHolder(t *testing.T) {
	holder := proc.Process{PID: os.Getpid()}
	t.Setenv(RunEnv, strconv.Itoa(holder.PID))
	l := Lease{Holder: holder}
	if !l.Own() {
		t.Fatal("Own = false, want true for a holder matching KOMODO_RUN_PID")
	}
}

func TestOwnReportsAnotherProcessAsNotTheHolder(t *testing.T) {
	t.Setenv(RunEnv, strconv.Itoa(os.Getpid()))
	l := Lease{Holder: proc.Process{PID: os.Getpid() + 1}}
	if l.Own() {
		t.Fatal("Own = true, want false for a holder other than KOMODO_RUN_PID")
	}
}

func TestRefusalNamesTheBranchGroupHolderAndDeadline(t *testing.T) {
	taken := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	l := Lease{Group: "TG-1", Branch: "feat/a", Holder: proc.Process{PID: 123}, Taken: taken}
	got := l.Refusal()
	for _, want := range []string{"feat/a", "TG-1", "123", l.Lapses().Format(time.RFC3339)} {
		if !strings.Contains(got, want) {
			t.Fatalf("Refusal() = %q, want it to contain %q", got, want)
		}
	}
}

func TestDirFailsOutsideAGitRepo(t *testing.T) {
	if _, err := Dir(t.TempDir()); err == nil {
		t.Fatal("Dir = nil error outside a git repo, want one naming the failure")
	}
}

func TestTakeFailsOutsideAGitRepo(t *testing.T) {
	if err := Take(t.TempDir(), "TG-1", "feat/a", self(t), time.Now()); err == nil {
		t.Fatal("Take = nil error outside a git repo, want one naming the failure")
	}
}

func TestHeldFailsOutsideAGitRepo(t *testing.T) {
	if _, ok := Held(t.TempDir(), "feat/a", time.Now()); ok {
		t.Fatal("Held = true outside a git repo, want false")
	}
}

func TestHeldRefusesABranchMismatch(t *testing.T) {
	dir := gitRepo(t)
	now := time.Now()
	if err := Take(dir, "TG-1", "feat/a", self(t), now); err != nil {
		t.Fatal(err)
	}
	if _, ok := Held(dir, "feat/other", now); ok {
		t.Fatal("Held = true for a lease filed under a different branch")
	}
}

func TestHeldRefusesUnparsableJSON(t *testing.T) {
	dir := gitRepo(t)
	file, err := path(dir, "feat/a")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := Held(dir, "feat/a", time.Now()); ok {
		t.Fatal("Held = true for a lease file that is not valid JSON")
	}
}

// TestTakeFailsWhenAnotherHolderWinsTheRace lands a second writer between the first's write and
// its reread, so the first must see it lost the branch instead of a false success.
func TestTakeFailsWhenAnotherHolderWinsTheRace(t *testing.T) {
	dir := gitRepo(t)
	now := time.Now()
	holderA := proc.Process{PID: 11111, Start: "a"}
	holderB := proc.Process{PID: 22222, Start: "b"}
	old := afterTake
	t.Cleanup(func() { afterTake = old })
	var raced bool
	afterTake = func() {
		if raced {
			return
		}
		raced = true
		if err := Take(dir, "TG-1", "feat/race", holderB, now); err != nil {
			t.Errorf("the second holder's Take = %v, want it to win cleanly", err)
		}
	}

	if err := Take(dir, "TG-1", "feat/race", holderA, now); err == nil {
		t.Fatal("the first holder's Take = nil, want it told another holder won the race")
	}
	file, err := path(dir, "feat/race")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var left Lease
	if json.Unmarshal(data, &left) != nil || left.Holder.PID != holderB.PID {
		t.Fatalf("lease on disk = %+v; want holder B's lease left there", left)
	}
}

// TestTakeRacesTwoRealGoroutines runs Take from two real goroutines under -race, so the fix is
// proven free of a Go-level data race on top of the deterministic ordering test above.
func TestTakeRacesTwoRealGoroutines(t *testing.T) {
	dir := gitRepo(t)
	now := time.Now()
	errs := make([]error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		errs[0] = Take(dir, "TG-1", "feat/real-race", proc.Process{PID: 33333, Start: "a"}, now)
	}()
	go func() {
		defer wait.Done()
		errs[1] = Take(dir, "TG-1", "feat/real-race", proc.Process{PID: 44444, Start: "b"}, now)
	}()
	wait.Wait()
	file, err := path(dir, "feat/real-race")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var lease Lease
	if err := json.Unmarshal(data, &lease); err != nil {
		t.Fatalf("the lease file is not whole JSON after two concurrent writers: %v\n%s", err, data)
	}
}
