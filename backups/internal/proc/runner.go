package proc

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Runner runs shell commands and programs; swap it for a fake so a test never shells out.
type Runner interface {
	// Exec runs one program in dir under timeout, as Exec does.
	Exec(dir string, timeout time.Duration, name string, args ...string) Result
	// ExecContext is Exec whose process group is also killed once ctx is done, as ExecContext does.
	ExecContext(ctx context.Context, dir string, timeout time.Duration, name string, args ...string) Result
	// Shell runs one shell command in dir under timeout, as Shell does.
	Shell(dir, command string, timeout time.Duration) Result
}

// execRunner is the Runner a live process runs on: the package's own Exec, ExecContext and Shell.
type execRunner struct{}

// Exec runs name in dir under timeout.
func (execRunner) Exec(dir string, timeout time.Duration, name string, args ...string) Result {
	return Exec(dir, timeout, name, args...)
}

// ExecContext runs name in dir under timeout, killed early once ctx is done.
func (execRunner) ExecContext(
	ctx context.Context, dir string, timeout time.Duration, name string, args ...string,
) Result {
	return ExecContext(ctx, dir, timeout, name, args...)
}

// Shell runs command in dir under timeout.
func (execRunner) Shell(dir, command string, timeout time.Duration) Result {
	return Shell(dir, command, timeout)
}

// DefaultRunner is the Runner every real process uses: the host's own git, gh and model CLIs.
var DefaultRunner Runner = execRunner{}

// Call is one command a FakeRunner's Exec, ExecContext or Shell recorded.
type Call struct {
	Dir     string
	Timeout time.Duration
	Name    string
	Args    []string
}

// FakeRunner is a Runner a test drives with no real process, answering each call from Results in order.
type FakeRunner struct {
	// Results is each call's answer, in call order, shared across Exec, ExecContext and Shell.
	Results []Result

	mu    sync.Mutex
	calls []Call
}

// next returns call's recorded answer, and files call under Calls.
func (f *FakeRunner) next(call Call) Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	index := len(f.calls) - 1
	if index < len(f.Results) {
		return f.Results[index]
	}
	return Result{}
}

// Exec records the call and returns its answer, with no real process.
func (f *FakeRunner) Exec(dir string, timeout time.Duration, name string, args ...string) Result {
	return f.next(Call{Dir: dir, Timeout: timeout, Name: name, Args: args})
}

// ExecContext records the call and returns its answer, with no real process; a cancelled ctx is
// still answered, since nothing it would kill ever started.
func (f *FakeRunner) ExecContext(
	ctx context.Context, dir string, timeout time.Duration, name string, args ...string,
) Result {
	if err := ctx.Err(); err != nil {
		return Result{ExitCode: 1, Output: err.Error()}
	}
	return f.next(Call{Dir: dir, Timeout: timeout, Name: name, Args: args})
}

// Shell records the call and returns its answer, with no real process.
func (f *FakeRunner) Shell(dir, command string, timeout time.Duration) Result {
	return f.next(Call{Dir: dir, Timeout: timeout, Name: fmt.Sprintf("sh -c %q", command)})
}

// Calls returns every call Exec, ExecContext or Shell recorded, in call order.
func (f *FakeRunner) Calls() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Call(nil), f.calls...)
}
