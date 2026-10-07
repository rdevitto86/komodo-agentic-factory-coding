//go:build unix

package proc

import (
	"os"
	"testing"
)

func TestHostSkipsEveryShellBetweenTheHookAndItsHost(t *testing.T) {
	table := parseStartTable(`
  10     1 Wed Sep 30 10:00:00 2026 /usr/local/bin/agent-host
  20    10 Wed Sep 30 10:05:00 2026 /bin/sh
  21    20 Wed Sep 30 10:05:00 2026 -zsh
`)
	host, ok := hostIn(21, table)
	if !ok || host.PID != 10 || host.Start != "Wed Sep 30 10:00:00 2026" {
		t.Fatalf("host = %+v, %v; want pid 10 with its start time", host, ok)
	}
}

func TestHostIsUnknownWhenTheChainLeavesTheTable(t *testing.T) {
	if host, ok := hostIn(99, map[int]started{}); ok {
		t.Fatalf("host = %+v, want none for a pid the table lacks", host)
	}
}

func TestAliveTellsAReusedPidFromTheProcessThatEnded(t *testing.T) {
	table, err := startTable()
	if err != nil {
		t.Skipf("no ps here: %v", err)
	}
	self := Process{PID: os.Getpid(), Start: table[os.Getpid()].start}
	if alive, known := Alive(self); !known || !alive {
		t.Fatalf("Alive(self) = %v, %v; want alive and known", alive, known)
	}
	self.Start = "Thu Jan  1 00:00:00 1970"
	if alive, _ := Alive(self); alive {
		t.Fatal("a pid whose start time differs must read as ended, not alive")
	}
}
