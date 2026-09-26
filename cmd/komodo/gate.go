package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"komodo/internal/backlog"
	"komodo/internal/comments"
	"komodo/internal/doctor"
	"komodo/internal/gate"
	"komodo/internal/guard"
	"komodo/internal/line"
)

// runGate runs the local precheck, or builds the local binary and installs it as a git hook.
func runGate(root string, args []string) {
	set := flag.NewFlagSet("gate", flag.ExitOnError)
	install := set.Bool("install", false, "build the local binary and write the pre-commit and pre-push hooks")
	fuzz := set.String("fuzz", "", "also fuzz each parser for this long, such as 10s")
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
	checks := append(buildChecks(root), []gate.Check{
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
	}...)
	if *fuzz != "" && toolkitCheckout(root) {
		checks = append(checks, gate.FuzzChecks(root, *fuzz)...)
	}
	if err := gate.Run(checks, os.Stdout); err != nil {
		fail(err)
	}
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
	for _, command := range append(line.CompileCommands(root, root), line.VerifyCommand(root, root)) {
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
