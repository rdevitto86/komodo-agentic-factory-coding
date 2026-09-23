// Package proc runs a shell command under a wall clock, in its own process group, so a hung child never hangs a station.
package proc

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DefaultTimeout is how long a station command may run when nothing narrower is set.
const DefaultTimeout = 10 * time.Minute

// ExitTimeout is the exit code a timed-out command reports, the same one timeout(1) uses.
const ExitTimeout = 124

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
	return fmt.Errorf("exit status %d", r.ExitCode)
}

// Shell runs one sh -c command in dir under timeout, killing its whole process group when the clock runs out.
func Shell(dir, command string, timeout time.Duration) Result {
	return Exec(dir, timeout, "sh", "-c", command)
}

// Exec runs one program in dir under timeout, killing its whole process group when the clock runs out.
func Exec(dir string, timeout time.Duration, name string, args ...string) Result {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	Group(cmd)
	cmd.Cancel = func() error {
		KillGroup(cmd)
		return nil
	}
	cmd.WaitDelay = 5 * time.Second
	started := time.Now()
	output, err := cmd.CombinedOutput()
	result := Result{Output: strings.TrimSpace(string(output)), Seconds: time.Since(started).Seconds()}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.ExitCode, result.TimedOut = ExitTimeout, true
		result.Output = strings.TrimSpace(result.Output + fmt.Sprintf("\n[timed out after %s; the process group was killed]", timeout))
		return result
	}
	if err != nil {
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
