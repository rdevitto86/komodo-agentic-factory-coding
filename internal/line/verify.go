package line

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/detect"
	"komodo/internal/guard"
	"komodo/internal/proc"
	repopkg "komodo/internal/repo"
)

// CommandTimeout is the wall clock a station command gets unless the task names its own.
const CommandTimeout = proc.DefaultTimeout

// GateTimeout is the wall clock the toolkit's own gate gets, since it runs the whole test suite.
const GateTimeout = 20 * time.Minute

// VerifyCommand is QC's verify command: the worktree's commands file, the root's, then detection on the worktree.
func VerifyCommand(root, worktree string) string {
	if override := overrideCommands(root, worktree).Verify; override != "" {
		return override
	}
	if found, _ := detect.Detect(worktree); found.Verify != "" {
		return found.Verify
	}
	if cached, ok := detect.LoadCached(root); ok {
		return cached.Verify
	}
	return ""
}

// overrideCommands reads the worktree's commands file, filling each empty field from the root's, since state is gitignored.
func overrideCommands(root, worktree string) repopkg.Commands {
	own := repopkg.LoadCommands(worktree)
	if root == worktree {
		return own
	}
	shared := repopkg.LoadCommands(root)
	if own.Verify == "" {
		own.Verify = shared.Verify
	}
	if own.Compile == "" {
		own.Compile = shared.Compile
	}
	if own.BeforeReview == "" {
		own.BeforeReview = shared.BeforeReview
	}
	if own.AfterPublish == "" {
		own.AfterPublish = shared.AfterPublish
	}
	return own
}

// BeforeReviewCommand is the repo's own command to run before the reviewer is spawned.
func BeforeReviewCommand(root, worktree string) string {
	return overrideCommands(root, worktree).BeforeReview
}

// AfterPublishCommand is the repo's own command to run once a group has shipped.
func AfterPublishCommand(root, worktree string) string {
	return overrideCommands(root, worktree).AfterPublish
}

// CompileCommands are QC's cheap whole-tree checks: a commands file override, else the worktree's manifests, else the root's profile.
func CompileCommands(root, worktree string) []string {
	if override := overrideCommands(root, worktree).Compile; override != "" {
		return []string{override}
	}
	commands := manifestCompiles(worktree)
	if len(commands) == 0 {
		if cached, ok := detect.LoadCached(root); ok && cached.Compile != "" {
			commands = append(commands, cached.Compile)
		}
	}
	return commands
}

// manifestCompiles are the compile checks for the manifests root carries.
func manifestCompiles(root string) []string {
	var commands []string
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		commands = append(commands, "go build ./... && go vet ./...")
	}
	if _, err := os.Stat(filepath.Join(root, "package.json")); err == nil {
		if _, err := os.Stat(filepath.Join(root, "tsconfig.json")); err == nil {
			commands = append(commands, "npx tsc --noEmit -p .")
		}
	}
	for _, manifest := range []string{"pyproject.toml", "setup.py"} {
		if _, err := os.Stat(filepath.Join(root, manifest)); err == nil {
			commands = append(commands, "python3 -m compileall -q .")
			break
		}
	}
	return commands
}

// CommandResult is one command's outcome, with its output trimmed for a report.
type CommandResult struct {
	Command  string  `json:"command"`
	ExitCode int     `json:"exit_code"`
	Output   string  `json:"output,omitempty"`
	Seconds  float64 `json:"seconds"`
}

// OK reports whether the command exited zero.
func (r CommandResult) OK() bool { return r.ExitCode == 0 }

// RunCommand runs one shell command in a directory under the default wall clock and captures its output.
func RunCommand(cwd, command string) CommandResult {
	return RunCommandFor(cwd, command, CommandTimeout)
}

// RunCommandFor runs one shell command in a directory under timeout, killing its process group when it hangs.
func RunCommandFor(cwd, command string, timeout time.Duration) CommandResult {
	return runGuarded(cwd, command, timeout, nil)
}

// RunCommandEnv runs one shell command under the default wall clock in the given environment.
func RunCommandEnv(cwd, command string, env []string) CommandResult {
	return runGuarded(cwd, command, CommandTimeout, env)
}

// runGuarded runs a command a model may have written only once the guard allows it, as the hook would.
func runGuarded(cwd, command string, timeout time.Duration, env []string) CommandResult {
	if refused := guardRefusal(cwd, command); refused != "" {
		return CommandResult{Command: command, ExitCode: guard.ExitDeny, Output: refused}
	}
	ran := proc.ShellEnv(cwd, command, timeout, env)
	return CommandResult{Command: command, ExitCode: ran.ExitCode, Seconds: ran.Seconds,
		Output: Clip(ran.Output, 12000, "output")}
}

// guardRefusal is the guard's reason to refuse a command the line runs in cwd, or empty when it may run.
func guardRefusal(cwd, command string) string {
	root := guard.WorktreeRoot(cwd)
	decision := guard.CheckCommand(command, cwd, guard.Load(root, root))
	if !decision.Deny {
		return ""
	}
	return "refused by the guard: " + strings.Join(decision.Findings, "; ")
}

// RunGate runs commands in order and stops at the first failure.
func RunGate(cwd string, commands []string) []CommandResult {
	var results []CommandResult
	for _, command := range commands {
		result := RunCommand(cwd, command)
		results = append(results, result)
		if !result.OK() {
			break
		}
	}
	return results
}

// FirstFailure names the first command that did not exit zero.
func FirstFailure(results []CommandResult) (CommandResult, bool) {
	for _, result := range results {
		if !result.OK() {
			return result, true
		}
	}
	return CommandResult{}, false
}

// FailureText renders a failed command for the failure slot.
func FailureText(result CommandResult) string {
	return fmt.Sprintf("`%s` exited %d\n\n%s", result.Command, result.ExitCode, result.Output)
}
