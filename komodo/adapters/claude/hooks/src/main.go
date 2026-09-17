// Command komodo-hooks carries every Claude Code hook as a subcommand, so one binary per machine replaces the Python scripts.
package main

import (
	"fmt"
	"os"
)

// main dispatches to a hook by subcommand; an unknown one is a usage error, not a silent success.
func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "guard":
			guardMain()
			return
		case "inject":
			injectMain()
			return
		}
	}
	fmt.Fprintln(os.Stderr, "usage: komodo-hooks guard|inject")
	os.Exit(2)
}
