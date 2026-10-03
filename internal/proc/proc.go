// Package proc runs a shell command under a wall clock, in its own process group, so a hung child never hangs a station.
package proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// DefaultTimeout is how long a station command may run when nothing narrower is set.
const DefaultTimeout = 10 * time.Minute

// ExitTimeout is the exit code a timed-out command reports, the same one timeout(1) uses.
const ExitTimeout = 124

// ExitRunaway is the exit code a command reports when its process tree broke DefaultLimits and was killed.
const ExitRunaway = 125

// Result is one command's outcome: its combined output, exit code, and whether the clock cut it.
type Result struct {
	Output   string
	ExitCode int
	TimedOut bool
	Seconds  float64
}

// OK reports whether the command exited zero.
func (r Result) OK() bool { return r.ExitCode == 0 }

// Err renders the result as an error, or nil when it exited zero.
func (r Result) Err() error {
	if r.OK() {
		return nil
	}
	if r.TimedOut {
		return fmt.Errorf("timed out after %.0fs", r.Seconds)
	}
	if r.ExitCode == ExitRunaway {
		return errors.New("stopped as a runaway process tree")
	}
	return fmt.Errorf("exit status %d", r.ExitCode)
}

// Shell runs one shell command in dir under timeout, killing its whole process group when the clock runs out.
func Shell(dir, command string, timeout time.Duration) Result {
	return ShellEnv(dir, command, timeout, nil)
}

// ShellEnv is Shell in the given environment, or in this process's own when env is nil.
func ShellEnv(dir, command string, timeout time.Duration, env []string) Result {
	argv := ShellArgv(command)
	return run(context.Background(), dir, timeout, env, argv[0], argv[1:]...)
}

// shellGOOS and lookPath are the platform and PATH ShellArgv reads; tests swap them.
var (
	shellGOOS = runtime.GOOS
	lookPath  = exec.LookPath
)

// ShellArgv runs command through POSIX sh, which native Windows takes from Git for Windows; with no sh
// on PATH there, it falls back to cmd /C.
func ShellArgv(command string) []string {
	if shellGOOS == "windows" {
		if _, err := lookPath("sh"); err != nil {
			return []string{"cmd", "/C", command}
		}
	}
	return []string{"sh", "-c", command}
}

// Exec runs one program in dir under timeout, killing its whole process group when the clock runs out.
func Exec(dir string, timeout time.Duration, name string, args ...string) Result {
	return run(context.Background(), dir, timeout, nil, name, args...)
}

// ExecContext is Exec whose process group is also killed once ctx is done.
func ExecContext(ctx context.Context, dir string, timeout time.Duration, name string, args ...string) Result {
	return run(ctx, dir, timeout, nil, name, args...)
}

// run is Exec in the given environment, or in this process's own when env is nil, and is also
// killed once the parent ctx is done.
func run(ctx context.Context, dir string, timeout time.Duration, env []string, name string, args ...string) Result {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	clock, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(clock, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	Group(cmd)
	cmd.Cancel = func() error {
		KillGroup(cmd)
		return nil
	}
	cmd.WaitDelay = 5 * time.Second
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	started := time.Now()
	err := cmd.Start()
	var breach string
	if err == nil {
		watcher := Watch(cmd.Process.Pid, DefaultLimits)
		err = cmd.Wait()
		// A background child left in the group outlives the command; nothing a station ran may keep running.
		KillGroup(cmd)
		if errors.Is(err, exec.ErrWaitDelay) {
			err = nil
		}
		watcher.Stop()
		breach = watcher.Breach()
	}
	result := Result{Output: strings.TrimSpace(output.String()), Seconds: time.Since(started).Seconds()}
	switch {
	case breach != "":
		result.ExitCode = ExitRunaway
		result.Output = strings.TrimSpace(result.Output + "\n[runaway: " + breach + "; the process tree was killed]")
	case ctx.Err() == nil && errors.Is(clock.Err(), context.DeadlineExceeded):
		result.ExitCode, result.TimedOut = ExitTimeout, true
		result.Output = strings.TrimSpace(
			result.Output + fmt.Sprintf("\n[timed out after %s; the process group was killed]", timeout),
		)
	case err != nil:
		result.ExitCode = 1
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			result.ExitCode = exit.ExitCode()
		}
		if result.Output == "" {
			result.Output = err.Error()
		}
	}
	return result
}
