package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"komodo/internal/ledger"
	"komodo/internal/line"
	"komodo/internal/pr"
	"komodo/internal/run"
)

// runNext prints the next ready group, and with --start cuts its branch in a worktree.
func runNext(root string, args []string) {
	set := flag.NewFlagSet("next", flag.ExitOnError)
	asJSON := set.Bool("json", false, "print JSON")
	start := set.Bool("start", false, "cut the group branch in its own worktree")
	base := set.String("base", "", "the branch to cut from, default the remote's default branch")
	force := set.Bool("force", false, "cut the group even while another run is open")
	needle, rest := splitPositional(args, "base")
	_ = set.Parse(rest)
	plan, err := line.PlanForStation(root, needle)
	if err != nil {
		fail(err)
	}
	if plan == nil {
		if *asJSON {
			fmt.Println("null")
		} else {
			fmt.Println("nothing is ready")
		}
		return
	}
	if *start {
		state, err := line.Start(root, plan, *base, *force)
		if err != nil {
			fail(err)
		}
		plan.Base, plan.Branch, plan.Worktree = state.Base, state.Branch, state.Worktree
	} else if *base != "" {
		plan.Base = *base
	}
	if *asJSON {
		printCompactJSON(os.Stdout, planForJSON(plan))
		return
	}
	fmt.Printf("%s %s\n", plan.Group, plan.Title)
	fmt.Printf("  branch %s from %s, %s v%s\n", plan.Branch, plan.Base, plan.Type, plan.Version)
	for index, wave := range plan.Waves {
		fmt.Printf("  wave %d: %s\n", index+1, strings.Join(wave, ", "))
	}
	if len(plan.Skipped) > 0 {
		fmt.Printf("  done already: %s\n", strings.Join(plan.Skipped, ", "))
	}
}

// planOutput is what next --json prints: tasks, waves, and machines, not the whole profile.
type planOutput struct {
	Group     string          `json:"group"`
	Title     string          `json:"title"`
	Type      string          `json:"type"`
	Version   string          `json:"version"`
	Mode      string          `json:"mode"`
	Base      string          `json:"base"`
	Branch    string          `json:"branch"`
	Worktree  string          `json:"worktree"`
	Tasks     []line.PlanTask `json:"tasks"`
	Waves     [][]string      `json:"waves"`
	Skipped   []string        `json:"skipped,omitempty"`
	Roles     []roleOutput    `json:"roles"`
	WaitUntil string          `json:"wait_until,omitempty"`
}

// roleOutput is one role's name, tier, and resolved machine, without its description.
type roleOutput struct {
	Name    string `json:"name"`
	Tier    string `json:"tier"`
	Machine string `json:"machine,omitempty"`
}

// planForJSON drops a plan's role descriptions and its whole profile, which no station reads.
func planForJSON(plan *line.Plan) planOutput {
	roles := make([]roleOutput, len(plan.Roles))
	for i, role := range plan.Roles {
		roles[i] = roleOutput{Name: role.Name, Tier: role.Tier, Machine: role.Machine}
	}
	return planOutput{
		Group: plan.Group, Title: plan.Title, Type: plan.Type, Version: plan.Version,
		Mode: plan.Mode, Base: plan.Base, Branch: plan.Branch, Worktree: plan.Worktree,
		Tasks: plan.Tasks, Waves: plan.Waves, Skipped: plan.Skipped,
		Roles: roles, WaitUntil: plan.WaitUntil,
	}
}

// runBrief fills a role's template for one task and writes it into the task worktree.
func runBrief(root string, args []string) {
	set := flag.NewFlagSet("brief", flag.ExitOnError)
	role := set.String("role", "builder", "the role the brief is for")
	dryRun := set.Bool("dry-run", false, "print slot sizes and a token estimate, write nothing")
	failure := set.String("failure", "", "the previous attempt's output, which makes this a repair")
	review := set.Bool("review", false, "write the fix brief for a group's blocking review findings")
	task, rest := splitPositional(args, "role", "failure")
	_ = set.Parse(rest)
	if task == "" {
		fail(fmt.Errorf("usage: komodo brief <task> [--role builder] [--dry-run] | --review <group>"))
	}
	if *review {
		brief, err := line.FixBrief(root, fixPlan(root, task))
		if err != nil {
			fail(err)
		}
		line.Stamp(root, ledger.Entry{
			Task: brief.Task, Station: "brief", Role: brief.Role, TokensIn: brief.Tokens, Outcome: "written",
		})
		printJSON(brief)
		return
	}
	// A repair reads the task's own worktree, where the failed attempt's edits still sit.
	cwd := line.TaskWorktree(root, task)
	previous := *failure
	if previous == "" {
		previous = line.RepairText(root, task)
	}
	brief, err := line.BuildBrief(root, cwd, task, *role, previous)
	if err != nil {
		fail(err)
	}
	if *dryRun {
		names := make([]string, 0, len(brief.Slots))
		for name := range brief.Slots {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Printf("%-14s %7d chars\n", name, brief.Slots[name])
		}
		fmt.Printf("%-14s %7d chars, about %d tokens\n", "brief", len(brief.Text), brief.Tokens)
		return
	}
	branch := brief.Task
	if state, err := line.RunFor(root, task); err == nil && state.Branch != "" {
		branch = state.Branch
	}
	if err := line.RefuseCollision(root, task); err != nil {
		fail(err)
	}
	if err := line.WriteBrief(root, brief, branch); err != nil {
		fail(err)
	}
	line.Stamp(root, ledger.Entry{Task: task, Station: "brief", Role: *role, TokensIn: brief.Tokens, Outcome: "written"})
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(brief); err != nil {
		fail(err)
	}
}

// runClose validates one task's result, reruns its checks, and flips its status.
func runClose(root string, args []string) {
	set := flag.NewFlagSet("close", flag.ExitOnError)
	withGate := set.Bool("gate", false, "run the local gate before the task commit")
	wave := set.Int("wave", 0, "QC one wave of the named group's run, or the open run, counting from 1")
	group := set.Bool("group", false, "ship the named group's run, or the open run: commit, push, pull request, changelog")
	fix := set.Bool("fix", false, "gate and commit a review fix round on the named group's branch")
	base := set.String("base", "", "the branch the pull request targets")
	task, rest := splitPositional(args, "wave", "base")
	_ = set.Parse(rest)
	if *wave > 0 {
		runWave(root, *wave, task)
		return
	}
	if *fix {
		if task == "" {
			fail(fmt.Errorf("usage: komodo close --fix <group>"))
		}
		outcome, err := line.CloseFix(root, fixPlan(root, task))
		if err != nil {
			fail(err)
		}
		printJSON(outcome)
		if code := closeExitCode(outcome.Status); code != 0 {
			exit(code)
		}
		return
	}
	if *group {
		runShip(root, *base, task)
		return
	}
	if task == "" {
		fail(fmt.Errorf("usage: komodo close <task> [--gate] | --wave N [group] | --group [group] | --fix <group>"))
	}
	outcome, err := line.CloseTask(root, task, *withGate)
	if err != nil {
		fail(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(outcome); err != nil {
		fail(err)
	}
	if outcome.Status == "IN_PROGRESS" {
		fmt.Fprintf(os.Stderr, "komodo: %s failed and is IN_PROGRESS for a repair; run komodo step\n", outcome.Task)
	}
	if code := closeExitCode(outcome.Status); code != 0 {
		exit(code)
	}
}

// closeExitCode is 0 for a closed task and one awaiting its repair, and 1 for a blocked one.
func closeExitCode(status string) int {
	if status == "DONE" || status == "IN_PROGRESS" {
		return 0
	}
	return 1
}

// fixPlan is the named group's open run plan with its recorded branch and worktree, refusing any other group.
func fixPlan(root, group string) *line.Plan {
	plan, err := line.PlanForGroup(root, group)
	if err != nil {
		fail(err)
	}
	state, runErr := line.LoadRunFor(root, group)
	if plan == nil || plan.Group != group || runErr != nil {
		fail(fmt.Errorf("%s is not an open run's group", group))
	}
	if state.Branch != "" {
		plan.Base, plan.Branch, plan.Worktree = state.Base, state.Branch, state.Worktree
	}
	return plan
}

// runPlan is the named group's plan, or the open run's with no group, with its recorded base, branch, and worktree.
func runPlan(root, group string) *line.Plan {
	plan, err := line.PlanForGroup(root, group)
	if err != nil {
		fail(err)
	}
	if plan == nil {
		fail(fmt.Errorf("no run is in progress"))
	}
	if state, err := line.LoadRunFor(root, plan.Group); err == nil && state.Branch != "" {
		plan.Base, plan.Branch, plan.Worktree = state.Base, state.Branch, state.Worktree
	}
	return plan
}

// runWave merges one wave into the group branch, then runs the compile and verify gates.
func runWave(root string, number int, group string) {
	plan := runPlan(root, group)
	result, err := line.CloseWave(root, plan, number-1)
	if err != nil {
		fail(err)
	}
	printJSON(result)
	if !result.OK {
		exit(1)
	}
}

// runShip commits, pushes, opens the pull request, and writes the changelog line.
func runShip(root, base, group string) {
	plan := runPlan(root, group)
	if base != "" {
		plan.Base = base
	}
	result, err := line.ShipGroup(root, plan, nil, pr.New(root))
	if err != nil {
		fail(err)
	}
	printJSON(result)
}

// currentPlan is the plan for the run in progress, with its recorded base and branch.
func currentPlan(root string) *line.Plan {
	plan, err := line.PlanForStation(root, "")
	if err != nil {
		fail(err)
	}
	if plan == nil {
		fail(fmt.Errorf("no group is ready and no run is in progress"))
	}
	if state, err := line.LoadRunFor(root, plan.Group); err == nil && state.Branch != "" {
		plan.Base, plan.Branch, plan.Worktree = state.Base, state.Branch, state.Worktree
	}
	if info, err := os.Stat(line.WorktreePath(root, plan.Worktree)); err != nil || !info.IsDir() {
		plan.Worktree = "."
	}
	return plan
}

// runDiff prints the whole input a reviewer reads.
func runDiff(root string) {
	input, err := line.DiffFor(root, currentPlan(root))
	if err != nil {
		fail(err)
	}
	fmt.Print(input.Text)
}

// runReport prints what the run did, for the group the run record names.
func runReport(root string) {
	plan, err := line.PlanForRun(root)
	if err != nil {
		fail(err)
	}
	if plan == nil {
		fail(fmt.Errorf("no run is in progress and nothing is ready"))
	}
	if state, err := line.LoadRunFor(root, plan.Group); err == nil && state.Branch != "" {
		plan.Base, plan.Branch, plan.Worktree = state.Base, state.Branch, state.Worktree
	}
	report, err := line.BuildReport(root, plan)
	if err != nil {
		fail(err)
	}
	fmt.Print(report.Text)
	if rounds := line.FixRounds(root, plan.Group); rounds > 0 {
		fmt.Printf("- **%d review fix round(s)** ran on %s.\n", rounds, plan.Group)
	}
}

// runStep prints the one next action of the run.
func runStep(root string, args []string) {
	needle := ""
	if len(args) > 0 {
		needle = args[0]
	}
	next, err := line.Step(root, needle)
	if err != nil {
		fail(err)
	}
	printCompactJSON(os.Stdout, next)
}

// runRun drives the line headless on the profile's host and exits with the host's code.
func runRun(root string, args []string) {
	flags := flag.NewFlagSet("run", flag.ExitOnError)
	dry := flags.Bool("dry-run", false, "print the command the host would be given and stop")
	budget := flags.Duration("budget", 0, "how long the run may take before it is killed (default: "+
		run.GroupBudget.String()+" per group)")
	target, rest := splitPositional(args, "budget")
	_ = flags.Parse(rest)
	// A targeted run locks its own group, so another group on disjoint files may run beside it.
	group := line.GroupFor(root, target)
	if !*dry {
		label := target
		if label == "" {
			label = "the open run"
		}
		if err := line.AcquireLock(root, group, label); err != nil {
			fail(err)
		}
		_ = os.Setenv(line.LockEnv, strconv.Itoa(os.Getpid()))
	}
	code, err := run.Launch(run.Options{Root: root, Target: target, Budget: *budget, DryRun: *dry})
	line.ReleaseLock(root, group)
	if err != nil {
		fail(err)
	}
	exit(code)
}

// runSync brings the root up to origin, rebuilds a stale binary, and re-renders drifted config.
func runSync(root string, args []string) {
	flags := flag.NewFlagSet("sync", flag.ExitOnError)
	dry := flags.Bool("dry-run", false, "print each step and write nothing")
	_ = flags.Parse(args)
	if err := run.Sync(run.SyncOptions{Root: root, DryRun: *dry, Stdout: os.Stdout}); err != nil {
		fail(err)
	}
}
