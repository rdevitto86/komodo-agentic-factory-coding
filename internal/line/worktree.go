package line

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/git"
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

// DefaultBase is the remote's default branch, falling back to main.
func DefaultBase(root string) string {
	head, err := git.Run(root, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
	if err == nil && head != "" {
		return strings.TrimPrefix(head, "refs/remotes/origin/")
	}
	return "main"
}

// Fetch updates the remote ref for one branch.
func Fetch(root, base string) error {
	_, err := git.Run(root, "fetch", "origin", base)
	return err
}

// BranchName is the branch a group's work lands on: its type and its slug.
func BranchName(groupType, slug string) string { return groupType + "/" + slug }

// RefusedPushURL is the pushurl that makes git push origin fail inside a line worktree.
const RefusedPushURL = "refused://the-line-pushes"

// AddWorktree cuts branch from base into its own worktree, whose scoped pushurl refuses git push origin.
func AddWorktree(root, branch, base, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	start := StartRef(root, base)
	if _, err := git.Run(root, "rev-parse", "--verify", "refs/heads/"+branch); err == nil {
		if _, err := git.Run(root, "worktree", "add", path, branch); err != nil {
			return err
		}
	} else if _, err := git.Run(root, "worktree", "add", "-b", branch, path, start); err != nil {
		return err
	}
	if err := refuseWorktreePush(root, path); err != nil {
		// A retry recuts a missing worktree, so a cut without its refusal is never left behind.
		_, _ = git.Run(root, "worktree", "remove", "--force", path)
		return err
	}
	return nil
}

// refuseWorktreePush sets path's pushurl to RefusedPushURL, erroring when it cannot; core.bare
// and core.worktree repos cannot hold worktree config, so those only note a skip.
func refuseWorktreePush(root, path string) error {
	if bare, err := git.Run(root, "config", "--get", "core.bare"); err == nil && bare == "true" {
		fmt.Fprintln(os.Stderr, "komodo: core.bare is true on the repo's common config; skipping the worktree push refusal")
		return nil
	}
	if _, err := git.Run(root, "config", "--get", "core.worktree"); err == nil {
		fmt.Fprintln(os.Stderr, "komodo: core.worktree is set on the repo's common config; skipping the worktree push refusal")
		return nil
	}
	// Written only when not already on locally, since concurrent cuts race on the shared config's lock.
	if on, err := git.Run(root, "config", "--local", "--get", "extensions.worktreeConfig"); err != nil || on != "true" {
		if _, err := git.Run(root, "config", "extensions.worktreeConfig", "true"); err != nil {
			return fmt.Errorf("enable extensions.worktreeConfig for the worktree push refusal: %w", err)
		}
	}
	if _, err := git.Run(path, "config", "--worktree", "remote.origin.pushurl", RefusedPushURL); err != nil {
		return fmt.Errorf("set the worktree's refused pushurl: %w", err)
	}
	return nil
}

// StartRef is the ref a group is cut from and diffed against: the remote-tracked copy of base
// when it exists, else base itself, so a stale local base never leaks another group's commits in.
func StartRef(dir, base string) string {
	ref := "origin/" + base
	if _, err := git.Run(dir, "rev-parse", "--verify", ref); err != nil {
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

// RunsDir holds one directory per group a run has cut, under StateDir.
const RunsDir = "runs"

// RunDir is where one group's run keeps its record, lock, ship handoff, and live status.
func RunDir(root, group string) string {
	return filepath.Join(root, StateDir, RunsDir, group)
}

// PlainGroup reports whether a group id can name its own run directory: not empty, a dot entry, or a path.
func PlainGroup(group string) bool {
	return group != "" && group != "." && group != ".." && !strings.ContainsAny(group, `/\`)
}

// HandoffPath is where a scrubbed ship leaves one group's push for the launcher.
func HandoffPath(root, group string) string {
	return filepath.Join(RunDir(root, group), "ship.json")
}

// SaveRun writes a group's run state under its run directory so every later station reads the same choices.
func SaveRun(root string, state RunState) error {
	if !PlainGroup(state.Group) {
		return fmt.Errorf("a run needs a plain group id, not %q", state.Group)
	}
	dir := RunDir(root, state.Group)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "run.json"), append(data, '\n'), 0o644)
}

// LoadRunFor reads one group's run state, if that group has been cut.
func LoadRunFor(root, group string) (RunState, error) {
	migrateRun(root)
	var state RunState
	if !PlainGroup(group) {
		return state, os.ErrNotExist
	}
	data, err := os.ReadFile(filepath.Join(RunDir(root, group), "run.json"))
	if err != nil {
		return state, err
	}
	return state, json.Unmarshal(data, &state)
}

// LoadRuns reads every group's run state, oldest start first.
func LoadRuns(root string) []RunState {
	migrateRun(root)
	entries, err := os.ReadDir(filepath.Join(root, StateDir, RunsDir))
	if err != nil {
		return nil
	}
	var runs []RunState
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if state, err := LoadRunFor(root, entry.Name()); err == nil && state.Group != "" {
			runs = append(runs, state)
		}
	}
	sort.SliceStable(runs, func(i, j int) bool {
		if !runs[i].Started.Equal(runs[j].Started) {
			return runs[i].Started.Before(runs[j].Started)
		}
		return runs[i].Group < runs[j].Group
	})
	return runs
}

// LoadRun reads the most recently started run's state, if any run has started.
func LoadRun(root string) (RunState, error) {
	runs := LoadRuns(root)
	if len(runs) == 0 {
		return RunState{}, os.ErrNotExist
	}
	return runs[len(runs)-1], nil
}

// RunFor reads the run an id belongs to: a group, a task, or a review or fix pseudo-task; an id
// naming no group falls back to LoadRun.
func RunFor(root, id string) (RunState, error) {
	group := GroupFor(root, id)
	if group == "" {
		return LoadRun(root)
	}
	return LoadRunFor(root, group)
}

// GroupFor names the group an id belongs to: a group itself, a task's group, or a pseudo-task's group.
func GroupFor(root, id string) string {
	if id == "" {
		return ""
	}
	var parsed backlog.Backlog
	if path, err := backlog.Find(root); err == nil {
		parsed, _ = backlog.Load(path)
	}
	if task, ok := parsed.Task(id); ok {
		return task.GroupID
	}
	for _, suffix := range []string{"", "-review", "-fix"} {
		group, found := strings.CutSuffix(id, suffix)
		if !found || group == "" {
			continue
		}
		for _, candidate := range parsed.Groups {
			if candidate.ID == group {
				return group
			}
		}
		if info, err := os.Stat(RunDir(root, group)); err == nil && info.IsDir() && PlainGroup(group) {
			return group
		}
	}
	if group, ok := parsed.Group(id); ok {
		return group.ID
	}
	return ""
}

// migrateRun moves a legacy .komodo/run.json, with its ship handoff and live status, under its group's directory once.
func migrateRun(root string) {
	legacy := filepath.Join(root, StateDir, "run.json")
	data, err := os.ReadFile(legacy)
	if err != nil {
		return
	}
	var state RunState
	if json.Unmarshal(data, &state) != nil || !PlainGroup(state.Group) {
		return
	}
	dir := RunDir(root, state.Group)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	for _, name := range []string{"ship.json", "status.json", "run.json"} {
		from, to := filepath.Join(root, StateDir, name), filepath.Join(dir, name)
		if _, err := os.Stat(from); err != nil {
			continue
		}
		if _, err := os.Stat(to); err == nil {
			continue
		}
		_ = os.Rename(from, to)
	}
	// A group already holding its own record keeps it; the stale legacy copy goes.
	_ = os.Remove(legacy)
}

// LockPath is where a run records the pid holding one group, or the whole repo when the group is empty.
func LockPath(root, group string) string {
	if group == "" {
		return filepath.Join(root, StateDir, "run.lock")
	}
	return filepath.Join(RunDir(root, group), "run.lock")
}

// RunLock is one run's claim on the repo: the process that took it, and what it is running.
type RunLock struct {
	PID int    `json:"pid"`
	Run string `json:"run"`
}

// AcquireLock takes a group's run lock, or the repo's with no group, for run, refusing when a live
// process already holds it and naming the holder, or reclaiming a lock whose process has since exited.
func AcquireLock(root, group, run string) error {
	if err := CheckLock(root, group); err != nil {
		return err
	}
	path := LockPath(root, group)
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
		if checkErr := CheckLock(root, group); checkErr != nil {
			return checkErr
		}
		_ = os.Remove(path)
	}
	return fmt.Errorf("could not take the run lock at %s", path)
}

// LockEnv carries the launcher's pid into the host it starts, so that host's stations pass the lock.
const LockEnv = "KOMODO_RUN_PID"

// CheckLock refuses when a live process other than this host's own launcher holds a lock that
// covers group: its own and the repo's, or, with no group, every lock.
func CheckLock(root, group string) error {
	paths := []string{LockPath(root, "")}
	if group != "" {
		paths = append(paths, LockPath(root, group))
	} else if entries, err := os.ReadDir(filepath.Join(root, StateDir, RunsDir)); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				paths = append(paths, LockPath(root, entry.Name()))
			}
		}
	}
	for _, path := range paths {
		if err := checkLockFile(path); err != nil {
			return err
		}
	}
	return nil
}

// checkLockFile refuses when a live process other than this host's own launcher holds the lock at path.
func checkLockFile(path string) error {
	data, err := os.ReadFile(path)
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

// ReleaseLock removes a group's run lock, or the repo's with no group, when this process holds it.
func ReleaseLock(root, group string) {
	path := LockPath(root, group)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var held RunLock
	if json.Unmarshal(data, &held) == nil && held.PID == os.Getpid() {
		_ = os.Remove(path)
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

// ResultPaths are the places a result can be: the task's worktree, each run's group worktree, then the root.
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
	// A task id is unique across groups, so every open group's worktree is searched.
	for _, state := range LoadRuns(root) {
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
