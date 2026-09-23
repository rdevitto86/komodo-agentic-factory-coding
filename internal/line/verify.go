package line

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"komodo/internal/detect"
	"komodo/internal/proc"
	repopkg "komodo/internal/repo"
)

// CommandTimeout is the wall clock a station command gets unless the task names its own.
const CommandTimeout = proc.DefaultTimeout

// GateTimeout is the wall clock the toolkit's own gate gets, since it runs the whole test suite.
const GateTimeout = 20 * time.Minute

// VerifyCommand is the repo's own verify command, from its commands file, else detect's root-only
// discovery order, else go test for a Go module, so QC never drifts from what detect reports.
func VerifyCommand(root string) string {
	if override := repopkg.LoadCommands(root).Verify; override != "" {
		return override
	}
	if command := detect.VerifyCommand(root); command != "" {
		return command
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		return "go test ./..."
	}
	return ""
}

// BeforeReviewCommand is the repo's own command to run before the reviewer is spawned.
func BeforeReviewCommand(root string) string {
	return repopkg.LoadCommands(root).BeforeReview
}

// AfterPublishCommand is the repo's own command to run once a group has shipped.
func AfterPublishCommand(root string) string {
	return repopkg.LoadCommands(root).AfterPublish
}

// CompileCommands are the cheap whole-tree checks for the manifests a repo carries.
func CompileCommands(root string) []string {
	if override := repopkg.LoadCommands(root).Compile; override != "" {
		return []string{override}
	}
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
	ran := proc.Shell(cwd, command, timeout)
	return CommandResult{Command: command, ExitCode: ran.ExitCode, Seconds: ran.Seconds,
		Output: Clip(ran.Output, 12000, "output")}
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
