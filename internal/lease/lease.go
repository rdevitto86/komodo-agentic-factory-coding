// Package lease holds the one lock on a branch: a live builder's lease, kept in the git common dir.
package lease

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"komodo/internal/fsx"
	"komodo/internal/git"
	"komodo/internal/proc"
)

// TTL is how long a lease holds a branch before it lapses on its own.
const TTL = 2 * time.Hour

// RunEnv carries a run launcher's pid into its host; a holder with that pid is the lease's own run.
const RunEnv = "KOMODO_RUN_PID"

// Lease is one builder's claim on a branch: its group, holder process, and when it was taken.
type Lease struct {
	Group  string       `json:"group"`
	Branch string       `json:"branch"`
	Holder proc.Process `json:"holder"`
	Taken  time.Time    `json:"taken"`
}

// Own reports whether this process runs under the lease's holder, so its push is the lease's own.
func (l Lease) Own() bool { return os.Getenv(RunEnv) == strconv.Itoa(l.Holder.PID) }

// Lapses is when the lease ends on its own.
func (l Lease) Lapses() time.Time { return l.Taken.Add(TTL) }

// Refusal is the sentence that tells a pusher who holds branch and when the hold ends.
func (l Lease) Refusal() string {
	return fmt.Sprintf("%s is leased by group %s (pid %d) until %s; the line's push drops it sooner",
		l.Branch, l.Group, l.Holder.PID, l.Lapses().Format(time.RFC3339))
}

// Dir is where every worktree of dir's repository finds its leases: under the git common dir.
func Dir(dir string) (string, error) {
	common, err := git.Run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return filepath.Join(common, "komodo", "leases"), nil
}

// path is the file one branch's lease lives in.
func path(dir, branch string) (string, error) {
	leases, err := Dir(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(leases, url.PathEscape(branch)+".json"), nil
}

// afterTake lets a test land a second writer between Take's write and its reread.
var afterTake = func() {}

// Take writes a lease on branch for holder atomically, replacing any earlier one; a second Take
// that lands on the file before this one rereads it fails this call instead of a false success.
func Take(dir, group, branch string, holder proc.Process, now time.Time) error {
	file, err := path(dir, branch)
	if err != nil {
		return err
	}
	data, err := json.Marshal(Lease{Group: group, Branch: branch, Holder: holder, Taken: now})
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := fsx.WriteFile(file, data, 0o644); err != nil {
		return err
	}
	afterTake()
	after, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if !bytes.Equal(after, data) {
		return fmt.Errorf("%s: another holder took the lease while this one wrote it", branch)
	}
	return nil
}

// Held returns branch's lease while it is under TTL old and its holder still runs.
func Held(dir, branch string, now time.Time) (Lease, bool) {
	file, err := path(dir, branch)
	if err != nil {
		return Lease{}, false
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return Lease{}, false
	}
	var held Lease
	if json.Unmarshal(data, &held) != nil || held.Branch != branch || !now.Before(held.Lapses()) {
		return Lease{}, false
	}
	if alive, known := proc.Alive(held.Holder); known && !alive {
		return Lease{}, false
	}
	return held, true
}

// Drop removes branch's lease; a branch with none is left as it is.
func Drop(dir, branch string) {
	if file, err := path(dir, branch); err == nil {
		_ = os.Remove(file)
	}
}
