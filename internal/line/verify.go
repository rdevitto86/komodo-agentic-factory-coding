package line

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RepoCommands are the commands a repo may override at each station.
type RepoCommands struct {
	Verify       string `json:"verify"`
	Compile      string `json:"compile"`
	BeforeReview string `json:"before_review"`
	AfterPublish string `json:"after_publish"`
}

// verifyOrder is the discovery order V1 used, first hit wins.
var verifyOrder = []struct{ file, command string }{
	{".komodo/verify.sh", "sh .komodo/verify.sh"},
	{"scripts/verify.sh", "sh scripts/verify.sh"},
	{"Makefile", "make verify"},
	{"Taskfile.yml", "task verify"},
	{"Taskfile.yaml", "task verify"},
	{"justfile", "just verify"},
}

// LoadRepoCommands reads .komodo/commands.json, which may be absent.
func LoadRepoCommands(root string) RepoCommands {
	var commands RepoCommands
	data, err := os.ReadFile(filepath.Join(root, StateDir, "commands.json"))
	if err != nil {
		return commands
	}
	_ = json.Unmarshal(data, &commands)
	return commands
}

// VerifyCommand is the repo's own verify command, from its commands file or the discovery order.
func VerifyCommand(root string) string {
	if override := LoadRepoCommands(root).Verify; override != "" {
		return override
	}
	for _, candidate := range verifyOrder {
		if _, err := os.Stat(filepath.Join(root, candidate.file)); err == nil {
			return candidate.command
		}
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		return "go test ./..."
	}
	return ""
}

// CompileCommands are the cheap whole-tree checks for the manifests a repo carries.
func CompileCommands(root string) []string {
	if override := LoadRepoCommands(root).Compile; override != "" {
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

// RunCommand runs one shell command in a directory and captures its combined output.
func RunCommand(cwd, command string) CommandResult {
	started := time.Now()
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	result := CommandResult{Command: command, Seconds: time.Since(started).Seconds()}
	result.Output = Clip(strings.TrimSpace(string(output)), 12000, "output")
	if err != nil {
		result.ExitCode = 1
		if exit, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exit.ExitCode()
		}
	}
	return result
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
