package guard

import (
	"fmt"
	"strings"
)

// noVerifyCommands are the git subcommands whose --no-verify skips the gate's own hook.
var noVerifyCommands = map[string]bool{
	"commit": true, "merge": true, "push": true, "rebase": true, "am": true, "cherry-pick": true,
}

// gitFindings checks one git call against the guard's rules: a critical ref is never committed,
// pushed, merged, or moved by hand; pushed history is never rewritten; hooks are never skipped.
func gitFindings(words []string, branch string, policy Policy) []string {
	args := skipGlobalFlags(words[1:])
	if len(args) == 0 {
		return nil
	}
	sub, rest := args[0], args[1:]
	var findings []string
	if noVerifyCommands[sub] && normalizeMode(policy.Mode) != ModeUnsafe && hasNoVerify(sub, rest) {
		findings = append(findings, fmt.Sprintf("git %s --no-verify skips the gate; fix what it reports instead", sub))
	}
	switch sub {
	case "push":
		findings = append(findings, pushFindings(rest, branch, policy)...)
	case "commit":
		if policy.IsCritical(branch) && normalizeMode(policy.Mode) != ModeUnsafe {
			findings = append(findings, fmt.Sprintf("git commit on %s: create a branch first", branch))
		}
	case "merge":
		if (policy.IsCritical(branch) || lineEpicBranch(branch)) && normalizeMode(policy.Mode) != ModeUnsafe {
			findings = append(findings, fmt.Sprintf("git merge on %s: landing is the human's merge button", branch))
		}
	case "branch":
		findings = append(findings, branchFindings(rest, policy)...)
	}
	return findings
}

// skipGlobalFlags drops a leading run of git's own flags, so the subcommand after them is found;
// -c and -C each take the next word as their value.
func skipGlobalFlags(args []string) []string {
	index := 0
	for index < len(args) && strings.HasPrefix(args[index], "-") {
		flag := args[index]
		index++
		if flag == "-c" || flag == "-C" {
			index++
		}
	}
	if index > len(args) {
		return nil
	}
	return args[index:]
}

// hasNoVerify reports whether rest skips hooks: --no-verify, or commit's bare -n.
func hasNoVerify(sub string, rest []string) bool {
	for _, arg := range rest {
		if arg == "--no-verify" {
			return true
		}
		if sub == "commit" && arg == "-n" {
			return true
		}
	}
	return false
}

// isForceFlag reports whether a push option rewrites the remote's history.
func isForceFlag(arg string) bool {
	return arg == "-f" || arg == "--force" || arg == "--force-if-includes" || strings.HasPrefix(arg, "--force-with-lease")
}

// pushFindings refuses a push that rewrites history or lands on a critical ref.
func pushFindings(rest []string, branch string, policy Policy) []string {
	var findings []string
	var positional []string
	deletes := false
	for _, arg := range rest {
		if isForceFlag(arg) {
			findings = append(findings, fmt.Sprintf("git push %s: pushed history is never rewritten; push a new commit instead", arg))
		}
		switch {
		case arg == "--delete" || arg == "-d":
			deletes = true
		case arg != "" && !strings.HasPrefix(arg, "-"):
			positional = append(positional, strings.TrimPrefix(arg, "+"))
		}
	}
	if normalizeMode(policy.Mode) == ModeUnsafe {
		return findings
	}
	targets := []string{branch}
	if len(positional) > 1 {
		targets = positional[1:]
	}
	for _, target := range targets {
		if _, after, found := strings.Cut(target, ":"); found {
			target = after
		}
		if target == "HEAD" {
			target = branch
		}
		if policy.IsCritical(target) || lineEpicBranch(target) {
			verb := "push to"
			if deletes {
				verb = "delete"
			}
			findings = append(findings, fmt.Sprintf("git %s %s: open a pull request instead", verb, target))
		}
	}
	return findings
}

// branchFindings refuses a branch delete, force, or move that names a critical ref.
func branchFindings(rest []string, policy Policy) []string {
	if normalizeMode(policy.Mode) == ModeUnsafe {
		return nil
	}
	deleting, forcing, moving := false, false, false
	for _, arg := range rest {
		switch arg {
		case "-d", "-D", "--delete":
			deleting = true
		case "-f", "--force":
			forcing = true
		case "-m", "-M", "--move", "-c", "-C", "--copy":
			moving = true
		}
	}
	if !deleting && !forcing && !moving {
		return nil
	}
	var findings []string
	for _, arg := range rest {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if !policy.IsCritical(arg) {
			continue
		}
		verb := "moved"
		if deleting {
			verb = "deleted"
		}
		findings = append(findings, fmt.Sprintf("git branch %s: a critical ref is never %s by hand", arg, verb))
	}
	return findings
}

// lineEpicBranch reports whether a line session names an epic branch, which only the conductor pushes and merges.
func lineEpicBranch(branch string) bool {
	return IsLineSession() && IsEpicBranch(branch)
}

// branchFor is the branch one git call is judged on: the real branch of its -C target when the
// call points at another directory, or dir's tracked branch when it does not.
func branchFor(words []string, dir, branch string) string {
	if len(words) < 1 {
		return branch
	}
	if target := gitDirectory(dir, words[1:]); target != dir {
		return CurrentBranch(target)
	}
	return branch
}

// afterGit is the branch dir holds after one git call there: the branch a switch or checkout in
// it moved to, or branch unchanged, including when the call targeted another directory via -C.
func afterGit(words []string, dir, branch string) string {
	if len(words) < 1 || gitDirectory(dir, words[1:]) != dir {
		return branch
	}
	args := skipGlobalFlags(words[1:])
	if len(args) == 0 {
		return branch
	}
	switch args[0] {
	case "switch":
		return switchTarget(args[1:], branch)
	case "checkout":
		return checkoutTarget(args[1:], branch)
	}
	return branch
}

// switchTarget is the branch git switch moves to: the name after -c/-C/--create, or the first
// positional operand, or branch unchanged when neither names one.
func switchTarget(args []string, branch string) string {
	for index, arg := range args {
		if arg == "-c" || arg == "-C" || arg == "--create" || arg == "--force-create" {
			if index+1 < len(args) {
				return args[index+1]
			}
			return branch
		}
	}
	for _, arg := range args {
		if arg != "" && !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return branch
}

// checkoutTarget is the branch git checkout moves to: the name after -b/-B, or the first
// positional operand before a `--` pathspec separator, or branch unchanged when neither names one.
func checkoutTarget(args []string, branch string) string {
	for index, arg := range args {
		if arg == "-b" || arg == "-B" {
			if index+1 < len(args) {
				return args[index+1]
			}
			return branch
		}
	}
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg != "" && !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return branch
}
