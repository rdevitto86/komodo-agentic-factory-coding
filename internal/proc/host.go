package proc

import (
	"os"
	"path/filepath"
	"strings"
)

// Process names one running process by pid and start time, so a reused pid never passes for it.
type Process struct {
	PID   int    `json:"pid"`
	Start string `json:"start"`
}

// started is one row of the process table: its parent, start time and command name.
type started struct {
	ppid  int
	start string
	comm  string
}

// shells are the command names a host may run a hook through, skipped when finding the host itself.
var shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "fish": true, "env": true}

// hostHops bounds the walk up the process tree, so a cycle in a bad table never loops.
const hostHops = 16

// Host is the nearest ancestor of this process that is not a shell: the agent host that ran the hook.
func Host() (Process, bool) {
	table, err := startTable()
	if err != nil {
		return Process{}, false
	}
	return hostIn(os.Getppid(), table)
}

// hostIn walks up from pid through table, past every shell, to the first process that is not one.
func hostIn(pid int, table map[int]started) (Process, bool) {
	for hops := 0; pid > 1 && hops < hostHops; hops++ {
		entry, ok := table[pid]
		if !ok {
			return Process{}, false
		}
		if !shells[strings.TrimPrefix(filepath.Base(entry.comm), "-")] {
			return Process{PID: pid, Start: entry.start}, true
		}
		pid = entry.ppid
	}
	return Process{}, false
}

// Alive reports whether process still runs; known is false when this platform cannot tell.
func Alive(process Process) (alive, known bool) {
	table, err := startTable()
	if err != nil {
		return false, false
	}
	entry, ok := table[process.PID]
	return ok && entry.start == process.Start, true
}
