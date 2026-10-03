package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"komodo/internal/gate"
	"komodo/internal/mount"
)

// runGitHook runs one git hook's steps: each rule check in this binary, then the gate through its runner.
func runGitHook(root string, args []string) {
	if len(args) == 0 {
		fail(fmt.Errorf("usage: komodo git-hook <name> [args]"))
	}
	code, err := gitHook(root, args[0], args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fail(err)
	}
	exit(code)
}

// gitHook plans and runs hook name from dir, returning the first failing step's exit code, or 0.
func gitHook(dir, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	var input string
	// Only these hooks read stdin; the rest leave it to git.
	if name == "pre-push" || name == "post-rewrite" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return 0, err
		}
		input = string(data)
	}
	steps, err := gate.HookSteps(dir, name, args, input)
	if err != nil {
		return 0, err
	}
	self, err := mount.Executable()
	if err != nil {
		return 0, err
	}
	runner, workdir := gate.GateRunner(dir, self)
	for _, step := range steps {
		argv, at := append(append([]string{}, runner...), step.Args...), workdir
		if step.Rules {
			argv, at = append([]string{self}, step.Args...), dir
		}
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir, cmd.Stdout, cmd.Stderr = at, stdout, stderr
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return exitErr.ExitCode(), nil
			}
			return 0, fmt.Errorf("git-hook %s: %s: %w", name, argv[0], err)
		}
	}
	return 0, nil
}
