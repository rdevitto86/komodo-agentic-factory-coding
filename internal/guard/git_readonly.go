package guard

import "strings"

// switchTarget finds the ref a switch or checkout targets, and whether it creates a fresh one;
// a ref before -- and paths is a restore, and -B, -C, and --force-create reset, not create.
func switchTarget(rest []string) (target string, create bool, ok bool) {
	for index, arg := range rest {
		if arg == "--" {
			return "", false, false
		}
		if arg == "-b" || arg == "-c" || arg == "--create" || arg == "--orphan" {
			return switchOperand(rest, index+1, true)
		}
		if arg == "-B" || arg == "-C" || arg == "--force-create" {
			return switchOperand(rest, index+1, false)
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			if restoresPaths(rest[index+1:]) {
				return "", false, false
			}
			return arg, false, true
		}
	}
	return "", false, false
}

// switchOperand is the ref after a create flag, or nothing when the flag ends the command.
func switchOperand(rest []string, index int, create bool) (string, bool, bool) {
	if index < len(rest) {
		return rest[index], create, true
	}
	return "", false, false
}

// restoresPaths reports whether the tokens after a ref are -- and at least one path.
func restoresPaths(rest []string) bool {
	return len(rest) >= 2 && rest[0] == "--"
}

// previousBranch reports whether a switch target names an earlier branch, as - and @{-1} do.
func previousBranch(target string) bool {
	return target == "-" || strings.HasPrefix(target, "@{-")
}

// readOnlyGitCommands never move a ref, so running them in another checkout is harmless.
var readOnlyGitCommands = map[string]bool{
	"status": true, "log": true, "diff": true, "show": true, "rev-parse": true, "rev-list": true,
	"ls-files": true, "ls-tree": true, "cat-file": true, "blame": true, "grep": true,
	"describe": true, "shortlog": true, "merge-base": true, "name-rev": true,
}

// readOnlyGit reports whether a git subcommand only reads, including a branch or worktree listing.
func readOnlyGit(sub string, rest []string) bool {
	switch sub {
	case "branch":
		for _, arg := range rest {
			if !strings.HasPrefix(arg, "-") || !branchListFlags[arg] {
				return false
			}
		}
		return true
	case "worktree":
		return len(rest) > 0 && rest[0] == "list"
	case "config":
		return hasReadFlag(rest)
	case "remote":
		return readOnlyGitRemote(rest)
	}
	return readOnlyGitCommands[sub]
}

// remoteReadCommands are the git remote subcommands that only read.
var remoteReadCommands = map[string]bool{"show": true, "get-url": true}

// readOnlyGitRemote reports whether a git remote call only reads: bare, -v, show, or get-url.
func readOnlyGitRemote(rest []string) bool {
	for _, arg := range rest {
		if arg == "-v" || arg == "--verbose" {
			continue
		}
		return remoteReadCommands[arg]
	}
	return true
}

// branchListFlags are the git branch flags that only list.
var branchListFlags = map[string]bool{
	"--list": true, "-a": true, "--all": true, "-r": true, "--remotes": true, "-v": true, "-vv": true, "--show-current": true,
}

// hasReadFlag reports whether a git config call only reads, never writes.
func hasReadFlag(rest []string) bool {
	for _, arg := range rest {
		if configReadFlags[arg] {
			return true
		}
	}
	return false
}
