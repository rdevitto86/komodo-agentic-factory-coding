package gate

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strings"

	"komodo/internal/git"
)

// HookStep is one command a git hook runs: a rule check in the installed binary, or the gate itself.
type HookStep struct {
	// Args follow the binary's name, such as gate --check-branch.
	Args []string
	// Rules runs the check in the installed binary, so a branch older than a rule still meets it.
	Rules bool
}

// GateRunner is the command line the gate runs through from dir, and the directory it runs in: the
// toolkit's own checkout runs its committed source, so its guard table and comment rules are the ones gated.
func GateRunner(dir, self string) (argv []string, workdir string) {
	top, err := git.Run(dir, "rev-parse", "--show-toplevel")
	if err == nil && IsToolkit(top) {
		return []string{"go", "run", "./cmd/komodo"}, top
	}
	return []string{self}, dir
}

// HookSteps plans what git hook name runs from dir, given its arguments and stdin.
func HookSteps(dir, name string, args []string, stdin string) ([]HookStep, error) {
	switch name {
	case "commit-msg":
		if len(args) == 0 {
			return nil, fmt.Errorf("commit-msg: git passed no message file")
		}
		return []HookStep{{Args: []string{"gate", "--commit-msg", args[0]}, Rules: true}}, nil
	case "pre-commit":
		// Tests run on push, not on every commit.
		return []HookStep{{Args: []string{"gate", "--check-branch"}, Rules: true}, {Args: []string{"gate", "--commit"}}}, nil
	case "pre-push":
		return prePushSteps(stdin), nil
	case "post-merge":
		if !mainWorktree(dir) {
			return nil, nil
		}
		from, _ := git.Run(dir, "rev-parse", "--quiet", "--verify", "ORIG_HEAD")
		return rebuild(from, git.Or(dir, "rev-parse", "HEAD")), nil
	case "post-checkout":
		// Only a branch checkout, flag 1, rebuilds; a file checkout never does.
		if len(args) < 3 || args[2] != "1" || !mainWorktree(dir) {
			return nil, nil
		}
		return rebuild(args[0], args[1]), nil
	case "post-commit":
		parent, err := git.Run(dir, "rev-parse", "--quiet", "--verify", "HEAD^1")
		if err != nil || !mainWorktree(dir) {
			return nil, nil
		}
		return rebuild(parent, git.Or(dir, "rev-parse", "HEAD")), nil
	case "post-rewrite":
		from, to := rewrittenSpan(stdin)
		if from == "" || !mainWorktree(dir) {
			return nil, nil
		}
		return rebuild(from, to), nil
	}
	return []HookStep{{Args: []string{"gate"}}}, nil
}

// prePushSteps refuses each pushed ref by the rules, then gates the first ref's commit, scoped to its commits;
// a push that only deletes refs carries no commits, so it runs no gate.
func prePushSteps(stdin string) []HookStep {
	var steps []HookStep
	var localSHA, remoteSHA string
	deletesOnly := strings.TrimSpace(stdin) != ""
	for index, line := range strings.Split(strings.TrimSpace(stdin), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if index == 0 {
			localSHA, remoteSHA = fields[1], fields[3]
		}
		if fields[1] != zeroOID {
			deletesOnly = false
		}
		steps = append(steps, HookStep{Args: []string{"gate", "--check-push", fields[2]}, Rules: true})
	}
	if deletesOnly {
		return steps
	}
	gate := []string{"gate", "--fuzz", "10s"}
	if remoteSHA != "" {
		gate = append(gate, "--from", remoteSHA, "--to", localSHA, "--at", localSHA)
	}
	return append(steps, HookStep{Args: gate})
}

// rebuild is the one step a post hook runs: rebuilding the binary when from to to changed its inputs.
func rebuild(from, to string) []HookStep {
	return []HookStep{{Args: []string{"gate", "--rebuild", "--from", from, "--to", to}}}
}

// rewrittenSpan is the first old and last new commit of a post-rewrite hook's pairs.
func rewrittenSpan(stdin string) (from, to string) {
	scanner := bufio.NewScanner(strings.NewReader(stdin))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		if from == "" {
			from = fields[0]
		}
		to = fields[1]
	}
	return from, to
}

// mainWorktree reports whether dir is the main working tree; the line rebases and merges in worktrees it cut.
func mainWorktree(dir string) bool {
	gitDir, err := git.Run(dir, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return false
	}
	common, err := git.Run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	return err == nil && filepath.Clean(gitDir) == filepath.Clean(common)
}
