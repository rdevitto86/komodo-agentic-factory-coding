package line

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"komodo/internal/git"
	"komodo/internal/lease"
	"komodo/internal/proc"
)

// self is this test process, named by pid and start time, a holder that is alive.
func self(t *testing.T) proc.Process {
	t.Helper()
	holder, ok := proc.Of(os.Getpid())
	if !ok {
		t.Skip("this platform cannot name a process by its start time")
	}
	return holder
}

func TestLeaseTakenByAStageHoldsItsBranchInEveryWorktree(t *testing.T) {
	root, _, worktree := detachedGroup(t, "feat/held")
	now := time.Now()
	if err := TakeLease(root, "TG-1", "feat/held", now); err != nil {
		t.Fatal(err)
	}
	held, ok := Lease(worktree, "feat/held", now)
	if !ok || held.Group != "TG-1" || held.Branch != "feat/held" {
		t.Fatalf("Lease from the worktree = %+v, %v; want TG-1's lease on feat/held", held, ok)
	}
	if _, ok := Lease(root, "feat/other", now); ok {
		t.Fatal("a lease on feat/held holds feat/other")
	}
	common, _ := git.Run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if _, err := os.Stat(filepath.Join(common, "komodo", "leases")); err != nil {
		t.Fatalf("the lease is not in the git common dir: %v", err)
	}
}

func TestLeaseIsTakenOnlyWhenABuilderWrites(t *testing.T) {
	root, _, _ := detachedGroup(t, "feat/held")
	plan := &Plan{Group: "TG-1", Branch: "feat/held"}
	for _, next := range []*Action{
		{Action: "spawn", Role: "reviewer", Task: "TG-1-review"},
		{Action: "run", Command: "komodo close TG-1.1", Task: "TG-1.1"},
		{Action: "done"},
	} {
		takeLeases(root, plan, next)
		if _, ok := Lease(root, "feat/held", time.Now()); ok {
			t.Fatalf("%+v took a lease", next)
		}
	}
	for _, next := range []*Action{
		{Action: "spawn", Role: "builder", Task: "TG-1.1"},
		{Action: "run", Command: "komodo machine --role builder TG-1.1", Task: "TG-1.1"},
		{Action: "spawn", Spawns: []Action{{Action: "spawn", Role: "builder", Task: "TG-1.1"}}},
	} {
		takeLeases(root, plan, next)
		if _, ok := Lease(root, "feat/held", time.Now()); !ok {
			t.Fatalf("%+v took no lease", next)
		}
		DropLease(root, "feat/held")
	}
}

func TestTheLeaseEnds(t *testing.T) {
	root, _, worktree := detachedGroup(t, "feat/held")
	taken := time.Now()
	t.Run("on the line's successful push", func(t *testing.T) {
		if err := TakeLease(root, "TG-1", "feat/held", taken); err != nil {
			t.Fatal(err)
		}
		envLog := installPrePush(t, root, 0)
		commit(t, worktree, "work.txt", "work\n", "work")
		if _, ok := Lease(root, "feat/held", taken); !ok {
			t.Fatal("the lease was not held before the push")
		}
		if err := PushFromWorktree(root, worktree, "feat/held"); err != nil {
			t.Fatalf("the lease's own push was refused: %v", err)
		}
		if _, ok := Lease(root, "feat/held", taken); ok {
			t.Fatal("the push left the lease held")
		}
		seen, _ := os.ReadFile(envLog)
		if !strings.Contains(string(seen), LockEnv+"=") {
			t.Fatalf("the pre-push hook was not told its push is the lease's own:\n%s", seen)
		}
	})
	t.Run("two hours after it was taken", func(t *testing.T) {
		if err := lease.Take(root, "TG-1", "feat/held", self(t), taken); err != nil {
			t.Fatal(err)
		}
		if _, ok := Lease(root, "feat/held", taken.Add(2*time.Hour+time.Minute)); ok {
			t.Fatal("a lease taken 2h1m ago still holds")
		}
		if held, ok := Lease(root, "feat/held", taken.Add(time.Hour+59*time.Minute)); !ok || !held.Lapses().Equal(taken.Add(LeaseTTL)) {
			t.Fatalf("a lease taken 1h59m ago = %+v, %v; want it held until %s", held, ok, taken.Add(LeaseTTL))
		}
	})
	t.Run("when its holder exits", func(t *testing.T) {
		dead := proc.Process{PID: os.Getpid(), Start: "not-this-process-start"}
		if err := lease.Take(root, "TG-1", "feat/held", dead, taken); err != nil {
			t.Fatal(err)
		}
		if _, ok := Lease(root, "feat/held", taken); ok {
			t.Fatal("a lease whose holder's start time no longer matches still holds")
		}
		gone := proc.Process{PID: 2147483000, Start: "x"}
		if err := lease.Take(root, "TG-1", "feat/held", gone, taken); err != nil {
			t.Fatal(err)
		}
		if _, ok := Lease(root, "feat/held", taken); ok {
			t.Fatal("a lease whose holder is gone still holds")
		}
	})
	t.Run("never by any other release", func(t *testing.T) {
		if err := lease.Take(root, "TG-1", "feat/held", self(t), taken); err != nil {
			t.Fatal(err)
		}
		if held, ok := Lease(root, "feat/held", taken); !ok || held.Holder.PID != os.Getpid() {
			t.Fatalf("a live lease under 2h = %+v, %v; want held by pid %s", held, ok, strconv.Itoa(os.Getpid()))
		}
	})
}
