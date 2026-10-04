// Package run is the headless launcher: it scrubs the environment and drives a host non-interactively.
package run

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/conductor"
	"komodo/internal/guard"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/pr"
	"komodo/internal/profile"
)

// GroupBudget is how long one headless group may run before the launcher kills it.
const GroupBudget = 90 * time.Minute

// Options are what one headless run needs: where, what, and how long.
type Options struct {
	Root   string
	Target string
	Budget time.Duration
	DryRun bool
	// NoShip stops each group at shipped-ready, skipping the push and the draft pull request.
	NoShip     bool
	Env        []string
	Stdout     io.Writer
	Stderr     io.Writer
	PR         *pr.Client
	Executable string
}

// Launch drives one target through Drive, or prints its plan with no model on a dry run; with no
// target it drains every ready group in order.
func Launch(options Options) (int, error) {
	markLine()
	if options.Target == "" {
		return drain(options)
	}
	if options.DryRun {
		return dryRunTarget(options)
	}
	return Drive(options)
}

// dryRunTarget prints the one group a targeted run would drive, launching no model.
func dryRunTarget(options Options) (int, error) {
	stdout := options.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	fmt.Fprintf(stdout, "1. %s\n", options.Target)
	return 0, nil
}

// LineRole is the marker every session komodo run starts inherits when no role of its own replaces it.
const LineRole = "line"

// markLine puts this process, and every session and subagent it starts, under the guard's line tier.
func markLine() {
	if os.Getenv(guard.RoleEnv) == "" {
		_ = os.Setenv(guard.RoleEnv, LineRole)
	}
}

// drain launches each ready group once, as many at once as the plan and their files allow, parking any
// that ends unshipped, until nothing is ready or the whole budget is spent, printing one line per group.
func drain(options Options) (int, error) {
	stdout := options.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	if options.DryRun {
		return 0, listDrain(options.Root, stdout)
	}
	total, err := drainBudget(options.Root, options.Budget)
	if err != nil {
		return 1, err
	}
	// Sync once before the run starts, and never again until it ends, so no group's binary moves under it.
	executable, err := Sync(SyncOptions{Root: options.Root, Stdout: stdout})
	if err != nil {
		return 1, err
	}
	// Every lane's session inherits this PATH, so a rebuild mid-drain never moves what one runs.
	if err := pinDrainPath(options.Root, executable); err != nil {
		return 1, err
	}
	// Every lane writes to the same output, so each write holds the one lock.
	var outputLock sync.Mutex
	stdout = lockedWriter{lock: &outputLock, out: stdout}
	options.Stdout = stdout
	if options.Stderr != nil {
		options.Stderr = lockedWriter{lock: &outputLock, out: options.Stderr}
	}
	capacity := profile.Select(options.Root).MaxParallel
	started := time.Now()
	ran := map[string]bool{}
	running := map[string]backlog.Group{}
	finished := make(chan laneResult, capacity)
	var shippedGroups, parked, holding []string
	code := 0
	stopping := false
	var drainErr error
	for {
		if !stopping {
			// Every open group drains first, oldest start first, then each ready group in file order.
			groups, err := drainGroups(options.Root)
			if err != nil {
				// Every lane already running still drains below, so its result still reaches shipped or parked.
				drainErr = err
				code = 1
				stopping = true
			} else {
				// A group this drain already shipped or parked is skipped, so one stuck group never holds the rest.
				var pending, busy []backlog.Group
				for _, group := range groups {
					if !ran[group.ID] {
						pending = append(pending, group)
					}
				}
				for _, group := range running {
					busy = append(busy, group)
				}
				// A group that waits on a parked or held group is held for the rest of the drain.
				pending, held := conductor.Hold(pending, append(slices.Clone(parked), holding...))
				for _, each := range held {
					ran[each.Group.ID] = true
					holding = append(holding, each.Group.ID)
					fmt.Fprintf(stdout, "%s held: it depends on %s, which parked\n", each.Group.ID, each.On)
				}
				startable := conductor.Startable(pending, busy, capacity)
				if len(startable) > 0 && !awaitWindow(options.Root, stdout, started.Add(total)) {
					code = 124
					stopping = true
					startable = nil
				}
				for _, group := range startable {
					ran[group.ID] = true
					remaining := total - time.Since(started)
					if remaining <= 0 {
						fmt.Fprintf(stdout, "%s stopped: the whole %s budget is spent\n", group.ID, total)
						code = 124
						stopping = true
						break
					}
					lane := options
					lane.Target = group.ID
					lane.Budget = min(GroupBudget, remaining)
					lane.Executable = executable
					running[group.ID] = group
					go runLane(lane, finished)
				}
				if len(running) == 0 && !stopping {
					fmt.Fprintf(stdout, "drain done: nothing is ready; %d shipped, %d parked", len(shippedGroups), len(parked))
					if len(parked) > 0 {
						fmt.Fprintf(stdout, " (%s)\n", strings.Join(parked, ", "))
						code = 1
					} else {
						fmt.Fprintln(stdout)
					}
					break
				}
			}
		}
		if len(running) == 0 {
			break
		}
		result := <-finished
		delete(running, result.group)
		if result.err != nil {
			fmt.Fprintf(stdout, "%s parked: %v\n", result.group, result.err)
			parked = append(parked, result.group)
			continue
		}
		if !shipped(options.Root, result.group, result.launched) {
			fmt.Fprintf(stdout, "%s parked: it ended without shipping (exit %d)\n", result.group, result.code)
			parked = append(parked, result.group)
			continue
		}
		url := result.url
		if url == "" {
			url = "no pull request was handed off"
		}
		fmt.Fprintf(stdout, "%s shipped: %s\n", result.group, url)
		shippedGroups = append(shippedGroups, result.group)
		restack(options, stdout)
	}
	// Sync once more after the run ends, so a binary gone stale mid-run rebuilds only once every group is done.
	_, syncErr := Sync(SyncOptions{Root: options.Root, Stdout: stdout})
	if drainErr != nil {
		return code, drainErr
	}
	if syncErr != nil {
		return 1, syncErr
	}
	return code, nil
}

// restack moves each group stacked on a parent that has since merged onto its new base, printing each move;
// a failure is printed and leaves the drain running.
func restack(options Options, stdout io.Writer) {
	client := options.PR
	if client == nil {
		client = pr.New(options.Root)
	}
	moved, err := conductor.Restack(options.Root, client)
	for _, each := range moved {
		fmt.Fprintf(stdout, "restacked %s\n", each)
	}
	if err != nil {
		fmt.Fprintf(stdout, "restack: %v\n", err)
	}
}

// laneResult is how one group's lane ended: its exit code, the pull request it opened, and when it launched.
type laneResult struct {
	group    string
	code     int
	url      string
	err      error
	launched time.Time
}

// runLane drives one group through the conductor under its own budget, so a runaway kills only its tree.
func runLane(options Options, finished chan<- laneResult) {
	launched := time.Now()
	code, err := Drive(options)
	url := ""
	if err == nil {
		url = laneURL(options)
	}
	finished <- laneResult{group: options.Target, code: code, url: url, err: err, launched: launched}
}

// laneURL is the pull request Drive shipped a group to, once its run's branch is known; a client
// that cannot look it up leaves the drain to print its own group's message with no URL.
func laneURL(options Options) string {
	state, err := line.LoadRunFor(options.Root, options.Target)
	if err != nil || state.Branch == "" {
		return ""
	}
	client := options.PR
	if client == nil {
		client = pr.New(options.Root)
	}
	pull, err := client.View(state.Branch)
	if err != nil {
		return ""
	}
	return pull.URL
}

// lockedWriter serialises writes from every lane onto one writer.
type lockedWriter struct {
	lock *sync.Mutex
	out  io.Writer
}

// Write writes p while holding the shared lock.
func (w lockedWriter) Write(p []byte) (int, error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	return w.out.Write(p)
}

// drainGroups is the groups a drain plans, in order, with their tasks; a group without parsed tasks claims no file.
func drainGroups(root string) ([]backlog.Group, error) {
	order, err := drainOrder(root)
	if err != nil {
		return nil, err
	}
	var parsed backlog.Backlog
	if loaded, err := backlog.LoadRoot(root); err == nil {
		parsed = loaded
	}
	groups := make([]backlog.Group, 0, len(order))
	for _, id := range order {
		group := backlog.Group{ID: id}
		for _, candidate := range parsed.Groups {
			if candidate.ID == id {
				group = candidate
				break
			}
		}
		groups = append(groups, group)
	}
	return groups, nil
}

// listDrain prints the groups a drain would run, in order, the open run first, and launches nothing.
func listDrain(root string, stdout io.Writer) error {
	order, err := drainOrder(root)
	if err != nil {
		return err
	}
	if len(order) == 0 {
		fmt.Fprintln(stdout, "drain would run nothing: nothing is ready")
	}
	for index, group := range order {
		fmt.Fprintf(stdout, "%d. %s\n", index+1, group)
	}
	return nil
}

// drainBudget is the whole drain's budget: the one given, else GroupBudget for every group the drain plans.
func drainBudget(root string, given time.Duration) (time.Duration, error) {
	if given > 0 {
		return given, nil
	}
	order, err := drainOrder(root)
	if err != nil {
		return 0, err
	}
	return GroupBudget * time.Duration(max(len(order), 1)), nil
}

// drainOrder is the groups a drain plans to run, in order: every open run first, then each ready group.
func drainOrder(root string) ([]string, error) {
	var order []string
	for _, state := range line.OpenRuns(root) {
		order = append(order, state.Group)
	}
	groups, err := line.ReadyGroups(root)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		if !contains(order, group.ID) {
			order = append(order, group.ID)
		}
	}
	return order, nil
}

// shipped reports whether the ledger records a finished ship for the group at or after since.
func shipped(root, group string, since time.Time) bool {
	entries, err := line.Book(root).All()
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Station == "ship" && entry.Group == group && entry.Outcome == "done" && !entry.At.Before(since) {
			return true
		}
	}
	return false
}

// symlink links a path to a target; a test swaps it to act like Windows without Developer Mode.
var symlink = os.Symlink

// binPathLock keeps two lanes from replacing the shared komodo link at once.
var binPathLock sync.Mutex

// pinDrainPath puts the drain's own executable first on the process's PATH before any lane starts,
// so every session Drive spawns runs it, never one rebuilt mid-drain; "" falls back to the running binary.
func pinDrainPath(root, executable string) error {
	if executable == "" {
		var err error
		executable, err = mount.Executable()
		if err != nil {
			return err
		}
	}
	binPathLock.Lock()
	defer binPathLock.Unlock()
	env, err := withBinPath(os.Environ(), root, executable)
	if err != nil {
		return err
	}
	for _, entry := range env {
		if key, value, ok := strings.Cut(entry, "="); ok && key == "PATH" {
			return os.Setenv("PATH", value)
		}
	}
	return nil
}

// withBinPath puts komodo on the run's PATH as root/.komodo/bin, then the inherited PATH, then the repo's own root/bin.
func withBinPath(env []string, root, executable string) ([]string, error) {
	link := filepath.Join(root, line.StateDir, "bin")
	name := "komodo"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.MkdirAll(link, 0o755); err != nil {
		return nil, err
	}
	target := filepath.Join(link, name)
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	first := link
	if err := symlink(executable, target); err != nil {
		if err := copyExecutable(executable, target); err != nil {
			first = filepath.Dir(executable)
		}
	}
	sep := string(os.PathListSeparator)
	out := make([]string, 0, len(env)+1)
	found := false
	for _, entry := range env {
		if key, value, ok := strings.Cut(entry, "="); ok && key == "PATH" {
			entry = "PATH=" + first + sep + value + sep + filepath.Join(root, "bin")
			found = true
		}
		out = append(out, entry)
	}
	if !found {
		out = append(out, "PATH="+first+sep+filepath.Join(root, "bin"))
	}
	return out, nil
}

// copyExecutable copies the running binary to target, removing a partial copy when it fails.
func copyExecutable(executable, target string) error {
	data, err := os.ReadFile(executable)
	if err != nil {
		return err
	}
	if err := os.WriteFile(target, data, 0o755); err != nil {
		_ = os.Remove(target)
		return err
	}
	return nil
}

// contains reports whether the slice already holds the value.
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
