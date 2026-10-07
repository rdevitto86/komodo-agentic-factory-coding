package guard

import (
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/git"
	"komodo/internal/lease"
)

// noVerifyCommands are the git subcommands whose --no-verify skips the gate's own hook.
var noVerifyCommands = map[string]bool{
	"commit": true, "merge": true, "push": true, "rebase": true, "am": true, "cherry-pick": true,
}

// gitFindings checks one git call against the guard's rules: a critical ref is never committed,
// pushed, merged, or moved by hand; pushed history and hooks hold; a branch is never pinned.
func gitFindings(words []string, dir, branch string, policy Policy) []finding {
	args := skipGlobalFlags(words[1:])
	if len(args) == 0 {
		return nil
	}
	sub, rest := args[0], args[1:]
	var findings []finding
	if noVerifyCommands[sub] && normalizeMode(policy.Mode) != ModeUnsafe && hasNoVerify(sub, rest) {
		findings = append(findings, newFinding("git %s --no-verify skips the gate; fix what it reports instead", sub))
	}
	switch sub {
	case "push":
		findings = append(findings, pushFindings(rest, dir, branch, policy)...)
	case "commit":
		if policy.IsCritical(branch) && normalizeMode(policy.Mode) != ModeUnsafe {
			findings = append(findings, newFinding("git commit on %s: create a branch first", branch))
		}
	case "merge":
		if (policy.IsCritical(branch) || harnessEpicBranch(branch)) && normalizeMode(policy.Mode) != ModeUnsafe {
			findings = append(findings, newFinding("git merge on %s: landing is the human's merge button", branch))
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
		findings = append(findings, updateRefFindings(rest, dir, policy)...)
	}
	return findings
}

// worktreeAttach names what komodo worktree add or --detach replaces, for every attach refusal.
const worktreeAttach = "name komodo worktree add or git switch --detach"

// worktreeFindings refuses a git worktree add that attaches a branch instead of detaching one, and
// a -B that would force-move a critical ref.
func worktreeFindings(rest []string, policy Policy) []finding {
	if len(rest) == 0 || rest[0] != "add" {
		return nil
	}
	args := rest[1:]
	if target, ok := flagValue(args, "-B"); ok && normalizeMode(policy.Mode) != ModeUnsafe && policy.IsCritical(target) {
		return []finding{newFinding("git worktree add -B %s: a critical ref is never moved by hand; %s", target, worktreeAttach)}
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
		return []finding{newFinding("git worktree add -b/-B attaches a branch; " + worktreeAttach)}
	case !detached:
		return []finding{newFinding("git worktree add without --detach attaches a branch; " + worktreeAttach)}
	}
	return nil
}

// checkoutFindings refuses checkout -B onto a critical ref, and, in a linked worktree, a plain
// checkout onto a branch git already has or would create tracking a same-named remote branch.
func checkoutFindings(rest []string, dir string, policy Policy) []finding {
	var findings []finding
	if target, ok := flagValue(rest, "-B"); ok && normalizeMode(policy.Mode) != ModeUnsafe && policy.IsCritical(target) {
		findings = append(findings, newFinding("git checkout -B %s: a critical ref is never moved by hand; %s", target, worktreeAttach))
	}
	if target := plainCheckoutTarget(rest); target != "" && isLinkedWorktree(dir) && attachesBranch(dir, target) {
		findings = append(findings, newFinding("git checkout %s: a linked worktree never attaches a branch; %s", target, worktreeAttach))
	}
	return findings
}

// switchFindings refuses switch -C onto a critical ref, and, in a linked worktree, a plain switch
// onto a branch git already has or would create tracking a same-named remote branch.
func switchFindings(rest []string, dir string, policy Policy) []finding {
	var findings []finding
	if target, ok := flagValue(rest, "-C"); ok && normalizeMode(policy.Mode) != ModeUnsafe && policy.IsCritical(target) {
		findings = append(findings, newFinding("git switch -C %s: a critical ref is never moved by hand; %s", target, worktreeAttach))
	}
	if target := plainSwitchTarget(rest); target != "" && isLinkedWorktree(dir) && attachesBranch(dir, target) {
		findings = append(findings, newFinding("git switch %s: a linked worktree never attaches a branch; %s", target, worktreeAttach))
	}
	return findings
}

// updateRefFindings refuses a git update-ref that moves a critical ref by hand.
func updateRefFindings(rest []string, dir string, policy Policy) []finding {
	if normalizeMode(policy.Mode) == ModeUnsafe {
		return nil
	}
	for _, arg := range rest {
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}
		ref := strings.TrimPrefix(arg, "refs/heads/")
		if policy.IsCritical(ref) {
			return []finding{newFinding("git update-ref %s: a critical ref is never moved by hand; %s", arg, worktreeAttach)}
		}
		if tip, ok := strings.CutPrefix(arg, "refs/komodo/"); ok {
			return leaseFindings(dir, tip, "git update-ref "+arg)
		}
		break
	}
	return nil
}

// leaseFindings refuses an orchestrator session's write to a branch a live builder holds, unless it is that builder's own run.
func leaseFindings(dir, branch, what string) []finding {
	if IsHarnessSession() {
		return nil
	}
	held, ok := lease.Held(dir, branch, time.Now())
	if !ok || held.Own() {
		return nil
	}
	return []finding{newFinding("%s: %s", what, held.Refusal())}
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

// noVerifyValueFlags are git commit's short options whose value swallows a cluster's trailing n.
var noVerifyValueFlags = map[rune]bool{'m': true, 'c': true, 'C': true, 'F': true}

// hasNoVerify reports whether rest skips hooks: --no-verify or an unambiguous prefix of it, or,
// for commit, a short cluster carrying -n ahead of any flag that takes a value, such as -nm.
func hasNoVerify(sub string, rest []string) bool {
	for _, arg := range rest {
		if strings.HasPrefix(arg, "--no-v") && strings.HasPrefix("--no-verify", arg) {
			return true
		}
		if sub != "commit" || !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
			continue
		}
		for _, char := range arg[1:] {
			if char == 'n' {
				return true
			}
			if noVerifyValueFlags[char] {
				break
			}
		}
	}
	return false
}

// isForceFlag reports whether a push option rewrites the remote's history: the long spellings, or
// any short-option cluster that packs in -f, such as -fu.
func isForceFlag(arg string) bool {
	if arg == "--force" || arg == "--force-if-includes" || strings.HasPrefix(arg, "--force-with-lease") {
		return true
	}
	return strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.ContainsRune(arg, 'f')
}

// pushFindings refuses a push that rewrites history or lands on a critical ref.
func pushFindings(rest []string, dir, branch string, policy Policy) []finding {
	var findings []finding
	var positional []string
	deletes := false
	for _, arg := range rest {
		if isForceFlag(arg) {
			findings = append(findings, newFinding("git push %s: pushed history is never rewritten; push a new commit instead", arg))
		}
		switch {
		case arg == "--delete" || arg == "-d":
			deletes = true
		case arg != "" && !strings.HasPrefix(arg, "-"):
			positional = append(positional, strings.TrimPrefix(arg, "+"))
		}
	}
	targets := []string{branch}
	if len(positional) > 1 {
		targets = positional[1:]
	}
	for index, target := range targets {
		if _, after, found := strings.Cut(target, ":"); found {
			target = after
		}
		if target == "HEAD" {
			target = branch
		}
		targets[index] = target
	}
	// A critical-ref delete and an epic-branch push or delete are refused in every mode, unsafe included.
	verb := "push to"
	if deletes {
		verb = "delete"
	}
	for _, target := range targets {
		switch {
		case deletes && policy.IsCritical(target):
			findings = append(findings, newFinding("git delete %s: open a pull request instead", target))
		case harnessEpicBranch(target):
			findings = append(findings, newFinding("git %s %s: open a pull request instead", verb, target))
		}
	}
	if normalizeMode(policy.Mode) == ModeUnsafe {
		return findings
	}
	for _, target := range targets {
		if policy.IsCritical(target) && !deletes {
			findings = append(findings, newFinding("git push to %s: open a pull request instead", target))
		}
		findings = append(findings, leaseFindings(dir, strings.TrimPrefix(target, "refs/heads/"), "git push to "+target)...)
	}
	return findings
}

// branchFindings refuses a branch delete, force, or move that names a critical ref.
// Forcing and copying judge only the ref actually moved or created, never one merely read.
func branchFindings(rest []string, policy Policy) []finding {
	if normalizeMode(policy.Mode) == ModeUnsafe {
		return nil
	}
	deleting, forcing, moving, copying := false, false, false, false
	var positional []string
	for _, arg := range rest {
		switch arg {
		case "-d", "-D", "--delete":
			deleting = true
		case "-f", "--force":
			forcing = true
		case "-m", "-M", "--move":
			moving = true
		case "-c", "-C", "--copy":
			copying = true
		default:
			if !strings.HasPrefix(arg, "-") {
				positional = append(positional, arg)
			}
		}
	}
	if !deleting && !forcing && !moving && !copying {
		return nil
	}
	targets := positional
	switch {
	case !deleting && !moving && copying && len(positional) > 0:
		targets = positional[len(positional)-1:]
	case !deleting && !moving && !copying && forcing && len(positional) > 0:
		targets = positional[:1]
	}
	var findings []finding
	for _, arg := range targets {
		if !policy.IsCritical(arg) {
			continue
		}
		verb := "moved"
		if deleting {
			verb = "deleted"
		}
		findings = append(findings, newFinding("git branch %s: a critical ref is never %s by hand", arg, verb))
	}
	return findings
}

// harnessEpicBranch reports whether a harness session names an epic branch, which only the conductor pushes and merges.
func harnessEpicBranch(branch string) bool {
	return IsHarnessSession() && IsEpicBranch(branch)
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
