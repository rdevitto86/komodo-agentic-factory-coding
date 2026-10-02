package main

import (
	"fmt"
	"os"

	"komodo/internal/hooks"
)

// runHook is each hook's entry point: the guard runs as komodo guard, every other hook through the table.
func runHook(root string, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "komodo hook: name a hook; allowing")
		return
	}
	if args[0] == "guard" {
		runGuard(root, nil)
		return
	}
	if args[0] == "prune" && os.Getenv(hooks.SweepEnv) == "1" {
		hooks.Sweep(root)
		return
	}
	exit(hooks.Dispatch(root, args[0], args[1:], os.Stdin, os.Stdout, os.Stderr))
}
