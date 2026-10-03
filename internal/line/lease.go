package line

import (
	"os"
	"strconv"
	"time"

	"komodo/internal/lease"
	"komodo/internal/proc"
)

// TakeLease leases branch to the process running this stage, as of now.
func TakeLease(root, group, branch string, now time.Time) error {
	return lease.Take(root, group, branch, leaseHolder(), now)
}

// Lease returns branch's lease while it is under lease.TTL old and its holder still runs.
func Lease(root, branch string, now time.Time) (lease.Lease, bool) {
	return lease.Held(root, branch, now)
}

// DropLease ends branch's lease; the line's successful push calls it.
func DropLease(root, branch string) {
	lease.Drop(root, branch)
}

// leaseHolder is the launcher a headless run names, else the agent host running this stage, else this process.
func leaseHolder() proc.Process {
	if pid, err := strconv.Atoi(os.Getenv(LockEnv)); err == nil {
		if holder, ok := proc.Of(pid); ok {
			return holder
		}
	}
	if holder, ok := proc.Host(); ok {
		return holder
	}
	holder, _ := proc.Of(os.Getpid())
	return holder
}

// takeLeases leases the group branch when action sends a builder to write on it, returning any
// failure to take it instead of discarding it.
func takeLeases(root string, plan *Plan, next *Action) error {
	if plan.Branch == "" || !writes(*next) {
		return nil
	}
	return TakeLease(root, plan.Group, plan.Branch, time.Now())
}

// writes reports whether action hands a builder the branch to write: a spawn, or a local builder run.
func writes(next Action) bool {
	if next.Role == "builder" || next.Command == "komodo machine --role builder "+next.Task {
		return true
	}
	for _, spawn := range next.Spawns {
		if writes(spawn) {
			return true
		}
	}
	return false
}
