package harness

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

// DropLease ends branch's lease; the harness's successful push calls it.
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
