// Command komodo is the conveyor and the devices of the code assembly line.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/comments"
	"komodo/internal/detect"
	"komodo/internal/doctor"
	"komodo/internal/gate"
	"komodo/internal/guard"
	"komodo/internal/ledger"
	"komodo/internal/line"
	"komodo/internal/mount"
	_ "komodo/internal/mount/ollama"
	"komodo/internal/pr"
	"komodo/internal/profile"
	"komodo/internal/release"
	"komodo/internal/run"
	"komodo/internal/toolkit"

	_ "komodo/internal/mount/claude"
	_ "komodo/internal/mount/codex"
)

const usage = `komodo: the code assembly line.

  komodo lint                 Check BACKLOG.md against the grammar
  komodo list [--json]        List every task, or one group's tasks
  komodo add <group> <title>  Append a task to a group
  komodo next [--json]        The next ready group: tasks, waves, machines
  komodo brief <task>         Fill the role template and write the brief
  komodo close <task>         Validate the result, rerun the checks, flip the status
  komodo close --wave N       QC: merge the wave, compile, verify
  komodo close --group        Ship: commit, push, the pull request, the changelog
  komodo comments check       The mechanical comment lint
  komodo diff                 The reviewer's whole input: tasks, standards, diff
  komodo report               What the run did, in the accessibility contract
  komodo tag                  Tag every changelog version no tag points at
  komodo release check        Audit the drift between changelog, tags, and groups
  komodo release build        Build the per-platform binaries as release assets
  komodo install --host X     Mount this repo on a host, or on both
  komodo detect [--json]      The cached repo profile: languages, cloud, data, CI, commands
  komodo doctor [--prune]     References, roles, leaks, drift, budgets, leftovers
  komodo guard [check]        The one agent hook; check runs its table
  komodo run [group|task]     Drive the line headless on this host, under a budget
  komodo step [group|task]    The one next action, as JSON
  komodo threads [pr]         The unresolved review threads, as JSON
  komodo threads --resolve id Mark one review thread resolved
  komodo machine <task>       Post a brief to the local machine, write the result, stamp the ledger
  komodo metrics              What the two ledger files hold
  komodo gate [--install]     The local precheck: vet, test, doctor, guard, comments
`

// main dispatches one subcommand.
func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	root, err := repoRoot()
	if err != nil {
		fail(err)
	}
	switch os.Args[1] {
	case "lint":
		runLint(root)
	case "list":
		runList(root, os.Args[2:])
	case "add":
		runAdd(root, os.Args[2:])
	case "next":
		runNext(root, os.Args[2:])
	case "brief":
		runBrief(root, os.Args[2:])
	case "close":
		runClose(root, os.Args[2:])
	case "comments":
		runComments(root, os.Args[2:])
	case "diff":
		runDiff(root)
	case "report":
		runReport(root)
	case "tag":
		runTag(root)
	case "release":
		runRelease(root, os.Args[2:])
	case "detect":
		runDetect(root, os.Args[2:])
	case "doctor":
		runDoctor(root, os.Args[2:])
	case "install":
		runInstall(root, os.Args[2:])
	case "guard":
		runGuard(root, os.Args[2:])
	case "run":
		runRun(root, os.Args[2:])
	case "step":
		runStep(root, os.Args[2:])
	case "threads":
		runThreads(root, os.Args[2:])
	case "machine":
		runMachine(root, os.Args[2:])
	case "metrics":
		runMetrics(root)
	case "gate":
		runGate(root, os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "komodo: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}

// fail prints one line to stderr and exits non-zero.
func fail(err error) {
	fmt.Fprintf(os.Stderr, "komodo: %v\n", err)
	os.Exit(1)
}

// repoRoot walks up from the working directory to the directory holding .git.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no git repository above %s", dir)
		}
		dir = parent
	}
}

// load reads and parses the repo's backlog.
func load(root string) (string, backlog.Backlog) {
	path, err := backlog.Find(root)
	if err != nil {
		fail(err)
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		fail(err)
	}
	return path, parsed
}

// runLint prints every grammar problem and exits non-zero when there is one.
func runLint(root string) {
	_, parsed := load(root)
	problems := append(backlog.Lint(parsed), backlog.LintContext(root, parsed)...)
	for _, problem := range problems {
		fmt.Println(problem)
	}
	fmt.Printf("%d task(s), %d group(s), %d problem(s)\n",
		len(parsed.Tasks()), len(parsed.Groups), len(problems))
	if len(problems) > 0 {
		os.Exit(1)
	}
}

// runList prints the tasks of one group, or of every group.
func runList(root string, args []string) {
	set := flag.NewFlagSet("list", flag.ExitOnError)
	asJSON := set.Bool("json", false, "print JSON")
	status := set.String("status", "", "only tasks with this status")
	needle, rest := splitPositional(args, "status")
	_ = set.Parse(rest)
	_, parsed := load(root)
	groups := parsed.Groups
	if needle != "" {
		group, ok := parsed.Group(needle)
		if !ok {
			fail(fmt.Errorf("no group matching %q", needle))
		}
		groups = []backlog.Group{group}
	}
	type row struct {
		ID       string `json:"id"`
		Group    string `json:"group"`
		Title    string `json:"title"`
		Priority string `json:"priority"`
		Status   string `json:"status"`
		Owner    string `json:"owner"`
	}
	var rows []row
	for _, group := range groups {
		for _, task := range group.Tasks {
			if *status != "" && !strings.EqualFold(task.Status, *status) {
				continue
			}
			rows = append(rows, row{task.ID, group.ID, task.Title, task.Priority, task.Status, task.Owner()})
		}
	}
	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(rows); err != nil {
			fail(err)
		}
		return
	}
	for _, item := range rows {
		fmt.Printf("%-16s %-12s %s %s\n", item.ID, "["+item.Status+"]", "[P: "+item.Priority+"]", item.Title)
	}
	fmt.Printf("%d task(s)\n", len(rows))
}

// runAdd appends one task to a group and prints its new id.
func runAdd(root string, args []string) {
	set := flag.NewFlagSet("add", flag.ExitOnError)
	files := set.String("files", "", "comma-separated paths the task touches")
	doneWhen := set.String("done-when", "", "comma-separated commands that prove it done")
	priority := set.String("priority", "M", "C, H, M, or L")
	status := set.String("status", "REFINEMENT", "the status to open the task in")
	taskType := set.String("type", "feat", "the conventional-commit type")
	positional, rest := splitFlags(args, "files", "done-when", "priority", "status", "type")
	_ = set.Parse(rest)
	if len(positional) < 2 {
		fail(fmt.Errorf("usage: komodo add <group> <title> [--files a,b] [--done-when cmd]"))
	}
	path, _ := load(root)
	data, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	var fields backlog.Fields
	fields.Set("files", split(*files))
	fields.Set("done_when", split(*doneWhen))
	fields.Set("type", *taskType)
	title := strings.Join(positional[1:], " ")
	out, id, err := backlog.AppendTask(string(data), positional[0], title, fields, *priority, *status)
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		fail(err)
	}
	fmt.Println(id)
	line.Stamp(root, ledger.Entry{Station: "add", Task: id, Outcome: "added"})
}

// split turns a comma-separated flag into the list a task block holds.
func split(value string) []any {
	items := []any{}
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

// runGate runs the local precheck, or builds the local binary and installs it as a git hook.
func runGate(root string, args []string) {
	set := flag.NewFlagSet("gate", flag.ExitOnError)
	install := set.Bool("install", false, "build the local binary and write the pre-commit and pre-push hooks")
	_ = set.Parse(args)
	if *install {
		path, err := gate.BuildLocal(root, os.Stdout)
		if err != nil {
			fail(err)
		}
		fmt.Println("built", path)
		written, err := gate.Install(filepath.Join(root, ".git"))
		if err != nil {
			fail(err)
		}
		for _, path := range written {
			fmt.Println("wrote", path)
		}
		return
	}
	checks := []gate.Check{
		gate.Command("go vet", root, "go", "vet", "./..."),
		gate.Command("go test", root, "go", "test", "./..."),
		{Name: "komodo lint", Run: func(_ io.Writer) error {
			_, parsed := load(root)
			problems := append(backlog.Lint(parsed), backlog.LintContext(root, parsed)...)
			for _, problem := range problems {
				fmt.Println(problem)
			}
			if len(problems) > 0 {
				return fmt.Errorf("%d problem(s) in the backlog", len(problems))
			}
			return nil
		}},
		{Name: "komodo doctor", Run: func(out io.Writer) error {
			problems, err := doctor.Run(root, doctor.Options{})
			if err != nil {
				return err
			}
			for _, problem := range problems {
				fmt.Fprintf(out, "%s %s: %s\n", problem.Check, problem.Where, problem.Detail)
			}
			if len(problems) > 0 {
				return fmt.Errorf("%d problem(s)", len(problems))
			}
			return nil
		}},
		{Name: "komodo guard check", Run: func(out io.Writer) error {
			if !guard.Report(root, guard.Load(root, root), out) {
				return fmt.Errorf("the guard table does not hold")
			}
			return nil
		}},
		{Name: "komodo comments check", Run: func(_ io.Writer) error {
			problems, err := comments.Check(root, trackedFiles(root), "nonobvious")
			if err != nil {
				return err
			}
			for _, problem := range problems {
				fmt.Println(problem)
			}
			if len(problems) > 0 {
				return fmt.Errorf("%d comment problem(s)", len(problems))
			}
			return nil
		}},
	}
	if err := gate.Run(checks, os.Stdout); err != nil {
		fail(err)
	}
}

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
		if err := line.CheckLock(root); err != nil {
			fail(err)
		}
		if !*force {
			if err := line.RefuseOpenRun(root, plan.Group); err != nil {
				fail(err)
			}
		}
		state, err := line.Start(root, plan, *base)
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
	task, rest := splitPositional(args, "role", "failure")
	_ = set.Parse(rest)
	if task == "" {
		fail(fmt.Errorf("usage: komodo brief <task> [--role builder] [--dry-run]"))
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
	if state, err := line.LoadRun(root); err == nil && state.Branch != "" {
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
	wave := set.Int("wave", 0, "QC one wave of the current run, counting from 1")
	group := set.Bool("group", false, "ship the current run: commit, push, pull request, changelog")
	base := set.String("base", "", "the branch the pull request targets")
	task, rest := splitPositional(args, "wave", "base")
	_ = set.Parse(rest)
	if *wave > 0 {
		runWave(root, *wave)
		return
	}
	if *group {
		runShip(root, *base)
		return
	}
	if task == "" {
		fail(fmt.Errorf("usage: komodo close <task> [--gate] | --wave N | --group"))
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
		os.Exit(code)
	}
}

// closeExitCode is 0 for a closed task and one awaiting its repair, and 1 for a blocked one.
func closeExitCode(status string) int {
	if status == "DONE" || status == "IN_PROGRESS" {
		return 0
	}
	return 1
}

// commentsArgs strips the check subcommand, if present, then splits what remains into paths and flags.
func commentsArgs(args []string, valueFlags ...string) (paths, rest []string) {
	if len(args) > 0 && args[0] == "check" {
		args = args[1:]
	}
	return splitFlags(args, valueFlags...)
}

// runComments lints the comments in the named files, or in every tracked source file.
func runComments(root string, args []string) {
	set := flag.NewFlagSet("comments", flag.ExitOnError)
	require := set.String("require", "nonobvious", "none, nonobvious, or exported")
	paths, rest := commentsArgs(args, "require")
	_ = set.Parse(rest)
	if len(paths) == 0 {
		paths = trackedFiles(root)
	} else if err := verifyPaths(root, paths); err != nil {
		fail(err)
	}
	problems, err := comments.Check(root, paths, *require)
	if err != nil {
		fail(err)
	}
	for _, problem := range problems {
		fmt.Println(problem)
	}
	fmt.Printf("%d file(s), %d problem(s)\n", len(paths), len(problems))
	if len(problems) > 0 {
		os.Exit(1)
	}
}

// trackedFiles lists what git tracks, which is what the lint walks by default.
func trackedFiles(root string) []string {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}

// runWave merges one wave into the group branch, then runs the compile and verify gates.
func runWave(root string, number int) {
	plan, err := line.PlanForStation(root, "")
	if err != nil {
		fail(err)
	}
	if plan == nil {
		fail(fmt.Errorf("no run is in progress"))
	}
	result, err := line.CloseWave(root, plan, number-1)
	if err != nil {
		fail(err)
	}
	printJSON(result)
	if !result.OK {
		os.Exit(1)
	}
}

// runShip commits, pushes, opens the pull request, and writes the changelog line.
func runShip(root, base string) {
	plan, err := line.PlanForStation(root, "")
	if err != nil {
		fail(err)
	}
	if plan == nil {
		fail(fmt.Errorf("no run is in progress"))
	}
	if state, err := line.LoadRun(root); err == nil {
		plan.Base, plan.Branch, plan.Worktree = state.Base, state.Branch, state.Worktree
	}
	if base != "" {
		plan.Base = base
	}
	result, err := line.ShipGroup(root, plan, nil, pr.New(root))
	if err != nil {
		fail(err)
	}
	printJSON(result)
}

// printJSON writes one value as indented JSON.
func printJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fail(err)
	}
}

// printCompactJSON writes one value as single-line JSON, for output a machine reads every loop.
func printCompactJSON(w io.Writer, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		fail(err)
	}
	fmt.Fprintln(w, string(data))
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
	if state, err := line.LoadRun(root); err == nil && state.Branch != "" {
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
	if state, err := line.LoadRun(root); err == nil && state.Branch != "" {
		plan.Base, plan.Branch, plan.Worktree = state.Base, state.Branch, state.Worktree
	}
	report, err := line.BuildReport(root, plan)
	if err != nil {
		fail(err)
	}
	fmt.Print(report.Text)
}

// runTag tags every changelog version no tag points at and pushes it.
func runTag(root string) {
	if err := tag(root, os.Stdout); err != nil {
		fail(err)
	}
}

// tag refuses to run off the branch origin's HEAD names, then tags and pushes every version origin lacks.
func tag(root string, out io.Writer) error {
	branch, err := gitRun(root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	if base := line.DefaultBase(root); branch != base {
		return fmt.Errorf("refusing to tag off %q, only the default branch %q tags", branch, base)
	}
	text, err := release.ReadChangelog(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return err
	}
	pending := release.Taggable(text, remoteTags(root))
	if len(pending) == 0 {
		fmt.Fprintln(out, "every changelog version is tagged")
		return nil
	}
	local := gitLines(root, "tag", "--list")
	for _, version := range pending {
		name := release.TagName(version)
		if !contains(local, name) {
			if _, err := gitRun(root, "tag", "-a", name, "-m", release.TagMessage(version)); err != nil {
				return err
			}
		}
		if _, err := gitRun(root, "push", "origin", name); err != nil {
			return err
		}
		fmt.Fprintln(out, "tagged", name)
	}
	return nil
}

// contains reports whether the slice holds the value.
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// remoteTags lists origin's tag names, so a local tag a failed push left behind is not mistaken for shipped.
func remoteTags(root string) []string {
	out, err := gitRun(root, "ls-remote", "--tags", "origin")
	if err != nil {
		return nil
	}
	var tags []string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(fields[1], "refs/tags/"), "^{}")
		if name != "" && !contains(tags, name) {
			tags = append(tags, name)
		}
	}
	return tags
}

// runRelease audits the drift between the changelog, the tags, and the groups, or builds release assets.
func runRelease(root string, args []string) {
	if len(args) == 0 || (args[0] != "check" && args[0] != "build") {
		fail(fmt.Errorf("usage: komodo release check | komodo release build"))
	}
	if args[0] == "build" {
		paths, err := release.BuildAssets(root, filepath.Join(root, "dist"), os.Stdout)
		if err != nil {
			fail(err)
		}
		for _, path := range paths {
			fmt.Println("wrote", path)
		}
		return
	}
	drift, err := checkRelease(root)
	if err != nil {
		fail(err)
	}
	for _, item := range drift {
		fmt.Printf("%s: %s\n", item.Subject, item.Detail)
	}
	fmt.Printf("%d drift(s)\n", len(drift))
	if len(drift) > 0 {
		os.Exit(1)
	}
}

// checkRelease audits the changelog against the tags and every shipped group's version.
func checkRelease(root string) ([]release.Drift, error) {
	text, err := release.ReadChangelog(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return nil, err
	}
	_, parsed := load(root)
	var versions []string
	for _, group := range parsed.Groups {
		if !shipped(group) {
			continue
		}
		versions = append(versions, group.Version())
	}
	return release.Check(text, gitLines(root, "tag", "--list"), versions), nil
}

// shipped reports whether every task in the group is DONE.
func shipped(group backlog.Group) bool {
	if len(group.Tasks) == 0 {
		return false
	}
	for _, task := range group.Tasks {
		if task.Status != "DONE" {
			return false
		}
	}
	return true
}

// gitRun runs one git command in the repo root.
func gitRun(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// gitLines runs one git command and splits its output into lines.
func gitLines(root string, args ...string) []string {
	out, err := gitRun(root, args...)
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// runMetrics prints what the two ledger files hold.
func runMetrics(root string) {
	entries, err := line.Book(root).All()
	if err != nil {
		fail(err)
	}
	fmt.Print(ledger.Render(ledger.Aggregate(entries)))
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
	budget := flags.Duration("budget", run.GroupBudget, "how long the run may take before it is killed")
	target, rest := splitPositional(args, "budget")
	_ = flags.Parse(rest)
	if !*dry {
		label := target
		if label == "" {
			label = "the open run"
		}
		if err := line.AcquireLock(root, label); err != nil {
			fail(err)
		}
		_ = os.Setenv(line.LockEnv, strconv.Itoa(os.Getpid()))
	}
	code, err := run.Launch(run.Options{Root: root, Target: target, Budget: *budget, DryRun: *dry})
	line.ReleaseLock(root)
	if err != nil {
		fail(err)
	}
	os.Exit(code)
}

// runThreads prints the unresolved review threads on a pull request, or resolves one by id.
func runThreads(root string, args []string) {
	set := flag.NewFlagSet("threads", flag.ExitOnError)
	resolve := set.String("resolve", "", "resolve the review thread with this id")
	number, rest := splitPositional(args, "resolve")
	_ = set.Parse(rest)
	client := pr.New(root)
	if *resolve != "" {
		if err := client.Resolve(*resolve); err != nil {
			fail(err)
		}
		fmt.Println("resolved", *resolve)
		return
	}
	threads, err := client.Threads(number)
	if err != nil {
		fail(err)
	}
	if threads == nil {
		threads = []pr.Thread{}
	}
	printJSON(threads)
}

// runMachine posts one task's brief to the local machine, writes the result, and stamps the ledger.
func runMachine(root string, args []string) {
	set := flag.NewFlagSet("machine", flag.ExitOnError)
	role := set.String("role", "builder", "the role the brief was written for")
	taskID, rest := splitTaskArg(args)
	if taskID == "" {
		fail(fmt.Errorf("usage: komodo machine <task> [--role reviewer]"))
	}
	_ = set.Parse(rest)
	definition, err := line.LoadRole(root, *role)
	if err != nil {
		fail(err)
	}
	local := mount.LocalMachine()
	if !local.Allowed(definition.Tools) {
		fail(fmt.Errorf("%s writes, and a write role cannot run on the local machine; step routes it to a remote one instead", *role))
	}
	brief, err := os.ReadFile(filepath.Join(root, line.StateDir, "briefs", taskID+".md"))
	if err != nil {
		fail(err)
	}
	schema, err := fs.ReadFile(toolkit.FS(root), path.Join("roles", definition.Returns))
	if err != nil {
		fail(err)
	}
	tier := definition.Tier
	if *role == "reviewer" {
		tier = "reviewer"
	}
	model, err := localModel(profile.Select(root).Tiers, tier)
	if err != nil {
		fail(err)
	}
	started := time.Now()
	result, err := local.Post(model, string(brief), schema)
	if err != nil {
		fail(err)
	}
	if err := os.MkdirAll(filepath.Dir(line.ResultPath(root, taskID)), 0o755); err != nil {
		fail(err)
	}
	data, err := json.MarshalIndent(result.Value, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(line.ResultPath(root, taskID), data, 0o644); err != nil {
		fail(err)
	}
	entry := ledger.Entry{
		Task: taskID, Station: "machine", Role: *role, Tier: definition.Tier,
		Provider: mount.LocalName, Model: model, Seconds: time.Since(started).Seconds(),
		TokensIn: result.TokensIn, TokensOut: result.TokensOut, Outcome: "done",
	}
	if state, err := line.LoadRun(root); err == nil {
		entry.Run, entry.Group = state.Run, state.Group
	}
	if err := line.Book(root).Stamp(entry); err != nil {
		fail(err)
	}
	fmt.Println("wrote", line.ResultPath(root, taskID))
}

// localModel resolves a role's tier to the model of whichever tier the local machine mounts;
// reviewer resolves through Tiers.Reviewer, which Tiers.Machine does not know.
func localModel(tiers mount.Tiers, tier string) (string, error) {
	machine := tiers.Machine(tier)
	if tier == "reviewer" {
		machine = tiers.Reviewer
	}
	if machine.Local() {
		return machine.Model, nil
	}
	for _, fallback := range []string{"light", "standard", "heavy"} {
		if candidate := tiers.Machine(fallback); candidate.Local() {
			return "", fmt.Errorf("%s tier does not mount the local machine; %s does, and komodo machine does not switch tiers", tier, fallback)
		}
	}
	return "", fmt.Errorf("no tier mounts the local machine")
}

// splitTaskArg pulls the task id out of a machine invocation's args, wherever it falls among the flags.
func splitTaskArg(args []string) (task string, rest []string) {
	return splitPositional(args, "role")
}

// takesValue reports whether an argument is one of the named flags in a form that consumes the next one.
func takesValue(arg string, names []string) bool {
	for _, name := range names {
		if arg == "--"+name || arg == "-"+name {
			return true
		}
	}
	return false
}

// splitPositional pulls the first positional out of args wherever it falls, keeping every flag.
func splitPositional(args []string, valueFlags ...string) (positional string, rest []string) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if takesValue(arg, valueFlags) {
			rest = append(rest, arg)
			if i+1 < len(args) {
				i++
				rest = append(rest, args[i])
			}
			continue
		}
		if positional == "" && !strings.HasPrefix(arg, "-") {
			positional = arg
			continue
		}
		rest = append(rest, arg)
	}
	return positional, rest
}

// splitFlags pulls every flag and its value out of args, keeping every other token as a positional, in order.
func splitFlags(args []string, valueFlags ...string) (positional, rest []string) {
	positional = make([]string, 0, len(args))
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if takesValue(arg, valueFlags) {
			rest = append(rest, arg)
			if i+1 < len(args) {
				i++
				rest = append(rest, args[i])
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			rest = append(rest, arg)
			continue
		}
		positional = append(positional, arg)
	}
	return positional, rest
}

// verifyPaths fails on the first path that does not exist under root, so a stray flag cannot pass silently.
func verifyPaths(root string, paths []string) error {
	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

// runGuard is the hook on stdin, or the table the gate runs.
func runGuard(root string, args []string) {
	if len(args) > 0 && args[0] == "check" {
		if !guard.Report(root, guard.Load(root, root), os.Stdout) {
			os.Exit(1)
		}
		return
	}
	os.Exit(guard.Hook(root, os.Stdin, os.Stdout, os.Stderr))
}

// runInstall renders this repo's host configuration, or prints what it would change.
func runInstall(root string, args []string) {
	set := flag.NewFlagSet("install", flag.ExitOnError)
	host := set.String("host", mount.Names()[0], "a mount name, several separated by commas, or both")
	dryRun := set.Bool("dry-run", false, "print what would change and write nothing")
	_ = set.Parse(args)
	binary := mount.BinaryPath()
	var chosen []mount.Host
	for _, name := range strings.Split(*host, ",") {
		name = strings.TrimSpace(name)
		if name == "both" || name == "all" {
			chosen = mount.Hosts()
			break
		}
		found, ok := mount.Get(name)
		if !ok {
			fail(fmt.Errorf("unknown host %q; mounted: %s", name, strings.Join(mount.Names(), ", ")))
		}
		chosen = append(chosen, found)
	}
	for _, host := range chosen {
		plan, err := host.Render(root, binary)
		if err != nil {
			fail(err)
		}
		if *dryRun {
			plan.Print(os.Stdout)
			continue
		}
		done, err := plan.Apply()
		if err != nil {
			fail(err)
		}
		for _, action := range done {
			fmt.Printf("%-7s %s\n", action.Verb, action.Path)
		}
		fmt.Printf("%s: %d file(s) changed\n", plan.Host, len(done))
	}
}

// runDetect prints the cached repo profile, detecting fresh when the manifests it read have changed.
func runDetect(root string, args []string) {
	set := flag.NewFlagSet("detect", flag.ExitOnError)
	asJSON := set.Bool("json", false, "print JSON")
	_ = set.Parse(args)
	found := detect.Load(root)
	if *asJSON {
		printJSON(found)
		return
	}
	fmt.Printf("languages: %s\n", listOrNone(found.Languages))
	fmt.Printf("cloud: %s\n", listOrNone(found.Cloud))
	fmt.Printf("data: %s\n", listOrNone(found.Data))
	fmt.Printf("ci: %s\n", listOrNone(found.CI))
	fmt.Printf("verify: %s\n", stringOrNone(found.Verify))
	fmt.Printf("compile: %s\n", stringOrNone(found.Compile))
}

// listOrNone joins a list for display, or names it empty.
func listOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}

// stringOrNone names an empty command as none.
func stringOrNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

// runDoctor audits the repo and, with --prune, clears what a run stranded.
func runDoctor(root string, args []string) {
	set := flag.NewFlagSet("doctor", flag.ExitOnError)
	noGit := set.Bool("no-git", false, "skip the checks that shell out to git")
	prune := set.Bool("prune", false, "remove stale worktrees and delete merged branches")
	remote := set.Bool("remote", false, "also audit the forge's branch rulesets through gh")
	asJSON := set.Bool("json", false, "print JSON")
	_ = set.Parse(args)
	if *prune {
		base := line.DefaultBase(root)
		if state, err := line.LoadRun(root); err == nil && state.Base != "" {
			base = state.Base
		}
		done, err := doctor.Prune(root, base)
		if err != nil {
			fail(err)
		}
		for _, item := range done {
			fmt.Println(item)
		}
	}
	problems, err := doctor.Run(root, doctor.Options{NoGit: *noGit, Remote: *remote})
	if err != nil {
		fail(err)
	}
	if *asJSON {
		printJSON(problems)
	} else {
		for _, problem := range problems {
			fmt.Printf("%s %s: %s\n", problem.Check, problem.Where, problem.Detail)
		}
		fmt.Printf("%d problem(s)\n", len(problems))
	}
	if len(problems) > 0 {
		os.Exit(1)
	}
}
