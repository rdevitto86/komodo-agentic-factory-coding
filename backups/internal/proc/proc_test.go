package proc

import (
	"context"
	"fmt"
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

func TestBoundedWriterDiscardsPastItsLimit(t *testing.T) {
	writer := NewBoundedWriter(5)
	n, err := writer.Write([]byte("hello world"))
	if n != 11 || err != nil {
		t.Fatalf("Write = %d, %v; want every byte reported written", n, err)
	}
	if got := writer.String(); got != "hello" {
		t.Fatalf("String = %q, want the first 5 bytes kept", got)
	}
	if n, err := writer.Write([]byte("more")); n != 4 || err != nil {
		t.Fatalf("Write past the limit = %d, %v; want no error and no growth", n, err)
	}
	if got := writer.String(); got != "hello" {
		t.Fatalf("String = %q, want the limit held", got)
	}
}

func TestShellBoundsAFloodingCommandsOutput(t *testing.T) {
	result := Shell(t.TempDir(), fmt.Sprintf("yes | head -c %d", MaxOutput+1<<20), 5*time.Second)
	if len(result.Output) > MaxOutput {
		t.Fatalf("output len = %d, want it capped at MaxOutput = %d", len(result.Output), MaxOutput)
	}
}

func TestShellArgvUsesAShellOnlyForShellSyntax(t *testing.T) {
	saved, savedLook := shellGOOS, lookPath
	t.Cleanup(func() { shellGOOS, lookPath = saved, savedLook })
	found := func(string) (string, error) { return "/usr/bin/sh", nil }
	missing := func(string) (string, error) { return "", exec.ErrNotFound }
	syntax := "go test ./... && ! grep -q x y"
	cases := []struct {
		goos, command string
		look          func(string) (string, error)
		want          []string
	}{
		{"linux", "go test ./...", missing, []string{"go", "test", "./..."}},
		{"windows", "go test ./...", found, []string{"go", "test", "./..."}},
		{"windows", "go test ./...", missing, []string{"go", "test", "./..."}},
		{"linux", syntax, missing, []string{"sh", "-c", syntax}},
		{"windows", syntax, found, []string{"sh", "-c", syntax}},
		{"windows", syntax, missing, []string{"cmd", "/C", syntax}},
		{"linux", "GOOS=windows go vet ./...", missing, []string{"sh", "-c", "GOOS=windows go vet ./..."}},
		{"linux", "echo $HOME", missing, []string{"sh", "-c", "echo $HOME"}},
		{"linux", "ls *.go", missing, []string{"sh", "-c", "ls *.go"}},
		{"linux", "exit 3", missing, []string{"sh", "-c", "exit 3"}},
		{"linux", "cd sub", missing, []string{"sh", "-c", "cd sub"}},
		{"linux", "", missing, []string{"sh", "-c", ""}},
	}
	for _, c := range cases {
		shellGOOS, lookPath = c.goos, c.look
		if got := ShellArgv(c.command); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %q: argv = %q, want %q", c.goos, c.command, got, c.want)
		}
	}
}

// TestAMissingProgramKeepsTheShellsNotFoundCode proves a plain command naming no program reports 127,
// as sh would, so callers that read the code keep working.
func TestAMissingProgramKeepsTheShellsNotFoundCode(t *testing.T) {
	if ran := Shell(t.TempDir(), "komodo-no-such-program-anywhere arg", time.Minute); ran.ExitCode != ExitNotFound {
		t.Fatalf("exit = %d, want %d", ran.ExitCode, ExitNotFound)
	}
}
