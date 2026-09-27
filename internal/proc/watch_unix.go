//go:build unix

package proc

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// listProcs reads every live process's parent, group, resident memory and command from ps.
func listProcs() (map[int]proc, error) {
	out, err := exec.Command("ps", "-A", "-o", "pid=,ppid=,pgid=,rss=,comm=").Output()
	if err != nil {
		return nil, err
	}
	procs := map[int]proc{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		pgid, err3 := strconv.Atoi(fields[2])
		rss, err4 := strconv.ParseInt(fields[3], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			continue
		}
		procs[pid] = proc{ppid: ppid, pgid: pgid, rssKB: rss, comm: strings.Join(fields[4:], " ")}
	}
	return procs, nil
}

// killPid kills one process outright.
func killPid(pid int) { _ = syscall.Kill(pid, syscall.SIGKILL) }
