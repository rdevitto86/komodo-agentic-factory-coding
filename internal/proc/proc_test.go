package proc

import (
	"os/exec"
	"strings"
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

func TestErrNamesTheExitOrTheClock(t *testing.T) {
	if ok := Shell(t.TempDir(), "true", time.Second); !ok.OK() || ok.Err() != nil {
		t.Fatalf("result = %+v; a zero exit is no error", ok)
	}
	if failed := Shell(t.TempDir(), "exit 4", time.Second); failed.OK() || !strings.Contains(failed.Err().Error(), "exit status 4") {
		t.Fatalf("err = %v", failed.Err())
	}
	if hung := Shell(t.TempDir(), "sleep 30", 100*time.Millisecond); hung.OK() || !strings.Contains(hung.Err().Error(), "timed out") {
		t.Fatalf("err = %v", hung.Err())
	}
}

func TestExecRunsAProgramWithoutAShell(t *testing.T) {
	result := Exec(t.TempDir(), time.Second, "echo", "a;b")
	if !result.OK() || result.Output != "a;b" {
		t.Fatalf("result = %+v; the argument must reach the program unsplit", result)
	}
	missing := Exec(t.TempDir(), time.Second, "komodo-no-such-program")
	if missing.OK() || missing.Output == "" {
		t.Fatalf("result = %+v; a missing program fails and says why", missing)
	}
}

func TestShellEnvRunsInOnlyTheGivenEnvironment(t *testing.T) {
	t.Setenv("PROC_TEST_SECRET", "parent")
	result := ShellEnv(t.TempDir(), `printf '%s|%s' "$PROC_TEST_SECRET" "$PROC_TEST_GIVEN"`, time.Second, []string{"PROC_TEST_GIVEN=child"})
	if result.Output != "|child" {
		t.Fatalf("output = %q; only the given environment reaches the command", result.Output)
	}
}

func TestKillGroupIgnoresACommandThatNeverStarted(t *testing.T) {
	KillGroup(exec.Command("true"))
}
