//go:build unix

package proc

import (
	"os/exec"
	"strconv"
	"strings"
)

// startTable reads every live process's parent, start time and command from ps.
func startTable() (map[int]started, error) {
	command := exec.Command("ps", "-A", "-o", "pid=,ppid=,lstart=,comm=")
	command.Env = append(command.Environ(), "LC_ALL=C")
	out, err := command.Output()
	if err != nil {
		return nil, err
	}
	return parseStartTable(string(out)), nil
}

// parseStartTable reads ps rows whose start time is the five-word lstart form, such as Wed Sep 30 10:00:00 2026.
func parseStartTable(out string) map[int]started {
	table := map[int]started{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}
		table[pid] = started{ppid: ppid, start: strings.Join(fields[2:7], " "), comm: strings.Join(fields[7:], " ")}
	}
	return table
}
