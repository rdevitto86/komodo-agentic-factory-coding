// Command komodo is the conveyor and the devices of the code assembly line.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/backlog"
	_ "komodo/internal/mount/ollama"

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
  komodo recall [--model m]   Score the local reviewer against the seeded-bug corpus
  komodo version              The changelog version and commit this binary was built from
  komodo gate [--install]     The local precheck: vet, race tests, doctor, guard, comments; --fuzz 10s adds fuzzing
`

// version and commit name this build; gate and release builds stamp them through -ldflags.
var (
	version = "dev"
	commit  = "unknown"
)

// exit ends the process; tests swap it to catch an exit code without leaving.
var exit = os.Exit

// main dispatches one subcommand.
func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		exit(2)
	}
	switch os.Args[1] {
	case "-v", "--version", "version":
		fmt.Printf("komodo %s (%s)\n", version, commit)
		return
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
	case "recall":
		runRecall(root, os.Args[2:])
	case "gate":
		runGate(root, os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "komodo: unknown command %q\n\n%s", os.Args[1], usage)
		exit(2)
	}
}

// fail prints one line to stderr and exits non-zero.
func fail(err error) {
	fmt.Fprintf(os.Stderr, "komodo: %v\n", err)
	exit(1)
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
