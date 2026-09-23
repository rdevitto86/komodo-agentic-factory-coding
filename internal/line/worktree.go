package line

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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
	start := StartRef(root, base)
	if _, err := git(root, "rev-parse", "--verify", "refs/heads/"+branch); err == nil {
		_, err := git(root, "worktree", "add", path, branch)
		return err
	}
	_, err := git(root, "worktree", "add", "-b", branch, path, start)
	return err
}

// StartRef is the ref a group is cut from and diffed against: the remote-tracked copy of base
// when it exists, else base itself, so a stale local base never leaks another group's commits in.
func StartRef(dir, base string) string {
	ref := "origin/" + base
	if _, err := git(dir, "rev-parse", "--verify", ref); err != nil {
		return base
	}
	return ref
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

// LockPath is where next --start records the pid and run holding this repo.
func LockPath(root string) string {
	return filepath.Join(root, StateDir, "run.lock")
}

// RunLock is one run's claim on the repo: the process that took it, and what it is running.
type RunLock struct {
	PID int    `json:"pid"`
	Run string `json:"run"`
}

// AcquireLock takes the run lock for run, refusing when a live process already holds it and
// naming the holder, or reclaiming a lock whose process has since exited.
func AcquireLock(root, run string) error {
	if err := CheckLock(root); err != nil {
		return err
	}
	path := LockPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(RunLock{PID: os.Getpid(), Run: run})
	if err != nil {
		return err
	}
	// Exclusive create, so two launchers cannot both take it; a dead holder's lock is removed once.
	for attempt := 0; attempt < 2; attempt++ {
		handle, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			_, err = handle.Write(data)
			return errors.Join(err, handle.Close())
		}
		if !os.IsExist(err) {
			return err
		}
		if checkErr := CheckLock(root); checkErr != nil {
			return checkErr
		}
		_ = os.Remove(path)
	}
	return fmt.Errorf("could not take the run lock at %s", path)
}

// LockEnv carries the launcher's pid into the host it starts, so that host's stations pass the lock.
const LockEnv = "KOMODO_RUN_PID"

// CheckLock refuses when a live process other than this host's own launcher holds the run lock.
func CheckLock(root string) error {
	data, err := os.ReadFile(LockPath(root))
	if err != nil {
		return nil
	}
	var held RunLock
	if json.Unmarshal(data, &held) != nil || held.PID == 0 || !processAlive(held.PID) {
		return nil
	}
	if held.PID == os.Getpid() || strconv.Itoa(held.PID) == os.Getenv(LockEnv) {
		return nil
	}
	return fmt.Errorf("%s already holds the run lock, pid %d", held.Run, held.PID)
}

// ReleaseLock removes the run lock when this process holds it.
func ReleaseLock(root string) {
	data, err := os.ReadFile(LockPath(root))
	if err != nil {
		return
	}
	var held RunLock
	if json.Unmarshal(data, &held) == nil && held.PID == os.Getpid() {
		_ = os.Remove(LockPath(root))
	}
}

// processAlive reports whether a pid still names a running process.
func processAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
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
