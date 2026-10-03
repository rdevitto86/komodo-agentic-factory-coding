package proc

import (
	"context"
	"os/exec"
	"reflect"
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

func TestExecContextReportsHowACommandEnded(t *testing.T) {
	const timeout = 200 * time.Millisecond
	cases := []struct {
		name     string
		command  string
		exit     int
		timedOut bool
		output   string
	}{
		{"it passes", "echo fine", 0, false, "fine"},
		{"it fails", "echo broken; exit 3", 3, false, "broken"},
		{"its clock runs out", "sleep 60", 124, true, "timed out after 200ms"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ran := ExecContext(context.Background(), t.TempDir(), timeout, "sh", "-c", tc.command)
			if ran.ExitCode != tc.exit || ran.TimedOut != tc.timedOut || !strings.Contains(ran.Output, tc.output) {
				t.Fatalf("result = %+v, want exit %d, timed out %v, output naming %q", ran, tc.exit, tc.timedOut, tc.output)
			}
		})
	}
}

func TestExecContextKillsTheGroupOnceItsParentContextIsDone(t *testing.T) {
	const stopAfter = 200 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), stopAfter)
	defer cancel()
	began := time.Now()
	ran := ExecContext(ctx, t.TempDir(), time.Minute, "sh", "-c", "sleep 60")
	if took := time.Since(began); took > 10*time.Second {
		t.Fatalf("exec took %s after its context ended, want under 10s", took)
	}
	if ran.TimedOut {
		t.Fatalf("result = %+v; a parent ctx ending is not the command's own clock running out", ran)
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

func TestShellArgvUsesShEverywhereAndCmdOnlyOnWindowsWithoutIt(t *testing.T) {
	saved, savedLook := shellGOOS, lookPath
	t.Cleanup(func() { shellGOOS, lookPath = saved, savedLook })
	found := func(string) (string, error) { return "/usr/bin/sh", nil }
	missing := func(string) (string, error) { return "", exec.ErrNotFound }
	cases := []struct {
		goos string
		look func(string) (string, error)
		want []string
	}{
		{"linux", missing, []string{"sh", "-c", "go test ./..."}},
		{"windows", found, []string{"sh", "-c", "go test ./..."}},
		{"windows", missing, []string{"cmd", "/C", "go test ./..."}},
	}
	for _, c := range cases {
		shellGOOS, lookPath = c.goos, c.look
		if got := ShellArgv("go test ./..."); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: argv = %q, want %q", c.goos, got, c.want)
		}
	}
}
