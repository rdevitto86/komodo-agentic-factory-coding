package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/detect"
	"komodo/internal/doctor"
	"komodo/internal/gate"
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

// runInstall renders this repo's host configuration, or with --global the user's, or prints what it would change.
func runInstall(root string, args []string) {
	set := flag.NewFlagSet("install", flag.ExitOnError)
	host := set.String("host", mount.Names()[0], "a mount name, several separated by commas, or both")
	dryRun := set.Bool("dry-run", false, "print what would change and write nothing")
	global := set.Bool("global", false, "install only the orchestrator layer in the user's host config, not this repo")
	_ = set.Parse(args)
	if root == "" {
		*global = true
	}
	binary := mount.BinaryPath()
	var chosen []mount.Host
	for _, name := range strings.Split(*host, ",") {
		name = strings.TrimSpace(name)
		if name == "both" || name == "all" {
			chosen = chosen[:0]
			for _, host := range mount.Active() {
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
		if found.Deferred != "" {
			fail(fmt.Errorf("host %q is not installable: %s", name, found.Deferred))
		}
		chosen = append(chosen, found)
	}
	hook := binary
	if !filepath.IsAbs(hook) {
		hook = filepath.Join(mount.MainCheckout(root), hook)
	}
	toolkit := root != "" && gate.IsToolkit(mount.MainCheckout(root))
	if !*dryRun {
		if err := install.CheckFilesystem(root); err != nil {
			fail(err)
		}
		// The toolkit's own checkout builds its binary when it has none, so one install sets up a fresh clone.
		if _, err := os.Stat(hook); err != nil && toolkit && !*global {
			if _, err := gate.BuildLocal(mount.MainCheckout(root), os.Stdout); err != nil {
				fail(err)
			}
		}
		if _, err := os.Stat(hook); err != nil {
			fail(fmt.Errorf("the guard hook would run %s, which does not exist; build it with komodo gate --install", hook))
		}
		publishAndLink(hook)
		if toolkit && !*global {
			installGitHooks(root)
		}
	}
	if *global {
		installGlobal(root, hook, chosen, *dryRun)
		return
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
	installGlobal(root, hook, chosen, *dryRun)
}

// publishAndLink puts binary at the one path every hook runs, then links it onto PATH for a person.
func publishAndLink(binary string) {
	published := mount.Publish(binary)
	fmt.Println("published", published)
	home, err := os.UserHomeDir()
	if err != nil {
		fail(err)
	}
	hint, err := install.LinkOnPath(published, install.PathDir(home), os.Getenv("PATH"))
	if err != nil {
		fail(err)
	}
	if hint != "" {
		fmt.Println(hint)
	}
}

// installGitHooks writes the git hooks, each a trampoline into the published binary, into the shared git dir.
func installGitHooks(root string) {
	common, err := git.Run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		fail(err)
	}
	written, err := gate.Install(common)
	if err != nil {
		fail(err)
	}
	fmt.Printf("git hooks: %d written\n", len(written))
}

// installGlobal renders each chosen host's orchestrator layer into the user's home, or prints what it would change.
func installGlobal(root, binary string, chosen []mount.Host, dryRun bool) {
	for _, host := range chosen {
		plan, err := install.GlobalPlan(host.Name, root, binary)
		if err != nil {
			fail(err)
		}
		applyPlan(plan, dryRun)
	}
}

// repoIgnores plans the .gitignore lines for the state dir, each rendered project copy, and each
// seeded personal overlay, whichever git does not already ignore.
func repoIgnores(root string, plans []install.Plan) install.Plan {
	ignore := install.Plan{Host: "repo", Root: root}
	ignore.AddIgnore("/"+line.StateDir+"/", "the line's run state and worktrees stay out of git")
	for _, plan := range plans {
		for _, change := range plan.Changes {
			if change.Remove || !(change.Project || change.Seed) {
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
			why := "a rendered copy the install rebuilds on every machine"
			if change.Seed {
				why = "a personal overlay the install seeds once and never overwrites"
			}
			ignore.AddIgnore("/"+rel, why)
		}
	}
	return ignore
}

// refreshMachine publishes the newest komodo to every hook, re-renders this repo's layer, and re-renders the
// user's global layer when one was installed before; it never installs a global layer unasked.
func refreshMachine(root string) ([]string, error) {
	latest := mount.LatestBinary(root)
	done := []string{"binary " + mount.Publish(latest)}
	var rendered strings.Builder
	if err := rerenderHosts(root, &rendered); err != nil {
		return done, err
	}
	for _, line := range strings.Split(strings.TrimSpace(rendered.String()), "\n") {
		if line != "" {
			done = append(done, line)
		}
	}
	for _, host := range mount.Active() {
		if _, ok := install.Global(host.Name); !ok || host.Deferred != "" {
			continue
		}
		plan, err := install.GlobalPlan(host.Name, root, latest)
		if err != nil {
			return done, err
		}
		if !plan.Installed() {
			continue
		}
		applied, err := plan.Apply()
		if err != nil {
			return done, err
		}
		for _, action := range applied {
			done = append(done, fmt.Sprintf("%-7s %s", action.Verb, filepath.Join(plan.Root, action.Path)))
		}
	}
	return done, nil
}

// rerenderHosts re-renders each host this main checkout already mounts, so a merged rule or skill leaves no drift.
func rerenderHosts(root string, out io.Writer) error {
	if mount.MainCheckout(root) != root {
		return nil
	}
	for _, host := range mount.Active() {
		if host.Render == nil || host.Deferred != "" {
			continue
		}
		plan, err := host.Render(root, mount.BinaryPath())
		if err != nil {
			return err
		}
		if !planMounted(plan) {
			continue
		}
		done, err := plan.Apply()
		if err != nil {
			return err
		}
		for _, action := range done {
			fmt.Fprintf(out, "%-7s %s\n", action.Verb, action.Path)
		}
	}
	return nil
}

// planMounted reports whether a host's plan already has a rendered file on disk, so an unmounted host stays so.
func planMounted(plan install.Plan) bool {
	for _, change := range plan.Changes {
		if change.Remove || change.Seed {
			continue
		}
		if _, err := os.Stat(change.Path); err == nil {
			return true
		}
	}
	return false
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
	prune := set.Bool("prune", false, "list stale worktrees, merged branches, and a settled run; add --confirm to delete")
	confirm := set.Bool("confirm", false, "with --prune, delete the branches it would otherwise only list")
	remote := set.Bool("remote", false, "also audit the forge's branch rulesets through gh")
	asJSON := set.Bool("json", false, "print JSON")
	_ = set.Parse(args)
	if *prune {
		base := line.DefaultBase(root)
		if state, err := line.LoadRun(root); err == nil && state.Base != "" {
			base = state.Base
		}
		done, err := doctor.Prune(root, base, *confirm)
		if err != nil {
			fail(err)
		}
		for _, item := range done {
			fmt.Println(item)
		}
	}
	options := doctor.Options{NoGit: *noGit, Remote: *remote}
	if !*asJSON {
		options.Warn = func(note string) { fmt.Println("warning " + note) }
	}
	problems, err := doctor.Run(root, options)
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
		for _, note := range doctor.PluginStates(root) {
			fmt.Println("note " + note)
		}
		fmt.Printf("%d problem(s)\n", len(problems))
	}
	if len(problems) > 0 {
		exit(1)
	}
}
