package proc

import (
	"testing"
	"time"
)

func TestShellReportsExitCodeAndOutput(t *testing.T) {
	result := Shell(t.TempDir(), "echo hi; exit 3", time.Second)
	if result.ExitCode != 3 || result.Output != "hi" || result.TimedOut {
		t.Fatalf("result = %+v", result)
	}
}

func TestShellKillsAHungCommandAndItsChildren(t *testing.T) {
	started := time.Now()
	result := Shell(t.TempDir(), "sleep 30 & sleep 30", 200*time.Millisecond)
	if !result.TimedOut || result.ExitCode != ExitTimeout {
		t.Fatalf("result = %+v", result)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("the kill took %s; the group was not killed", time.Since(started))
	}
}
