package proc

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeTable is a process table a test edits while a watcher samples it, recording every kill.
type fakeTable struct {
	mu     sync.Mutex
	procs  map[int]proc
	killed []int
}

func (f *fakeTable) list() (map[int]proc, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[int]proc, len(f.procs))
	for pid, p := range f.procs {
		out[pid] = p
	}
	return out, nil
}

func (f *fakeTable) kill(pid int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.killed = append(f.killed, pid)
	delete(f.procs, pid)
}

func (f *fakeTable) killedSorted() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := append([]int(nil), f.killed...)
	sort.Ints(out)
	return out
}

func TestAWatcherKillsATreeOverTheProcessCap(t *testing.T) {
	table := &fakeTable{procs: map[int]proc{100: {ppid: 1, pgid: 100, comm: "claude"}}}
	for pid := 101; pid <= 110; pid++ {
		table.procs[pid] = proc{ppid: 100, pgid: 100, comm: "go"}
	}
	table.procs[500] = proc{ppid: 1, pgid: 500, comm: "unrelated"}
	w := watchWith(100, Limits{Procs: 5}, 10*time.Millisecond, table.list, table.kill)
	deadline := time.Now().Add(2 * time.Second)
	for w.Breach() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	w.Stop()
	if !strings.Contains(w.Breach(), "11 processes") {
		t.Fatalf("breach = %q; eleven processes over a cap of five must stop the tree", w.Breach())
	}
	for _, pid := range table.killedSorted() {
		if pid == 500 {
			t.Fatal("the watcher killed a process outside its tree")
		}
	}
	if got := len(table.killedSorted()); got != 11 {
		t.Fatalf("killed %d processes, want the whole tree of 11", got)
	}
}

func TestAWatcherCountsAnOrphanStillInTheGroup(t *testing.T) {
	procs := map[int]proc{
		100: {ppid: 1, pgid: 100, comm: "claude"},
		200: {ppid: 1, pgid: 100, comm: "go"},
	}
	tree := treeOf(100, procs)
	if len(tree) != 2 {
		t.Fatalf("tree = %v; a child reparented to init but still in the group belongs to the tree", tree)
	}
}

func TestAWatcherKillsATreeOverTheMemoryCap(t *testing.T) {
	table := &fakeTable{procs: map[int]proc{
		100: {ppid: 1, pgid: 100, rssKB: 1 << 20, comm: "claude"},
		101: {ppid: 100, pgid: 100, rssKB: 3 << 20, comm: "go"},
	}}
	w := watchWith(100, Limits{RSSBytes: 2 << 30}, 10*time.Millisecond, table.list, table.kill)
	deadline := time.Now().Add(2 * time.Second)
	for w.Breach() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	w.Stop()
	if !strings.Contains(w.Breach(), "4.0 GB resident") {
		t.Fatalf("breach = %q; 4 GB over a 2 GB cap must stop the tree", w.Breach())
	}
}

// awaitSample blocks until the watcher has sampled its tree at least once, bounded by a deadline.
func awaitSample(t *testing.T, w *Watcher) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		w.mu.Lock()
		seen := len(w.seen) > 0
		w.mu.Unlock()
		if seen {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the watcher never sampled the tree")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestStopReapsAStragglerButNotAReusedPid(t *testing.T) {
	table := &fakeTable{procs: map[int]proc{
		100: {ppid: 1, pgid: 100, comm: "claude"},
		101: {ppid: 100, pgid: 300, comm: "go"},
		102: {ppid: 100, pgid: 301, comm: "sleep"},
	}}
	w := watchWith(100, Limits{Procs: 50}, 10*time.Millisecond, table.list, table.kill)
	awaitSample(t, w)
	table.mu.Lock()
	delete(table.procs, 100)
	table.procs[101] = proc{ppid: 1, pgid: 300, comm: "go"}
	table.procs[102] = proc{ppid: 1, pgid: 999, comm: "someone-else"}
	table.mu.Unlock()
	w.Stop()
	killed := table.killedSorted()
	if len(killed) != 1 || killed[0] != 101 {
		t.Fatalf("killed = %v; the straggler 101 goes, and 102, now another command, stays", killed)
	}
}

func TestShellStopsARunawayTreeAndLeavesNothingBehind(t *testing.T) {
	saved, savedInterval := DefaultLimits, WatchInterval
	DefaultLimits, WatchInterval = Limits{Procs: 10}, 50*time.Millisecond
	t.Cleanup(func() { DefaultLimits, WatchInterval = saved, savedInterval })
	sleep := uniqueSleep(37)
	result := Shell(t.TempDir(), "for i in $(seq 1 30); do sleep "+sleep+" & done; wait", time.Minute)
	if result.ExitCode != ExitRunaway || !strings.Contains(result.Output, "runaway") {
		t.Fatalf("result = %+v; thirty sleeps over a cap of ten must be stopped as a runaway", result)
	}
	// A killed child takes a moment to be reaped, so the process table is polled until none remains.
	deadline := time.Now().Add(3 * time.Second)
	for {
		procs, err := listProcs()
		if err != nil {
			t.Skip("no process listing here")
		}
		stuck := false
		for pid, p := range procs {
			if strings.Contains(p.comm, "sleep") && p.ppid == 1 {
				if out := Exec("", time.Second, "ps", "-o", "args=", "-p", strconv.Itoa(pid)); strings.TrimSpace(out.Output) == "sleep "+sleep {
					stuck = true
				}
			}
		}
		if !stuck {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("a runaway session's children outlived it")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestShellReapsABackgroundChildLeftInTheGroup(t *testing.T) {
	sleep := uniqueSleep(41)
	result := Shell(t.TempDir(), "sleep "+sleep+" & echo started", time.Minute)
	if !result.OK() {
		t.Fatalf("result = %+v", result)
	}
	// pgrep exits zero only when a process still matches; a killed child takes a moment to be reaped.
	deadline := time.Now().Add(3 * time.Second)
	for {
		out := Exec("", 5*time.Second, "pgrep", "-f", "^sleep "+sleep+"$")
		if !out.OK() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the background child outlived its command: pids %s", out.Output)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// uniqueSleep is a sleep duration no other test process uses, so a process search finds only this test's children.
func uniqueSleep(seconds int) string {
	return fmt.Sprintf("%d.%d", seconds, os.Getpid())
}
