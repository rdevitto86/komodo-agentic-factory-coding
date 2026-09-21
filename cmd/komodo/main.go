// Command komodo is the conveyor and the devices of the code assembly line.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/gate"
	"komodo/internal/line"
)

const usage = `komodo: the code assembly line.

  komodo lint                 Check BACKLOG.md against the grammar
  komodo list [--json]        List every task, or one group's tasks
  komodo add <group> <title>  Append a task to a group
  komodo next [--json]        The next ready group: tasks, waves, machines
  komodo brief <task>         Fill the role template and write the brief
  komodo gate [--install]     The local precheck: vet, test, binaries
`

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
	problems := backlog.Lint(parsed)
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
	_ = set.Parse(args)
	_, parsed := load(root)
	groups := parsed.Groups
	if needle := set.Arg(0); needle != "" {
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
	_ = set.Parse(args)
	if set.NArg() < 2 {
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
	title := strings.Join(set.Args()[1:], " ")
	out, id, err := backlog.AppendTask(string(data), set.Arg(0), title, fields, *priority, *status)
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		fail(err)
	}
	fmt.Println(id)
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

// runGate runs the local precheck, or installs it as a git hook.
func runGate(root string, args []string) {
	set := flag.NewFlagSet("gate", flag.ExitOnError)
	install := set.Bool("install", false, "write the pre-commit and pre-push hooks")
	rebuild := set.Bool("rebuild", false, "rebuild every binary and rewrite the manifest")
	_ = set.Parse(args)
	if *install {
		written, err := gate.Install(filepath.Join(root, ".git"))
		if err != nil {
			fail(err)
		}
		for _, path := range written {
			fmt.Println("wrote", path)
		}
		return
	}
	if *rebuild {
		if err := gate.WriteManifest(root, os.Stdout); err != nil {
			fail(err)
		}
		fmt.Println("wrote bin/" + gate.ManifestName)
		return
	}
	checks := []gate.Check{
		gate.Command("go vet", root, "go", "vet", "./..."),
		gate.Command("go test", root, "go", "test", "./..."),
		{Name: "komodo lint", Run: func(_ io.Writer) error {
			_, parsed := load(root)
			problems := backlog.Lint(parsed)
			for _, problem := range problems {
				fmt.Println(problem)
			}
			if len(problems) > 0 {
				return fmt.Errorf("%d problem(s) in the backlog", len(problems))
			}
			return nil
		}},
		gate.Binaries(root),
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
	_ = set.Parse(args)
	plan, err := line.Next(root, set.Arg(0))
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
		state, err := line.Start(root, plan, *base)
		if err != nil {
			fail(err)
		}
		plan.Base, plan.Branch, plan.Worktree = state.Base, state.Branch, state.Worktree
	} else if *base != "" {
		plan.Base = *base
	}
	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(plan); err != nil {
			fail(err)
		}
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

// runBrief fills a role's template for one task and writes it into the task worktree.
func runBrief(root string, args []string) {
	set := flag.NewFlagSet("brief", flag.ExitOnError)
	role := set.String("role", "builder", "the role the brief is for")
	dryRun := set.Bool("dry-run", false, "print slot sizes and a token estimate, write nothing")
	failure := set.String("failure", "", "the previous attempt's output, which makes this a repair")
	_ = set.Parse(args)
	if set.NArg() < 1 {
		fail(fmt.Errorf("usage: komodo brief <task> [--role builder] [--dry-run]"))
	}
	cwd := root
	if state, err := line.LoadRun(root); err == nil && state.Worktree != "" {
		cwd = state.Worktree
	}
	brief, err := line.BuildBrief(root, cwd, set.Arg(0), *role, *failure)
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
	if err := line.WriteBrief(root, brief, branch); err != nil {
		fail(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(brief); err != nil {
		fail(err)
	}
}
