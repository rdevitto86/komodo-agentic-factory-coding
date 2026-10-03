package doctor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"komodo/internal/fsx"
	"komodo/internal/git"
	"komodo/internal/guard"
	"komodo/internal/install"
	"komodo/internal/lease"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/pr"
	"komodo/internal/proc"
)

// keptRuns is how many run folders Prune keeps, the newest by start; a starting value.
const keptRuns = 10

// idleFor is how long a worktree's git state must sit unchanged before its pushed work alone lets prune take it.
const idleFor = 24 * time.Hour

// now is the clock the idle and lease checks read; tests swap it.
var now = time.Now

// Prune clears stale worktrees, landed refs/komodo tips and old runs, and only lists merged local branches; confirm gates the sweep.
func Prune(root, base string, confirm bool) ([]string, error) {
	var done []string
	worktrees, err := git.Worktrees(root)
	if err != nil {
		return nil, err
	}
	for _, worktree := range worktrees {
		path := worktree.Path
		if !strings.Contains(path, filepath.Join(".komodo", "wt")) {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if _, err := git.Run(root, "worktree", "remove", "--force", path); err == nil {
			done = append(done, "removed worktree "+rel(root, path))
		}
	}
	if _, err := git.Run(root, "worktree", "prune"); err == nil {
		done = append(done, "pruned the worktree list")
	}
	open := line.OpenRuns(root)
	done = append(done, settleShippedRun(root, base, open, confirm)...)
	policy := guard.Load(root, root)
	merged := lines(git.Run(root, "branch", "--merged", base, "--format=%(refname:short)"))
	for _, branch := range merged {
		if branch == base || branch == "" || strings.HasPrefix(branch, "*") || policy.IsCritical(branch) {
			continue
		}
		done = append(done, "merged local branch "+branch+"; komodo never deletes it, run git branch -d "+branch+" to")
	}
	done = append(done, pruneRuns(root, open)...)
	done = append(done, pruneSpentState(root, open, confirm)...)
	done = append(done, pruneStashes(root, now(), confirm)...)
	done = append(done, pruneHookCopies(root, confirm)...)
	done = append(done, pruneClaims(root)...)
	return done, nil
}

// spentDirs are the state folders that hold one file per task or group: briefs and results.
var spentDirs = []string{"briefs", "results"}

// pruneSpentState deletes briefs, results and run archives of each group with no run folder, open run or
// group file; an unreadable backlog deletes nothing.
func pruneSpentState(root string, open []line.RunState, confirm bool) []string {
	parsed, _, err := line.LoadBacklog(root)
	if err != nil {
		return nil
	}
	live := map[string]bool{}
	for _, group := range parsed.Groups {
		live[group.ID] = true
	}
	for _, state := range append(line.LoadRuns(root), open...) {
		live[state.Group] = true
	}
	var spent []string
	state := filepath.Join(root, line.StateDir)
	for _, dir := range spentDirs {
		entries, _ := os.ReadDir(filepath.Join(state, dir))
		for _, entry := range entries {
			if group := groupOf(entry.Name()); group != "" && !live[group] {
				spent = append(spent, filepath.Join(state, dir, entry.Name()))
			}
		}
	}
	archives, _ := filepath.Glob(filepath.Join(state, "line.*.jsonl"))
	for _, archive := range archives {
		if group := groupOf(strings.TrimPrefix(filepath.Base(archive), "line.")); group != "" && !live[group] {
			spent = append(spent, archive)
		}
	}
	var done []string
	for _, path := range spent {
		if !confirm {
			done = append(done, "would remove "+rel(root, path)+", state of a group the line no longer records")
			continue
		}
		if err := os.RemoveAll(path); err == nil {
			done = append(done, "removed "+rel(root, path))
		}
	}
	return done
}

// runSuffix is the start time a run id appends to its group id.
var runSuffix = regexp.MustCompile(`-\d+$`)

// groupOf is the group id a state file's name carries through a task, group or run id, else "".
func groupOf(name string) string {
	for _, ext := range []string{".jsonl", ".json", ".md"} {
		name = strings.TrimSuffix(name, ext)
	}
	if task, ok := strings.CutPrefix(name, "TSK-"); ok {
		if cut := strings.LastIndex(task, "."); cut > 0 {
			return "TG-" + task[:cut]
		}
		return ""
	}
	if !strings.HasPrefix(name, "TG-") {
		return ""
	}
	name = runSuffix.ReplaceAllString(name, "")
	if id, _, found := strings.Cut(name[len("TG-"):], "-"); found {
		return "TG-" + id
	}
	return name
}

// stashAge is how old a stash grows before the sweep archives it as a patch and drops it.
const stashAge = 7 * 24 * time.Hour

// pruneStashes writes each stash older than stashAge to .komodo/stash-archive as a patch, then drops it;
// a stash whose patch cannot be written stays.
func pruneStashes(root string, at time.Time, confirm bool) []string {
	list := lines(git.Run(root, "stash", "list", "--format=%gd %ct %H"))
	var done []string
	// Dropping from the highest index down keeps every lower stash@{n} pointing where it did.
	for index := len(list) - 1; index >= 0; index-- {
		fields := strings.Fields(list[index])
		if len(fields) != 3 {
			continue
		}
		seconds, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || at.Sub(time.Unix(seconds, 0)) < stashAge {
			continue
		}
		patch := filepath.Join(root, line.StateDir, "stash-archive", time.Unix(seconds, 0).UTC().Format("2006-01-02")+"-"+fields[2][:12]+".patch")
		if !confirm {
			done = append(done, "would archive "+fields[0]+" to "+rel(root, patch)+" and drop it")
			continue
		}
		body, err := git.Run(root, "stash", "show", "-p", "--include-untracked", fields[2])
		if err != nil {
			continue
		}
		if err := fsx.WriteFile(patch, []byte(body+"\n"), 0o644); err != nil {
			continue
		}
		if _, err := git.Run(root, "stash", "drop", "--quiet", fields[0]); err == nil {
			done = append(done, "archived "+fields[0]+" to "+rel(root, patch)+" and dropped it")
		}
	}
	return done
}

// pruneHookCopies deletes the hook binary copies earlier installs left, keeping any an installed hook still runs.
func pruneHookCopies(root string, confirm bool) []string {
	keep := map[string]bool{}
	for _, host := range renderInstalled(root, func() func() { return func() {} }) {
		for _, change := range host.Plan.Changes {
			installed, err := os.ReadFile(change.Path)
			if err != nil {
				continue
			}
			for _, binary := range install.HookBinaries(installed) {
				keep[filepath.Clean(binary)] = true
			}
		}
	}
	if !confirm {
		return nil
	}
	removed, err := mount.PruneHookCopies(keep)
	if err != nil {
		return []string{"could not prune old hook binaries: " + err.Error()}
	}
	var done []string
	for _, path := range removed {
		done = append(done, "removed old hook binary "+path)
	}
	return done
}

// settleShippedRun sweeps clean, unleased, landed worktrees and orphan landed tips, never an open run's; unconfirmed, it lists.
func settleShippedRun(root, base string, open []line.RunState, confirm bool) []string {
	if _, err := git.Run(root, "fetch", "--quiet", "--prune", "origin"); err != nil {
		return []string{"skipped the worktree sweep: origin did not answer"}
	}
	remote := "origin/" + base
	running := map[string]bool{}
	for _, state := range open {
		running[state.Branch] = true
		running[filepath.Clean(line.WorktreePath(root, state.Worktree))] = true
		for _, wave := range state.Waves {
			for _, task := range wave {
				running[line.TaskBranch(task)] = true
				running[filepath.Join(root, line.StateDir, "wt", task)] = true
			}
		}
	}
	var done []string
	held := map[string]bool{}
	for _, worktree := range stateWorktrees(root) {
		branch := TrackedOf(worktree)
		held[branch] = true
		if running[branch] || running[filepath.Clean(worktree.Path)] {
			continue
		}
		if _, leased := lease.Held(root, branch, now()); leased {
			continue
		}
		if status, err := git.Run(worktree.Path, "status", "--porcelain"); err != nil || status != "" {
			continue
		}
		tip := worktree.Head
		if ref := git.Or(root, "rev-parse", "--verify", "--quiet", line.TipRef(branch)); ref != "" {
			tip = ref
		}
		pushed := git.Or(worktree.Path, "config", "--worktree", "--get", "komodo.pushed")
		// A commit neither on origin nor the line's own recorded push is unpushed work, whatever else says landed.
		if !safe(root, pushed, worktree.Head) || !safe(root, pushed, tip) {
			continue
		}
		if !landed(root, branch, tip, remote) && !squashLanded(root, branch, worktree.Head, pushed) &&
			!idlePushed(root, worktree.Path, worktree.Head, tip) {
			continue
		}
		if !confirm {
			done = append(done, "would remove worktree "+rel(root, worktree.Path)+" and its ref "+line.TipRef(branch)+
				"; rerun with --confirm to delete")
			continue
		}
		if _, err := git.Run(root, "worktree", "remove", "--force", worktree.Path); err != nil {
			continue
		}
		done = append(done, "removed worktree "+rel(root, worktree.Path))
		if _, err := git.Run(root, "update-ref", "-d", line.TipRef(branch)); err == nil && worktree.Detached {
			done = append(done, "removed ref "+line.TipRef(branch))
		}
	}
	for _, ref := range lines(git.Run(root, "for-each-ref", "--format=%(refname)", "refs/komodo/")) {
		branch := strings.TrimPrefix(ref, "refs/komodo/")
		if branch == "" || held[branch] || running[branch] {
			continue
		}
		if !landed(root, branch, ref, remote) && !squashLanded(root, branch, ref, "") {
			continue
		}
		if !confirm {
			done = append(done, "would remove ref "+ref+"; rerun with --confirm to delete")
			continue
		}
		if _, err := git.Run(root, "update-ref", "-d", ref); err == nil {
			done = append(done, "removed ref "+ref)
		}
	}
	return done
}

// TrackedOf is the branch a worktree holds or, detached, the branch its komodo.branch config names.
func TrackedOf(worktree git.Worktree) string {
	if worktree.Branch != "" {
		return worktree.Branch
	}
	return worktree.Tracked
}

// landed reports whether branch was pushed (origin holds a remote-tracking ref for it) and tip is in remote.
func landed(root, branch, tip, remote string) bool {
	if _, err := git.Run(root, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+branch); err != nil {
		return false
	}
	_, err := git.Run(root, "merge-base", "--is-ancestor", tip, remote)
	return err == nil
}

// idlePushed reports a worktree whose git state sat unchanged for idleFor and whose every commit is on origin.
func idlePushed(root, path string, commits ...string) bool {
	admin, err := git.Run(path, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return false
	}
	var touched time.Time
	for _, name := range []string{"HEAD", "index"} {
		if info, err := os.Stat(filepath.Join(admin, name)); err == nil && info.ModTime().After(touched) {
			touched = info.ModTime()
		}
	}
	if touched.IsZero() || now().Sub(touched) < idleFor {
		return false
	}
	for _, commit := range commits {
		if !onOrigin(root, commit) {
			return false
		}
	}
	return true
}

// safe reports whether commit is on origin or is the push the line recorded for this worktree.
func safe(root, pushed, commit string) bool {
	return commit == pushed || onOrigin(root, commit)
}

// onOrigin reports whether some remote-tracking branch of origin contains commit.
func onOrigin(root, commit string) bool {
	return git.Or(root, "for-each-ref", "--count=1", "--contains", commit, "refs/remotes/origin/") != ""
}

// mergedOnForge reports whether the forge holds a merged pull request headed by branch; no forge says no.
var mergedOnForge = func(root, branch string) bool {
	merged, err := pr.New(root).MergedHead(branch)
	return err == nil && merged
}

// squashLanded reports a squash-merged branch: the forge lists its merged pull request, or origin dropped it at the pushed tip.
func squashLanded(root, branch, tip, pushed string) bool {
	if pushed != "" && pushed == tip && originLacks(root, branch) {
		return true
	}
	return mergedOnForge(root, branch)
}

// originLacks reports whether origin answers and holds no branch of that name; a hung ls-remote
// is killed, process group included, once toolTimeout passes.
func originLacks(root, branch string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--exit-code", "--heads", "origin", branch)
	cmd.Dir = root
	proc.Group(cmd)
	cmd.Cancel = func() error {
		proc.KillGroup(cmd)
		return nil
	}
	cmd.WaitDelay = waitDelay
	err := cmd.Run()
	proc.KillGroup(cmd)
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 2
}

// pruneRuns removes every run folder older than the newest keptRuns, never an open run's.
func pruneRuns(root string, open []line.RunState) []string {
	runs := line.LoadRuns(root)
	if len(runs) <= keptRuns {
		return nil
	}
	running := map[string]bool{}
	for _, state := range open {
		running[state.Group] = true
	}
	var done []string
	for _, state := range runs[:len(runs)-keptRuns] {
		if running[state.Group] {
			continue
		}
		dir := line.RunDir(root, state.Group)
		if err := os.RemoveAll(dir); err == nil {
			done = append(done, "removed run folder "+rel(root, dir))
		}
	}
	return done
}

// stateWorktrees lists the worktrees under .komodo/wt that exist on disk and hold or track a branch.
func stateWorktrees(root string) []git.Worktree {
	worktrees, err := git.Worktrees(root)
	if err != nil {
		return nil
	}
	var found []git.Worktree
	for _, current := range worktrees {
		if TrackedOf(current) != "" && strings.Contains(current.Path, filepath.Join(".komodo", "wt")) && exists(current.Path) {
			found = append(found, current)
		}
	}
	return found
}

// claimsDir is where the guard kept branch claims until they were removed; nothing reads it now.
const claimsDir = "komodo-claims"

// pruneClaims deletes the shared git dir's dead branch-claim directory.
func pruneClaims(root string) []string {
	common, err := git.Run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return nil
	}
	dir := filepath.Join(common, claimsDir)
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	if err := os.RemoveAll(dir); err != nil {
		return []string{"could not remove the dead branch claims at " + dir + ": " + err.Error()}
	}
	return []string{"removed the dead branch claims at " + dir}
}
