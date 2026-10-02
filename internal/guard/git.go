package guard

import (
	"fmt"
	"path/filepath"
	"strings"

	"komodo/internal/git"
)

// noVerifyCommands are the git subcommands whose --no-verify skips the gate's own hook.
var noVerifyCommands = map[string]bool{
	"commit": true, "merge": true, "push": true, "rebase": true, "am": true, "cherry-pick": true,
}

// gitFindings checks one git call against the guard's rules: a critical ref is never committed,
// pushed, merged, or moved by hand; pushed history and hooks hold; a branch is never pinned.
func gitFindings(words []string, dir, branch string, policy Policy) []string {
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
	case "worktree":
		findings = append(findings, worktreeFindings(rest, policy)...)
	case "checkout":
		findings = append(findings, checkoutFindings(rest, gitDirectory(dir, words[1:]), policy)...)
	case "switch":
		findings = append(findings, switchFindings(rest, gitDirectory(dir, words[1:]), policy)...)
	case "update-ref":
		findings = append(findings, updateRefFindings(rest, policy)...)
	}
	return findings
}

// worktreeAttach names what komodo worktree add or --detach replaces, for every attach refusal.
const worktreeAttach = "name komodo worktree add or git switch --detach"

// worktreeFindings refuses a git worktree add that attaches a branch instead of detaching one, and
// a -B that would force-move a critical ref.
func worktreeFindings(rest []string, policy Policy) []string {
	if len(rest) == 0 || rest[0] != "add" {
		return nil
	}
	args := rest[1:]
	if target, ok := flagValue(args, "-B"); ok && normalizeMode(policy.Mode) != ModeUnsafe && policy.IsCritical(target) {
		return []string{fmt.Sprintf("git worktree add -B %s: a critical ref is never moved by hand; %s", target, worktreeAttach)}
	}
	detached, attach := false, false
	for _, arg := range args {
		switch arg {
		case "--detach":
			detached = true
		case "-b", "-B":
			attach = true
		}
	}
	switch {
	case attach:
		return []string{"git worktree add -b/-B attaches a branch; " + worktreeAttach}
	case !detached:
		return []string{"git worktree add without --detach attaches a branch; " + worktreeAttach}
	}
	return nil
}

// checkoutFindings refuses checkout -B onto a critical ref, and, in a linked worktree, a plain
// checkout onto a branch git already has or would create tracking a same-named remote branch.
func checkoutFindings(rest []string, dir string, policy Policy) []string {
	var findings []string
	if target, ok := flagValue(rest, "-B"); ok && normalizeMode(policy.Mode) != ModeUnsafe && policy.IsCritical(target) {
		findings = append(findings, fmt.Sprintf("git checkout -B %s: a critical ref is never moved by hand; %s", target, worktreeAttach))
	}
	if target := plainCheckoutTarget(rest); target != "" && isLinkedWorktree(dir) && attachesBranch(dir, target) {
		findings = append(findings, fmt.Sprintf("git checkout %s: a linked worktree never attaches a branch; %s", target, worktreeAttach))
	}
	return findings
}

// switchFindings refuses switch -C onto a critical ref, and, in a linked worktree, a plain switch
// onto a branch git already has or would create tracking a same-named remote branch.
func switchFindings(rest []string, dir string, policy Policy) []string {
	var findings []string
	if target, ok := flagValue(rest, "-C"); ok && normalizeMode(policy.Mode) != ModeUnsafe && policy.IsCritical(target) {
		findings = append(findings, fmt.Sprintf("git switch -C %s: a critical ref is never moved by hand; %s", target, worktreeAttach))
	}
	if target := plainSwitchTarget(rest); target != "" && isLinkedWorktree(dir) && attachesBranch(dir, target) {
		findings = append(findings, fmt.Sprintf("git switch %s: a linked worktree never attaches a branch; %s", target, worktreeAttach))
	}
	return findings
}

// updateRefFindings refuses a git update-ref that moves a critical ref by hand.
func updateRefFindings(rest []string, policy Policy) []string {
	if normalizeMode(policy.Mode) == ModeUnsafe {
		return nil
	}
	for _, arg := range rest {
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}
		ref := strings.TrimPrefix(arg, "refs/heads/")
		if policy.IsCritical(ref) {
			return []string{fmt.Sprintf("git update-ref %s: a critical ref is never moved by hand; %s", arg, worktreeAttach)}
		}
		break
	}
	return nil
}

// flagValue is the word after flag's first occurrence in args, or nothing when flag is absent or last.
func flagValue(args []string, flag string) (string, bool) {
	for index, arg := range args {
		if arg == flag && index+1 < len(args) {
			return args[index+1], true
		}
	}
	return "", false
}

// plainCheckoutTarget is the branch a checkout with no -b, -B, or --detach would attach, or "".
func plainCheckoutTarget(rest []string) string {
	for _, arg := range rest {
		if arg == "-b" || arg == "-B" || arg == "--detach" {
			return ""
		}
	}
	return checkoutTarget(rest, "")
}

// plainSwitchTarget is the branch a switch with no -c, -C, or --detach would attach, or "".
func plainSwitchTarget(rest []string) string {
	for _, arg := range rest {
		if arg == "-c" || arg == "-C" || arg == "--create" || arg == "--force-create" || arg == "--detach" {
			return ""
		}
	}
	return switchTarget(rest, "")
}

// isLinkedWorktree reports whether dir is a linked worktree, not the repository's main checkout.
func isLinkedWorktree(dir string) bool {
	gitDir := git.Or(dir, "rev-parse", "--path-format=absolute", "--git-dir")
	common := git.Or(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	return gitDir != "" && gitDir != common
}

// attachesBranch reports whether target names a local branch dir already has, or one with no local
// branch git would create there tracking a same-named branch on origin; either way an attach.
func attachesBranch(dir, target string) bool {
	if target == "" {
		return false
	}
	if _, err := git.Run(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+target); err == nil {
		return true
	}
	_, err := git.Run(dir, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+target)
	return err == nil
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

// gitDirectory is where one git call runs: dir, moved by each leading -C flag in turn.
func gitDirectory(dir string, args []string) string {
	for index := 0; index < len(args) && strings.HasPrefix(args[index], "-"); index++ {
		if args[index] == "-c" {
			index++
			continue
		}
		if args[index] == "-C" && index+1 < len(args) {
			index++
			dir = resolveDir(dir, args[index])
		}
	}
	return dir
}

// resolveDir joins path onto dir unless it is absolute, expanding a leading tilde first.
func resolveDir(dir, path string) string {
	path = expandHome(path)
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(dir, path)
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

// switchTarget is the branch git switch moves to: HEAD on --detach/-d, the name after
// -c/-C/--create, or the first positional operand, or branch unchanged when none names one.
func switchTarget(args []string, branch string) string {
	for _, arg := range args {
		if arg == "--detach" || arg == "-d" {
			return "HEAD"
		}
	}
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

// checkoutTarget is the branch git checkout moves to: HEAD on --detach, the name after -b/-B, or
// the first positional operand before a `--` pathspec separator, or branch unchanged when none names one.
func checkoutTarget(args []string, branch string) string {
	for _, arg := range args {
		if arg == "--detach" {
			return "HEAD"
		}
	}
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
