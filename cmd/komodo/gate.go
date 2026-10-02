package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/doctor"
	"komodo/internal/gate"
	"komodo/internal/git"
	"komodo/internal/guard"
	"komodo/internal/line"
	"komodo/internal/mount"
)

// runGate runs the local precheck, one of its git-hook checks, or builds and installs the binary.
func runGate(root string, args []string) {
	set := flag.NewFlagSet("gate", flag.ExitOnError)
	install := set.Bool("install", false, "build the local binary and write the pre-commit and pre-push hooks")
	fuzz := set.String("fuzz", "", "also fuzz each parser for this long, such as 10s")
	rebuild := set.Bool("rebuild", false, "rebuild the local binary when a Go file, go.mod or go.sum changed between --from and --to")
	from := set.String("from", "", "the commit before the change, for --rebuild")
	to := set.String("to", "", "the commit after the change, for --rebuild")
	commitMsg := set.String("commit-msg", "", "refuse an attribution trailer in this message file, for the commit-msg hook")
	checkBranch := set.Bool("check-branch", false, "refuse a critical ref or a branch outside <type>/<kebab-name>, for the pre-commit hook")
	checkPushRef := set.String("check-push-ref", "", "refuse a push to a critical ref or a branch outside <type>/<kebab-name>, for the pre-push hook")
	_ = set.Parse(args)
	if *commitMsg != "" {
		message, err := os.ReadFile(*commitMsg)
		if err != nil {
			fail(err)
		}
		if problem := gate.TrailerProblem(string(message), guard.Load(root, root)); problem != "" {
			fail(fmt.Errorf("%s", problem))
		}
		return
	}
	if *checkBranch {
		branch := git.TrackedBranch(root)
		if problem := gate.BranchProblem(branch, guard.Load(root, root)); problem != "" {
			fail(fmt.Errorf("%s", problem))
		}
		return
	}
	if *checkPushRef != "" {
		branch := strings.TrimPrefix(*checkPushRef, "refs/heads/")
		if problem := gate.BranchProblem(branch, guard.Load(root, root)); problem != "" {
			fail(fmt.Errorf("%s", problem))
		}
		return
	}
	if *rebuild {
		if err := gate.Rebuild(root, *from, *to, os.Stdout); err != nil {
			fail(err)
		}
		if err := rerenderHosts(root, os.Stdout); err != nil {
			fail(err)
		}
		return
	}
	if *install {
		path, err := gate.BuildLocalIfGoRepo(root, os.Stdout)
		if err != nil {
			fail(err)
		}
		if path != "" {
			fmt.Println("built", path)
		}
		written, err := gate.Install(filepath.Join(root, ".git"))
		if err != nil {
			fail(err)
		}
		for _, path := range written {
			fmt.Println("wrote", path)
		}
		return
	}
	scoped, err := gate.PushChecks(root, *from, *to, fuzzDuration(root, *fuzz), buildChecks(root))
	if err != nil {
		fail(err)
	}
	checks := append(scoped, []gate.Check{
		// Rendered host files are gitignored and derived, so the gate refreshes them before doctor judges drift.
		{Name: "komodo render", Run: func(out io.Writer) error { return rerenderHosts(root, out) }},
		{Name: "komodo lint", Run: func(_ io.Writer) error {
			problems, err := lintProblems(root)
			if err != nil {
				return err
			}
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
		gate.CommentsCheck(root, "nonobvious"),
	}...)
	if err := gate.Run(checks, os.Stdout); err != nil {
		fail(err)
	}
}

// fuzzDuration is the fuzz flag's value, but only in the toolkit's own checkout, whose fuzz targets exist.
func fuzzDuration(root, requested string) string {
	if requested != "" && toolkitCheckout(root) {
		return requested
	}
	return ""
}

// buildChecks are the toolkit's own vet and race tests in its checkout, else the compile and verify
// commands QC runs, so the gate fits any repo's language.
func buildChecks(root string) []gate.Check {
	if toolkitCheckout(root) {
		return []gate.Check{
			gate.Command("go vet", root, "go", "vet", "./..."),
			gate.Command("go test", root, gate.TestArgs()...),
		}
	}
	var checks []gate.Check
	seen := map[string]bool{}
	// A line worktree reads the main checkout's gitignored commands.json, as QC does.
	config := mount.MainCheckout(root)
	for _, command := range append(line.CompileCommands(config, root), line.VerifyCommand(config, root)) {
		if command == "" || seen[command] {
			continue
		}
		seen[command] = true
		checks = append(checks, gate.Check{Name: command, Run: func(out io.Writer) error {
			result := line.RunCommand(root, command)
			fmt.Fprintln(out, result.Output)
			if !result.OK() {
				return fmt.Errorf("exited %d", result.ExitCode)
			}
			return nil
		}})
	}
	if len(checks) == 0 {
		checks = append(checks, gate.Check{Name: "detect build checks", Run: func(_ io.Writer) error {
			return fmt.Errorf("no build checks found; add compile or verify to .komodo/commands.json")
		}})
	}
	return checks
}

// toolkitCheckout reports whether root is the toolkit's own source, whose Go checks and fuzz targets the gate runs.
func toolkitCheckout(root string) bool {
	_, err := os.Stat(filepath.Join(root, "cmd", "komodo", "main.go"))
	return err == nil
}
