package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/conductor"
	"komodo/internal/line"
	"komodo/internal/proc"
	"komodo/internal/run"
)

// HiddenTestTimeout is how long a group's hidden tests may run before they count as failed.
const HiddenTestTimeout = 10 * time.Minute

// cloneTimeout bounds each git command that builds a run's fresh clone.
const cloneTimeout = 10 * time.Minute

// evalIdentity is who commits the group file and the mount into a run's clone.
var evalIdentity = []string{"-c", "user.name=komodo eval", "-c", "user.email=eval@komodo.invalid"}

// Clone builds a fresh clone of url in dir at commit, holding nothing later than commit.
type Clone func(ctx context.Context, url, commit, dir string) error

// Line drives one group through the whole line in dir, stopping at Ship with no push.
type Line func(ctx context.Context, dir string, group Group) error

// Options are what an eval needs: the suite, the runs per group, where clones go, and the line to drive.
type Options struct {
	Suite Suite
	Runs  int
	// Work holds one fresh clone per run of each group.
	Work     string
	Platform string
	// Budget bounds one run of one group; zero is run.GroupBudget.
	Budget time.Duration
	Clone  Clone
	Line   Line
	Stdout io.Writer
}

// Outcome is one run of one group: whether it reached Ship, whether its hidden tests passed, and its cost.
type Outcome struct {
	Repo         string  `json:"repo"`
	Group        string  `json:"group"`
	Run          int     `json:"run"`
	Platform     string  `json:"platform"`
	Shipped      bool    `json:"shipped"`
	Passed       bool    `json:"passed"`
	Sessions     int     `json:"sessions"`
	Turns        int     `json:"turns"`
	Tokens       int     `json:"tokens"`
	Minutes      float64 `json:"minutes"`
	ReviewRounds int     `json:"review_rounds"`
	Error        string  `json:"error,omitempty"`
}

// Accepted reports whether the run shipped and its hidden tests passed.
func (o Outcome) Accepted() bool { return o.Shipped && o.Passed }

// Run drives every group in the suite Runs times, each in a fresh clone, and returns every run's outcome;
// it stops only when a clone cannot be built, since a line that fails is a measured outcome.
func Run(ctx context.Context, options Options) ([]Outcome, error) {
	if options.Line == nil {
		return nil, errors.New("an eval needs a line to drive")
	}
	if options.Runs < 1 {
		return nil, fmt.Errorf("an eval needs at least one run per group, not %d", options.Runs)
	}
	if options.Clone == nil {
		options.Clone = CloneAt
	}
	if options.Platform == "" {
		options.Platform = runtime.GOOS
	}
	if options.Budget <= 0 {
		options.Budget = run.GroupBudget
	}
	if options.Stdout == nil {
		options.Stdout = io.Discard
	}
	outcomes := make([]Outcome, 0, len(options.Suite.Groups)*options.Runs)
	for _, group := range options.Suite.Groups {
		for index := 1; index <= options.Runs; index++ {
			outcome, err := runOnce(ctx, options, group, index)
			if err != nil {
				return outcomes, err
			}
			verdict := "rejected"
			if outcome.Accepted() {
				verdict = "accepted"
			}
			fmt.Fprintf(options.Stdout, "%s/%s run %d on %s: %s\n", group.Repo, group.ID, index, options.Platform, verdict)
			outcomes = append(outcomes, outcome)
		}
	}
	return outcomes, nil
}

// runOnce clones the group's repo at its pinned commit, places the group file, drives the line, and runs the
// hidden tests in the worktree the ship handed off.
func runOnce(ctx context.Context, options Options, group Group, index int) (Outcome, error) {
	repo := options.Suite.Repo(group)
	outcome := Outcome{Repo: repo.Name, Group: group.ID, Run: index, Platform: options.Platform}
	dir := filepath.Join(options.Work, fmt.Sprintf("%s-%s-%d", repo.Name, group.ID, index))
	if err := options.Clone(ctx, repo.URL, group.Commit, dir); err != nil {
		return outcome, fmt.Errorf("%s/%s: clone: %w", repo.Name, group.ID, err)
	}
	if err := appendGroup(dir, filepath.Join(options.Suite.Dir, group.File)); err != nil {
		return outcome, fmt.Errorf("%s/%s: %w", repo.Name, group.ID, err)
	}

	started := time.Now()
	lineCtx, cancel := context.WithTimeout(ctx, options.Budget)
	lineErr := options.Line(lineCtx, dir, group)
	cancel()
	outcome.Minutes = time.Since(started).Minutes()
	if lineErr != nil {
		outcome.Error = lineErr.Error()
	}
	outcome.measure(dir)

	worktree, shipped, err := shippedWorktree(dir, group.ID)
	if err != nil {
		outcome.Error = err.Error()
		return outcome, nil
	}
	if !shipped {
		return outcome, nil
	}
	outcome.Shipped = true
	for _, test := range group.HiddenTests {
		if err := copyFile(filepath.Join(options.Suite.Dir, group.Hidden, test), filepath.Join(worktree, test)); err != nil {
			return outcome, fmt.Errorf("%s/%s: %w", repo.Name, group.ID, err)
		}
	}
	result := line.RunCommandFor(worktree, group.Test, HiddenTestTimeout)
	outcome.Passed = result.OK()
	if !outcome.Passed {
		outcome.Error = "hidden tests: " + line.FailureText(result)
	}
	return outcome, nil
}

// measure fills the sessions, turns, tokens and review rounds the clone's ledger recorded.
func (o *Outcome) measure(dir string) {
	entries, err := line.Book(dir).All()
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.Role != "" {
			o.Sessions++
		}
		o.Turns += entry.Turns
		o.Tokens += entry.TokensIn + entry.TokensOut
		if entry.Station == conductor.StationReview || entry.Station == conductor.StationReReview {
			o.ReviewRounds++
		}
	}
}

// shippedWorktree reads the ship handoff the group left, returning its worktree when the ship reached it;
// a session can write the handoff, so a worktree outside the clone is refused.
func shippedWorktree(dir, group string) (string, bool, error) {
	data, err := os.ReadFile(line.HandoffPath(dir, group))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	var handoff line.ShipHandoff
	if err := json.Unmarshal(data, &handoff); err != nil {
		return "", false, fmt.Errorf("the ship handoff: %w", err)
	}
	worktree := handoff.Worktree
	if worktree == "" {
		worktree = dir
	}
	if !filepath.IsAbs(worktree) {
		worktree = filepath.Join(dir, worktree)
	}
	// The line may record a worktree through a resolved symlink, such as /private/var for /var.
	realDir, dirErr := filepath.EvalSymlinks(dir)
	realWorktree, worktreeErr := filepath.EvalSymlinks(worktree)
	if dirErr != nil || worktreeErr != nil {
		return "", false, fmt.Errorf("the ship handoff names worktree %s, which does not exist", handoff.Worktree)
	}
	rel, err := filepath.Rel(realDir, realWorktree)
	if err != nil || !filepath.IsLocal(rel) {
		return "", false, fmt.Errorf("the ship handoff names worktree %s, outside the clone", handoff.Worktree)
	}
	return worktree, true, nil
}

// appendGroup appends the group file to the clone's backlog, starting a root BACKLOG.md when it has none.
func appendGroup(dir, groupFile string) error {
	text, err := os.ReadFile(groupFile)
	if err != nil {
		return err
	}
	path, err := backlog.Find(dir)
	if err != nil {
		path = filepath.Join(dir, "BACKLOG.md")
		if err := os.WriteFile(path, []byte("# Backlog\n"), 0o644); err != nil {
			return err
		}
	}
	handle, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer handle.Close()
	_, err = handle.Write(append([]byte("\n"), text...))
	return err
}

// copyFile copies one file to target, making its directories.
func copyFile(source, target string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}

// CloneAt fetches only commit and its history from url into a new repo at dir on main, then points origin
// at a local bare copy, so the change's own commits stay hidden and every push stays on this machine.
func CloneAt(ctx context.Context, url, commit, dir string) error {
	origin := dir + ".origin.git"
	steps := [][]string{
		{"init", "-q", dir},
		{"-C", dir, "fetch", "-q", url, commit},
		{"-C", dir, "checkout", "-q", "-B", "main", "FETCH_HEAD"},
		{"clone", "-q", "--bare", dir, origin},
		{"-C", dir, "remote", "add", "origin", origin},
		{"-C", dir, "fetch", "-q", "origin"},
		{"-C", dir, "branch", "-q", "--set-upstream-to", "origin/main"},
	}
	for _, args := range steps {
		if err := command(ctx, "", cloneTimeout, "git", args...); err != nil {
			return err
		}
	}
	return nil
}

// Launch is the live line: it mounts host in the clone (the default host when empty), commits the group
// file and the mount, and runs the group with executable, stopping at Ship with no push.
func Launch(executable, host string, budget time.Duration) Line {
	return func(ctx context.Context, dir string, group Group) error {
		install := []string{"install"}
		if host != "" {
			install = append(install, "--host", host)
		}
		commit := append(append([]string{}, evalIdentity...), "commit", "-q", "-m", "eval: "+group.ID)
		steps := []struct {
			name string
			args []string
		}{
			{executable, install},
			{"git", []string{"add", "-A"}},
			{"git", commit},
			{"git", []string{"push", "-q", "origin", "main"}},
			{executable, []string{"run", group.ID, "--no-ship", "--budget", budget.String()}},
		}
		for _, step := range steps {
			if err := command(ctx, dir, 0, step.name, step.args...); err != nil {
				return err
			}
		}
		return nil
	}
}

// command runs one program in dir, killing its process tree when ctx ends or timeout passes, and returns
// an error carrying the program's output when it fails.
func command(ctx context.Context, dir string, timeout time.Duration, name string, args ...string) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	proc.Group(cmd)
	cmd.Cancel = func() error {
		proc.KillGroup(cmd)
		return nil
	}
	cmd.WaitDelay = 5 * time.Second
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, line.Clip(output.String(), 4000, "output"))
	}
	return nil
}
