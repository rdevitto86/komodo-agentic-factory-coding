package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/ledger"
	"komodo/internal/line"
)

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
		exit(1)
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
	if line.RunIsOpen(root) {
		// A run keeps live status in .komodo, not BACKLOG.md, until it ships.
		parsed = line.OverlayStatus(parsed, line.LoadStatus(root))
	}
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
