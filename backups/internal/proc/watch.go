package proc

import (
	"fmt"
	"sync"
	"time"
)

// Limits bound one process tree: how many live processes it may hold and how much memory they may keep resident.
type Limits struct {
	Procs    int
	RSSBytes int64
}

// DefaultLimits stop a runaway tree, such as a test rerunning its own suite, before it starves the machine.
var DefaultLimits = Limits{Procs: 256, RSSBytes: 8 << 30}

// WatchInterval is how often a watcher samples its tree.
var WatchInterval = 5 * time.Second

// proc is one sampled process: its parent, group, resident memory and command name.
type proc struct {
	ppid, pgid int
	rssKB      int64
	comm       string
}

// Watcher samples one process tree until stopped, kills the whole tree past its limits, and reaps what outlives it.
type Watcher struct {
	root   int
	limits Limits
	list   func() (map[int]proc, error)
	kill   func(pid int)

	mu     sync.Mutex
	seen   map[int]string
	breach string
	stop   chan struct{}
	done   chan struct{}
}

// Watch starts sampling the tree rooted at pid, which leads its own process group, every WatchInterval.
func Watch(pid int, limits Limits) *Watcher {
	return watchWith(pid, limits, WatchInterval, listProcs, killPid)
}

// watchWith is Watch with its clock, process listing and kill supplied, which is what a test drives.
func watchWith(pid int, limits Limits, interval time.Duration, list func() (map[int]proc, error), kill func(int)) *Watcher {
	w := &Watcher{root: pid, limits: limits, list: list, kill: kill, seen: map[int]string{},
		stop: make(chan struct{}), done: make(chan struct{})}
	go w.loop(interval)
	return w
}

// loop samples the tree every interval until Stop, or until a sample finds a breach.
func (w *Watcher) loop(interval time.Duration) {
	defer close(w.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			if w.sample() {
				return
			}
		}
	}
}

// sample records the tree's processes and kills all of them when it is over either limit, reporting a breach.
func (w *Watcher) sample() bool {
	procs, err := w.list()
	if err != nil {
		return false
	}
	tree := treeOf(w.root, procs)
	var rssKB int64
	w.mu.Lock()
	for _, pid := range tree {
		rssKB += procs[pid].rssKB
		w.seen[pid] = procs[pid].comm
	}
	w.mu.Unlock()
	over := ""
	switch {
	case w.limits.Procs > 0 && len(tree) > w.limits.Procs:
		over = fmt.Sprintf("%d processes, over the cap of %d", len(tree), w.limits.Procs)
	case w.limits.RSSBytes > 0 && rssKB*1024 > w.limits.RSSBytes:
		over = fmt.Sprintf("%.1f GB resident, over the cap of %.1f GB", float64(rssKB)/(1<<20), float64(w.limits.RSSBytes)/(1<<30))
	}
	if over == "" {
		return false
	}
	w.mu.Lock()
	w.breach = over
	w.mu.Unlock()
	for _, pid := range tree {
		w.kill(pid)
	}
	return true
}

// Breach names the limit the tree broke, or is empty when it never did.
func (w *Watcher) Breach() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.breach
}

// Stop ends the sampling, then kills what the tree left behind: its group and each process it owned.
func (w *Watcher) Stop() {
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
	<-w.done
	w.mu.Lock()
	seen := len(w.seen) > 0
	w.mu.Unlock()
	if !seen {
		return
	}
	procs, err := w.list()
	if err != nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for pid, p := range procs {
		// A pid seen in the tree still running the same command is a straggler, not a reused pid.
		if comm, ok := w.seen[pid]; (ok && comm == p.comm) || p.pgid == w.root {
			w.kill(pid)
		}
	}
}

// treeOf is root and every live process descended from it or still in its process group, orphans included.
func treeOf(root int, procs map[int]proc) []int {
	children := map[int][]int{}
	for pid, p := range procs {
		children[p.ppid] = append(children[p.ppid], pid)
	}
	in := map[int]bool{}
	queue := []int{root}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		if in[pid] {
			continue
		}
		if _, live := procs[pid]; live {
			in[pid] = true
		}
		queue = append(queue, children[pid]...)
	}
	for pid, p := range procs {
		if p.pgid == root {
			in[pid] = true
		}
	}
	tree := make([]int, 0, len(in))
	for pid := range in {
		tree = append(tree, pid)
	}
	return tree
}
