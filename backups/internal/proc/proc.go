// Package proc runs a shell command under a wall clock, in its own process group, so a hung child never hangs a station.
package proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// DefaultTimeout is how long a station command may run when nothing narrower is set.
const DefaultTimeout = 10 * time.Minute

// MaxOutput is how many bytes of a subprocess's output any BoundedWriter keeps.
const MaxOutput = 4 << 20

// ExitTimeout is the exit code a timed-out command reports, the same one timeout(1) uses.
const ExitTimeout = 124

// ExitRunaway is the exit code a command reports when its process tree broke DefaultLimits and was killed.
const ExitRunaway = 125

// ExitNotExecutable and ExitNotFound are the shell's codes for a program that cannot run or does not exist.
const (
	ExitNotExecutable = 126
	ExitNotFound      = 127
)

// BoundedWriter keeps up to Limit bytes of what it is written, discarding the rest.
type BoundedWriter struct {
	Limit int
	buf   bytes.Buffer
}

// NewBoundedWriter returns a writer that keeps at most limit bytes of what it is written.
func NewBoundedWriter(limit int) *BoundedWriter {
	return &BoundedWriter{Limit: limit}
}

// Write keeps up to the writer's limit and reports every byte as written, so a caller never blocks or errors.
func (w *BoundedWriter) Write(p []byte) (int, error) {
	if room := w.Limit - w.buf.Len(); room > 0 {
		if room > len(p) {
			room = len(p)
		}
		w.buf.Write(p[:room])
	}
	return len(p), nil
}

// String returns the bytes the writer has kept so far.
func (w *BoundedWriter) String() string {
	return w.buf.String()
}

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

// plainWord is a word no shell would change: no operator, quote, expansion, glob or assignment.
var plainWord = regexp.MustCompile(`^[A-Za-z0-9_./:@%+,-]+$`)

// shellBuiltins are the commands only a shell runs; a plain line starting with one still needs the shell.
var shellBuiltins = map[string]bool{
	":": true, ".": true, "alias": true, "bg": true, "break": true, "cd": true, "command": true, "continue": true,
	"eval": true, "exec": true, "exit": true, "export": true, "fg": true, "getopts": true, "hash": true, "jobs": true,
	"local": true, "read": true, "readonly": true, "return": true, "set": true, "shift": true, "source": true,
	"times": true, "trap": true, "type": true, "ulimit": true, "umask": true, "unalias": true, "unset": true, "wait": true,
}

// ShellArgv runs a plain command line as its own argv with no shell; shell syntax or a builtin goes through
// POSIX sh, which native Windows takes from Git for Windows, else cmd /C.
func ShellArgv(command string) []string {
	fields := strings.Fields(command)
	plain := len(fields) > 0 && !shellBuiltins[fields[0]]
	for _, field := range fields {
		plain = plain && plainWord.MatchString(field)
	}
	if plain {
		return fields
	}
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
	output := NewBoundedWriter(MaxOutput)
	cmd.Stdout, cmd.Stderr = output, output
	started := time.Now()
	err := cmd.Start()
	var breach string
	if err == nil {
		// A process group forms atomically at fork on Unix; Windows can only join its job once the process exists.
		Started(cmd)
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
		switch {
		case errors.As(err, &exit):
			result.ExitCode = exit.ExitCode()
		case errors.Is(err, exec.ErrNotFound):
			result.ExitCode = ExitNotFound
		case errors.Is(err, fs.ErrPermission):
			result.ExitCode = ExitNotExecutable
		}
		if result.Output == "" {
			result.Output = err.Error()
		}
	}
	return result
}
