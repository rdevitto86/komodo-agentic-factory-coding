package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/conductor"
	"komodo/internal/harness"
	"komodo/internal/hooks"
	"komodo/internal/pr"
	"komodo/internal/preflight"
	"komodo/internal/run"
)

// runResume prints the state a killed run left for one group, so a resumed komodo run knows
// whether to continue its last session or start a fresh one from the group's WIP commit.
func runResume(root string, args []string) {
	set := flag.NewFlagSet("resume", flag.ExitOnError)
	asJSON := set.Bool("json", false, "print JSON")
	target, rest := splitPositional(args)
	_ = set.Parse(rest)
	group := harness.GroupFor(root, target)
	if group == "" {
		group = target
	}
	if group == "" {
		fail(fmt.Errorf("usage: komodo resume <group>"))
	}
	state, err := conductor.LoadState(conductor.StatePath(root, group))
	if err != nil {
		fail(fmt.Errorf("%s has no saved state to resume: %w", group, err))
	}
	if state.Current == conductor.Blocked && !state.Edited {
		if err := clearBlocker(state); err != nil {
			fail(err)
		}
		state.Edited = true
		if err := conductor.SaveState(conductor.StatePath(root, group), state); err != nil {
			fail(err)
		}
	}
	if *asJSON {
		printCompactJSON(os.Stdout, state)
		return
	}
	fmt.Printf("%s is at %s with %d session(s) and %d repair round(s) recorded\n",
		state.Group, state.Current, len(state.Sessions), state.Repairs)
}

// clearScreen moves the cursor home and clears the terminal, so status --watch redraws in place.
const clearScreen = "\033[H\033[2J"

// runStatus prints the current run: every group's state, the time it used, and what blocks it;
// --watch redraws it in place every interval until interrupted.
func runStatus(root string, args []string) {
	set := flag.NewFlagSet("status", flag.ExitOnError)
	asJSON := set.Bool("json", false, "print JSON")
	watch := set.Bool("watch", false, "redraw the status in place until interrupted")
	interval := set.Duration("interval", 2*time.Second, "how often --watch redraws")
	_ = set.Parse(args)
	show := func() {
		groups := hooks.RunStatus(root)
		if *asJSON {
			printCompactJSON(os.Stdout, groups)
			return
		}
		if text := hooks.StatusText(groups); text != "" {
			fmt.Print(text)
			return
		}
		fmt.Println("no run is recorded")
	}
	if !*watch {
		show()
		return
	}
	if *interval <= 0 {
		fail(fmt.Errorf("--interval must be positive, got %s", interval))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	watchStatus(ctx, *interval, show)
}

// watchStatus clears the screen and calls show every interval until ctx ends.
func watchStatus(ctx context.Context, interval time.Duration, show func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		fmt.Print(clearScreen)
		show()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// runAbandon removes a group's worktree and branch, and marks its open tasks BLOCKED under a note saying why.
func runAbandon(root string, args []string) {
	target, _ := splitPositional(args)
	group := harness.GroupFor(root, target)
	if group == "" {
		group = target
	}
	if group == "" {
		fail(fmt.Errorf("usage: komodo abandon <group>"))
	}
	if err := conductor.Abandon(root, group, time.Now()); err != nil {
		fail(err)
	}
	fmt.Printf("%s is abandoned: its worktree and branch are removed, and its open tasks are BLOCKED\n", group)
}

// clearBlocker removes a blocked group's note from its branch's backlog once a person set every task back
// from BLOCKED, so komodo run feeds the edited group to its resumed builder.
func clearBlocker(state conductor.State) error {
	path, text, found, err := backlog.FindGroupFile(state.Worktree, state.Group)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%s is not in %s", state.Group, backlog.GroupFilesDir)
	}
	group := backlog.ParseGroupFile(text)
	if group.Status == "BLOCKED" {
		return fmt.Errorf("%s is still BLOCKED in %s; edit the group and set it READY, then resume", state.Group, path)
	}
	out, _ := backlog.RemoveGroupFileNote(text)
	return os.WriteFile(path, []byte(out), 0o644)
}

// runFinishShip publishes a group a missing or expired credential stopped before Ship: push, draft PR, labels.
func runFinishShip(root string, args []string) {
	group, _ := splitPositional(args)
	if group == "" {
		fail(fmt.Errorf("usage: komodo ship <group>"))
	}
	if owner := harness.GroupFor(root, group); owner != "" {
		group = owner
	}
	result, err := harness.FinishShip(root, group, pr.New(root))
	if err != nil {
		fail(err)
	}
	printJSON(result)
}

// currentPlan is the plan for the run in progress, with its recorded base and branch.
func currentPlan(root string) *harness.Plan {
	plan, err := harness.PlanForStation(root, "")
	if err != nil {
		fail(err)
	}
	if plan == nil {
		fail(fmt.Errorf("no group is ready and no run is in progress"))
	}
	if state, err := harness.LoadRunFor(root, plan.Group); err == nil && state.Branch != "" {
		plan.Base, plan.Branch, plan.Worktree = state.Base, state.Branch, state.Worktree
	}
	if info, err := os.Stat(harness.WorktreePath(root, plan.Worktree)); err != nil || !info.IsDir() {
		plan.Worktree = "."
	}
	return plan
}

// runDiff prints the whole input a reviewer reads.
func runDiff(root string) {
	input, err := harness.DiffFor(root, currentPlan(root))
	if err != nil {
		fail(err)
	}
	fmt.Print(input.Text)
}

// runRun drives one group through the conductor to Shipped; --no-ship stops it ready to ship instead,
// and a drain or a dry run skip the preflight and the lock.
func runRun(root string, args []string) {
	flags := flag.NewFlagSet("run", flag.ExitOnError)
	dry := flags.Bool("dry-run", false, "print the command the host would be given and stop")
	noShip := flags.Bool("no-ship", false,
		"stop each group at shipped-ready, skipping the forge credential check and the push")
	budget := flags.Duration("budget", 0, "how long the run may take before it is killed (default: "+
		run.GroupBudget.String()+" per group)")
	target, rest := splitPositional(args, "budget")
	_ = flags.Parse(rest)
	// A headless session the run itself started must never call komodo run again.
	if os.Getenv(harness.LockEnv) != "" {
		fail(fmt.Errorf("komodo run is already driving this run; a session it started must not call it again"))
	}
	if !*dry {
		if err := runPreflight(root, *noShip); err != nil {
			fail(err)
		}
	}
	// A targeted run locks its own group, so another group on disjoint files may run beside it.
	group := harness.GroupFor(root, target)
	if !*dry {
		label := target
		if label == "" {
			label = "the open run"
		}
		if err := harness.AcquireLock(root, group, label); err != nil {
			fail(err)
		}
		_ = os.Setenv(harness.LockEnv, strconv.Itoa(os.Getpid()))
	}
	options := run.Options{Root: root, Target: target, Budget: *budget, DryRun: *dry, NoShip: *noShip}
	var code int
	var err error
	if *dry || *noShip || target == "" {
		code, err = run.Launch(options)
	} else {
		code, err = run.Drive(options)
	}
	harness.ReleaseLock(root, group)
	if err != nil {
		fail(err)
	}
	exit(code)
}

// runPreflight runs every preflight check and joins each failure with the fix it names.
func runPreflight(root string, noShip bool) error {
	failures, err := preflight.Run(root, preflight.Options{NoShip: noShip})
	if err != nil {
		return err
	}
	if len(failures) == 0 {
		return nil
	}
	lines := make([]string, 0, len(failures))
	for _, failure := range failures {
		lines = append(lines, fmt.Sprintf("%s: %s", failure.Name, failure.Fix))
	}
	return fmt.Errorf("preflight failed:\n%s", strings.Join(lines, "\n"))
}

// runSync brings the root up to origin, rebuilds a stale binary, and re-renders drifted config.
func runSync(root string, args []string) {
	flags := flag.NewFlagSet("sync", flag.ExitOnError)
	dry := flags.Bool("dry-run", false, "print each step and write nothing")
	_ = flags.Parse(args)
	if _, err := run.Sync(run.SyncOptions{Root: root, DryRun: *dry, Stdout: os.Stdout}); err != nil {
		fail(err)
	}
}
