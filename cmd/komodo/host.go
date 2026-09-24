package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/detect"
	"komodo/internal/doctor"
	"komodo/internal/git"
	"komodo/internal/guard"
	"komodo/internal/install"
	"komodo/internal/line"
	"komodo/internal/mount"
)

// runGuard is the hook on stdin, or the table the gate runs.
func runGuard(root string, args []string) {
	if len(args) > 0 && args[0] == "check" {
		if !guard.Report(root, guard.Load(root, root), os.Stdout) {
			exit(1)
		}
		return
	}
	exit(guard.Hook(root, os.Stdin, os.Stdout, os.Stderr))
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
			chosen = chosen[:0]
			for _, host := range mount.Hosts() {
				if host.Render != nil {
					chosen = append(chosen, host)
				}
			}
			break
		}
		found, ok := mount.Get(name)
		if !ok {
			fail(fmt.Errorf("unknown host %q; mounted: %s", name, strings.Join(mount.Names(), ", ")))
		}
		if found.Render == nil {
			fail(fmt.Errorf("host %q has nothing to install; the binary itself is its mount", name))
		}
		chosen = append(chosen, found)
	}
	if !*dryRun {
		hook := binary
		if !filepath.IsAbs(hook) {
			hook = filepath.Join(mount.MainCheckout(root), hook)
		}
		if _, err := os.Stat(hook); err != nil {
			fail(fmt.Errorf("the guard hook would run %s, which does not exist; build it with komodo gate --install", hook))
		}
	}
	var plans []install.Plan
	for _, host := range chosen {
		plan, err := host.Render(root, binary)
		if err != nil {
			fail(err)
		}
		plans = append(plans, plan)
	}
	if ignore := repoIgnores(root, plans); len(ignore.Changes) > 0 {
		applyPlan(ignore, *dryRun)
	}
	for _, plan := range plans {
		applyPlan(plan, *dryRun)
	}
}

// repoIgnores plans the .gitignore lines for the state dir and each rendered project copy git does not already ignore.
func repoIgnores(root string, plans []install.Plan) install.Plan {
	ignore := install.Plan{Host: "repo", Root: root}
	ignore.AddIgnore("/"+line.StateDir+"/", "the line's run state and worktrees stay out of git")
	for _, plan := range plans {
		for _, change := range plan.Project().Changes {
			if change.Remove {
				continue
			}
			rel, err := filepath.Rel(root, change.Path)
			if err != nil || strings.HasPrefix(rel, "..") {
				continue
			}
			rel = filepath.ToSlash(rel)
			// A path an existing rule already covers needs no line of its own.
			if _, err := git.Run(root, "check-ignore", "-q", "--no-index", "--", rel); err == nil {
				continue
			}
			ignore.AddIgnore("/"+rel, "a rendered copy the install rebuilds on every machine")
		}
	}
	return ignore
}

// applyPlan prints a plan under --dry-run, or writes it and lists what it changed.
func applyPlan(plan install.Plan, dryRun bool) {
	if dryRun {
		plan.Print(os.Stdout)
		return
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
	prune := set.Bool("prune", false, "remove stale worktrees, delete merged branches, and settle a run origin has merged")
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
		for _, note := range doctor.HostLeftovers(root) {
			fmt.Println("note " + note)
		}
		for _, note := range doctor.StrayWorktrees(root) {
			fmt.Println("note " + note)
		}
		fmt.Printf("%d problem(s)\n", len(problems))
	}
	if len(problems) > 0 {
		exit(1)
	}
}
