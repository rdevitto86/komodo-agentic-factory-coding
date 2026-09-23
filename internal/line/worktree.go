package line

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// StateDir is where a run's own files live, gitignored, never committed.
const StateDir = ".komodo"

// RunState records the base and branch a run chose so close and ship target the same one.
type RunState struct {
	Run      string     `json:"run"`
	Group    string     `json:"group"`
	Base     string     `json:"base"`
	Branch   string     `json:"branch"`
	Worktree string     `json:"worktree"`
	Waves    [][]string `json:"waves,omitempty"`
	Started  time.Time  `json:"started"`
}

// git runs one git command in dir and returns its trimmed stdout.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// DefaultBase is the remote's default branch, falling back to main.
func DefaultBase(root string) string {
	head, err := git(root, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
	if err == nil && head != "" {
		return strings.TrimPrefix(head, "refs/remotes/origin/")
	}
	return "main"
}

// Fetch updates the remote ref for one branch.
func Fetch(root, base string) error {
	_, err := git(root, "fetch", "origin", base)
	return err
}

// BranchName is the branch a group's work lands on: its type and its slug.
func BranchName(groupType, slug string) string { return groupType + "/" + slug }

// AddWorktree creates branch at the base's head in its own worktree under .komodo/wt.
func AddWorktree(root, branch, base, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	start := "origin/" + base
	if _, err := git(root, "rev-parse", "--verify", start); err != nil {
		start = base
	}
	if _, err := git(root, "rev-parse", "--verify", "refs/heads/"+branch); err == nil {
		_, err := git(root, "worktree", "add", path, branch)
		return err
	}
	_, err := git(root, "worktree", "add", "-b", branch, path, start)
	return err
}

// WorktreePath resolves a plan's worktree: a run records it absolute, a plan builds it relative to the root.
func WorktreePath(root, worktree string) string {
	if worktree == "" {
		return root
	}
	if filepath.IsAbs(worktree) {
		return worktree
	}
	return filepath.Join(root, worktree)
}

// SaveRun writes the run's state under .komodo so every later station reads the same choices.
func SaveRun(root string, state RunState) error {
	dir := filepath.Join(root, StateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "run.json"), append(data, '\n'), 0o644)
}

// LoadRun reads the run's state, if a run has started.
func LoadRun(root string) (RunState, error) {
	var state RunState
	data, err := os.ReadFile(filepath.Join(root, StateDir, "run.json"))
	if err != nil {
		return state, err
	}
	return state, json.Unmarshal(data, &state)
}

// ResultPath is where a task's result JSON lands.
func ResultPath(root, taskID string) string {
	return filepath.Join(root, StateDir, "results", taskID+".json")
}

// ResultPaths are the places a result can be: the task's worktree, the group's, then the root.
func ResultPaths(root, taskID string) []string {
	rootPath := ResultPath(root, taskID)
	var paths []string
	add := func(base string) {
		if base == "" {
			return
		}
		candidate := ResultPath(base, taskID)
		if candidate == rootPath || contains(paths, candidate) {
			return
		}
		paths = append(paths, candidate)
	}
	add(filepath.Join(root, StateDir, "wt", taskID))
	if state, err := LoadRun(root); err == nil {
		add(state.Worktree)
	}
	return append(paths, rootPath)
}

// ReadResultFile returns the first result that exists for a task, and where it was found.
func ReadResultFile(root, taskID string) ([]byte, string, error) {
	var err error
	for _, path := range ResultPaths(root, taskID) {
		var data []byte
		data, err = os.ReadFile(path)
		if err == nil {
			return data, path, nil
		}
	}
	return nil, "", err
}

// HasResult reports whether a task already has a result on disk that parses, which is resume.
func HasResult(root, taskID string) bool {
	data, _, err := ReadResultFile(root, taskID)
	if err != nil {
		return false
	}
	var parsed map[string]any
	return json.Unmarshal(data, &parsed) == nil
}
