package line

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/changelog"
	"komodo/internal/git"
	"komodo/internal/guard"
	"komodo/internal/ledger"
	"komodo/internal/pr"
)

// ShipResult is what the ship station did with one group.
type ShipResult struct {
	Group     string         `json:"group"`
	Branch    string         `json:"branch"`
	Base      string         `json:"base"`
	StaleBase string         `json:"stale_base,omitempty"`
	URL       string         `json:"url,omitempty"`
	Draft     bool           `json:"draft"`
	Ready     bool           `json:"ready,omitempty"`
	Labels    []string       `json:"labels,omitempty"`
	Changelog string         `json:"changelog,omitempty"`
	Done      []string       `json:"done,omitempty"`
	Blocked   []string       `json:"blocked,omitempty"`
	Filed     []string       `json:"filed,omitempty"`
	Warnings  []string       `json:"warnings,omitempty"`
	Published *CommandResult `json:"published,omitempty"`
}

// ShipHandoff is what a scrubbed ship leaves for a credentialed process to push and open.
type ShipHandoff struct {
	Group        string   `json:"group"`
	Worktree     string   `json:"worktree"`
	Branch       string   `json:"branch"`
	Base         string   `json:"base"`
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	Labels       []string `json:"labels,omitempty"`
	Draft        bool     `json:"draft"`
	AfterPublish string   `json:"after_publish,omitempty"`
}

// dropped are the environment variables that would hand a process a push credential.
var dropped = []string{
	"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GIT_ASKPASS", "SSH_AUTH_SOCK",
	"GIT_CONFIG_PARAMETERS",
}

// forgeWords and secretWords are the name parts that together mark a push credential.
var (
	forgeWords  = map[string]bool{"GIT": true, "GITHUB": true, "GH": true, "GITLAB": true, "GL": true, "BITBUCKET": true}
	secretWords = map[string]bool{"TOKEN": true, "PAT": true, "SECRET": true, "PASSWORD": true, "KEY": true}
)

// Scrub returns the environment with every push credential removed and git left unable to prompt.
func Scrub(base []string) []string {
	out := make([]string, 0, len(base)+6)
	for _, entry := range base {
		key, _, found := strings.Cut(entry, "=")
		if !found || slices.Contains(dropped, key) || credentialShaped(key) || isOverride(key) {
			continue
		}
		out = append(out, entry)
	}
	return append(out,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=credential.helper",
		"GIT_CONFIG_VALUE_0=",
		"GIT_SSH_COMMAND=ssh -F "+os.DevNull+" -o BatchMode=yes -o IdentitiesOnly=yes -o IdentityFile="+os.DevNull,
		"GH_CONFIG_DIR="+filepath.Join(os.TempDir(), "komodo-gh-noauth"),
	)
}

// credentialShaped reports whether a key names a forge's secret, such as GITHUB_PAT or GITLAB_TOKEN,
// so a push credential is dropped while the model host keeps its own login.
func credentialShaped(key string) bool {
	forge, secret := false, false
	for _, part := range strings.Split(key, "_") {
		forge = forge || forgeWords[part]
		secret = secret || secretWords[part]
	}
	return forge && secret
}

// isOverride reports whether Scrub sets this key itself, so an inherited value never survives.
func isOverride(key string) bool {
	switch key {
	case "GIT_TERMINAL_PROMPT", "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0",
		"GIT_CONFIG_VALUE_0", "GIT_SSH_COMMAND", "GH_CONFIG_DIR":
		return true
	}
	return false
}

// scrubbed reports whether Scrub stripped this process of every push credential.
func scrubbed() bool {
	return os.Getenv("GIT_TERMINAL_PROMPT") == "0" &&
		os.Getenv("GIT_CONFIG_KEY_0") == "credential.helper" &&
		os.Getenv("GIT_CONFIG_VALUE_0") == ""
}

// writeShipHandoff writes the branch, title, body, labels and draft flag a later push finishes,
// under the handoff's own group's run directory.
func writeShipHandoff(root string, handoff ShipHandoff) error {
	if !PlainGroup(handoff.Group) {
		return fmt.Errorf("a ship handoff needs a plain group id, not %q", handoff.Group)
	}
	path := HandoffPath(root, handoff.Group)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(handoff, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// ShipGroup commits, pushes, opens the pull request, writes the changelog, and flips the statuses.
// A scrubbed environment commits and hands the push off instead of failing on a credential it lacks.
func ShipGroup(root string, plan *Plan, waves []*WaveResult, client *pr.Client) (result *ShipResult, err error) {
	if plan.WaitUntil != "" {
		return nil, fmt.Errorf("%s is paused until %s; a plan whose waves a pause blanked cannot ship",
			plan.Group, plan.WaitUntil)
	}
	if waves == nil {
		// Ship runs as its own process after QC, so it reads what each wave ran from the ledger.
		waves = RecordedWaves(root, plan.Group)
	}
	group := WorktreePath(root, plan.Worktree)
	rootParsed, _, err := LoadBacklog(root)
	if err != nil {
		return nil, err
	}
	groupParsed, err := backlog.LoadRoot(group)
	if err != nil {
		return nil, err
	}
	if refined := refinedTasks(rootParsed, groupParsed, plan.Tasks); len(refined) > 0 {
		return nil, fmt.Errorf("the root's backlog differs from this group's queue at %s; rebase or edit before ship",
			strings.Join(refined, ", "))
	}
	// A missing review reads as no findings, so ship refuses rather than ship an unreviewed group.
	if !HasResult(root, plan.Group+"-review") {
		return nil, fmt.Errorf("%s has no review result; run the review, then ship", plan.Group)
	}
	if staleReview(root, plan) {
		return nil, fmt.Errorf("%s changed after its review; run the review again, then ship", plan.Group)
	}
	live := LoadStatus(root)
	blocking, minor := SplitFindings(ReviewFindings(root, plan.Group), plan.Profile.SeverityFloor)
	if len(blocking) > 0 {
		return nil, fmt.Errorf("the review left %d finding(s) at or above %s; fix them on %s, then ship",
			len(blocking), plan.Profile.SeverityFloor, plan.Branch)
	}
	started := time.Now()
	result = &ShipResult{Group: plan.Group, Branch: plan.Branch, Base: liveBase(root, plan.Base)}
	if result.Base != plan.Base {
		result.StaleBase = plan.Base
	}
	if err := catchUp(group, result.Base); err != nil {
		return nil, err
	}
	outcome := ""
	lines := 0
	defer func() {
		if outcome == "" {
			outcome = "done"
			if err != nil {
				outcome = "failed"
			}
		}
		Stamp(root, ledger.Entry{Group: plan.Group, Station: "ship", Seconds: Since(started), Outcome: outcome, Lines: lines})
	}()
	if err := tickTasks(plan, group, rootParsed, live, result); err != nil {
		return nil, err
	}
	result.Draft = len(result.Blocked) > 0
	var shippedIDs []string
	for _, task := range plan.Tasks {
		shippedIDs = append(shippedIDs, task.ID)
	}
	declared := declaredFiles(plan)
	if err := stageWork(group, declared); err != nil {
		return nil, err
	}
	// File findings before push so they are in the ship commit.
	filed, err := FileFindings(group, plan.Group, minor)
	if err != nil {
		return result, err
	}
	result.Filed = filed
	if len(filed) > 0 {
		// Stage whichever backlog file the findings landed in for commit.
		stagePath, err := findingsPath(group, plan.Group)
		if err != nil {
			return nil, err
		}
		if err := stageWork(group, []string{stagePath}); err != nil {
			return nil, err
		}
	}
	if staged, _ := git.Run(group, "diff", "--cached", "--name-only"); staged != "" {
		message := fmt.Sprintf("%s: %s (%s)", plan.Type, plan.Title, plan.Group)
		if _, err := git.Run(group, "commit", "-m", message); err != nil {
			return nil, err
		}
	}
	if err := ClearStatus(root, shippedIDs); err != nil {
		return nil, err
	}
	lines = ChangedLines(group, StartRef(group, plan.Base), plan.Branch)
	files, added := ReviewSize(group, StartRef(group, plan.Base), plan.Branch)
	if err := checkPRSize(plan.Group, files, added, plan.Profile.PRFiles, plan.Profile.PRLinesMax); err != nil {
		return nil, err
	}
	if isToolkit(root) {
		if err := gateCommand(group); err != nil {
			return nil, fmt.Errorf("gate: %w", err)
		}
	}
	title := fmt.Sprintf("%s: %s (%s)", plan.Type, plan.Title, plan.Group)
	context := BodyContext{Sections: templateSections(group), DefaultBase: DefaultBase(root)}
	context.BlastRadius, context.BlastRadiusWhy = reviewBlast(root, plan.Group)
	context.SizeNote = sizeNote(added, plan.Profile.PRLinesPreferred)
	body := ReportBody(plan, result, waves, context)
	wanted := []string{"@agent", ScopeLabel(root, declared)}
	handoff := ShipHandoff{
		Group: plan.Group, Worktree: group, Branch: plan.Branch, Base: result.Base, Title: title, Body: body,
		Labels: wanted, Draft: true,
		AfterPublish: AfterPublishCommand(root, group),
	}
	if scrubbed() {
		outcome = "handoff"
		if err := writeShipHandoff(root, handoff); err != nil {
			return nil, err
		}
		return result, nil
	}
	if err := PushFromWorktree(root, group, plan.Branch); err != nil {
		if !errors.Is(err, ErrNoCredential) {
			return nil, err
		}
		// The group keeps its commits and stops before Ship; komodo ship finishes it once a credential is back.
		outcome = "handoff"
		return result, errors.Join(err, writeShipHandoff(root, handoff), writeCredentialNote(root, plan, group, err))
	}
	// The credential stays with the push; after_publish is repo-written, so it runs scrubbed.
	if command := AfterPublishCommand(root, group); command != "" {
		published := RunCommandEnv(group, command, Scrub(os.Environ()))
		result.Published = &published
		if !published.OK() {
			return result, fmt.Errorf("after_publish: %s", FailureText(published))
		}
	}
	if client == nil {
		return result, nil
	}
	// Every PR opens as a draft, or labelled status: wip where the forge refuses one.
	url, draft, wip, warnings, err := createEpicPull(client, result.Base, plan.Branch, title, body)
	if err != nil {
		// A re-ship after an escalation finds its pull request still open, and refreshes it instead.
		open, viewErr := client.View(plan.Branch)
		if viewErr != nil || open.State != "OPEN" {
			return result, err
		}
		if err := client.Edit(open.URL, "--title", title, "--body", body); err != nil {
			return result, err
		}
		url, draft = open.URL, open.Draft
	}
	result.URL, result.Draft = url, draft
	kept, labelWarnings := ApplyLabelSet(client, url, wanted, OptionalLabels(root, result.Base))
	result.Warnings = append(warnings, labelWarnings...)
	if len(result.Blocked) > 0 || !checksPassed(waves) {
		result.Labels = append(wip, kept...)
		return result, nil
	}
	if err := markReady(client, url, draft); err != nil {
		result.Labels = append(wip, kept...)
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not mark the PR ready for review: %v", err))
		return result, nil
	}
	result.Labels, result.Draft, result.Ready = kept, false, true
	return result, nil
}

// checksPassed reports whether the group ran its checks and every one passed.
func checksPassed(waves []*WaveResult) bool {
	if len(waves) == 0 {
		return false
	}
	for _, wave := range waves {
		if wave == nil || !wave.OK {
			return false
		}
	}
	return true
}

// markReady turns a PR ready for review: a draft leaves draft, and a normal PR drops the status: wip label.
func markReady(client *pr.Client, url string, draft bool) error {
	if draft {
		return client.Ready(url)
	}
	known, err := client.Labels()
	if err != nil {
		return err
	}
	if label := wipLabel(known); label != "" {
		return client.Unlabel(url, []string{label})
	}
	return nil
}

// tickTasks sorts the plan's tasks into done and blocked, writes each tick and blocker into the
// group's own backlog, and writes its changelog fragment, so the group's commit carries both.
func tickTasks(
	plan *Plan, group string, rootParsed backlog.Backlog, live map[string]TaskStatus, result *ShipResult,
) error {
	// A no-epic group's own file is already gone once an earlier commit shipped it; nothing left to tick.
	_, _, ownFileGone, err := backlog.FindGroupFile(group, plan.Group)
	if err != nil {
		return err
	}
	ownFileGone = !ownFileGone
	for _, task := range plan.Tasks {
		current, ok := rootParsed.Task(task.ID)
		if !ok {
			continue
		}
		// Only a task close marked DONE shipped; a blocked one, or a dependent step skipped, did not.
		if current.Status == "DONE" {
			result.Done = append(result.Done, task.ID)
		} else {
			result.Blocked = append(result.Blocked, task.ID)
		}
	}
	for _, taskID := range result.Done {
		if err := writeStatus(group, taskID, "DONE"); err != nil && !(ownFileGone && errors.Is(err, errTaskGroupFileGone)) {
			return err
		}
	}
	// This group's ticks and blockers land in its group file once, in its commit; a status in flight never does.
	for _, task := range plan.Tasks {
		if status, ok := live[task.ID]; ok && backlogStatus(status.Status) {
			if err := writeStatus(group, task.ID, status.Status); err != nil && !(ownFileGone && errors.Is(err, errTaskGroupFileGone)) {
				return err
			}
		}
	}
	// A fragment per group, never an edit to CHANGELOG.md, so two open pull requests never conflict there.
	if line := ChangelogLine(plan, result); line != "" {
		if err := changelog.WriteFragment(group, plan.Version, plan.Group, line); err != nil {
			return err
		}
		result.Changelog = line
	}
	return nil
}

// findingsPath is the file FileFindings wrote into, relative to group: its own docs/backlog file.
func findingsPath(group, groupID string) (string, error) {
	path, _, found, err := backlog.FindGroupFile(group, groupID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("%s is not in %s", groupID, backlog.GroupFilesDir)
	}
	return filepath.Rel(group, path)
}

// declaredFiles is every file the plan's tasks declare.
func declaredFiles(plan *Plan) []string {
	var declared []string
	for _, task := range plan.Tasks {
		declared = append(declared, task.Files...)
	}
	return declared
}

// StationPrepare is the ledger station Prepare stamps, always before any push.
const StationPrepare = "prepare"

// PrepareGroup commits the group's work with its ticked tasks and changelog fragment, runs its pre-commit and
// pre-push hooks, and rebases it on its base; a hook's refusal and each conflicted file come back as fixes.
func PrepareGroup(root string, plan *Plan) (fixes []string, err error) {
	started := time.Now()
	defer func() {
		outcome := "done"
		if err != nil {
			outcome = "failed"
		} else if len(fixes) > 0 {
			outcome = "fixes"
		}
		Stamp(root, ledger.Entry{Group: plan.Group, Station: StationPrepare, Seconds: Since(started), Outcome: outcome})
	}()
	group := WorktreePath(root, plan.Worktree)
	base := liveBase(root, plan.Base)
	declared := declaredFiles(plan)
	if fixes, err := finishCatchUp(group, base, declared); err != nil || len(fixes) > 0 {
		return fixes, err
	}
	if backlog.Exists(group) {
		rootParsed, _, err := LoadBacklog(root)
		if err != nil {
			return nil, err
		}
		if err := tickTasks(plan, group, rootParsed, LoadStatus(root), &ShipResult{}); err != nil {
			return nil, err
		}
	}
	ended, err := endedEpicFiles(group, plan.Group)
	if err != nil {
		return nil, err
	}
	for _, file := range ended {
		if err := os.Remove(file); err != nil {
			return nil, err
		}
	}
	if err := stageWork(group, declared); err != nil {
		return nil, err
	}
	staged, err := git.Run(group, "diff", "--cached", "--name-only")
	if err != nil {
		return nil, err
	}
	// Committing runs the pre-commit hook; with nothing to commit, the hook runs on its own.
	hook := []string{"hook", "run", "--ignore-missing", "pre-commit"}
	if staged != "" {
		hook = []string{"commit", "-m", fmt.Sprintf("%s: %s (%s)", plan.Type, plan.Title, plan.Group)}
	}
	if _, err := git.Run(group, hook...); err != nil {
		return []string{"the group does not commit: " + err.Error()}, nil
	}
	pushURL, _ := git.Run(root, "remote", "get-url", "--push", "origin")
	clean, _, _ := splitCredential(pushURL)
	if err := runPrePush(group, clean, "refs/heads/"+plan.Branch); err != nil {
		return []string{"the pre-push hook refuses the group: " + redactURL(err.Error(), pushURL)}, nil
	}
	return rebaseForRepair(group, base)
}

// endedEpicFiles lists the docs/backlog group files a group's commit deletes: its epic's every file once no
// other group of that epic is open, or its own file when it names no epic.
func endedEpicFiles(worktree, groupID string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(worktree, "docs", "backlog", "*.md"))
	if err != nil {
		return nil, err
	}
	files := make(map[string]backlog.GroupFile, len(paths))
	own := ""
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		files[path] = backlog.ParseGroupFile(string(data))
		if files[path].ID == groupID {
			own = path
		}
	}
	if own == "" {
		return nil, nil
	}
	epic := files[own].EpicID
	if epic == "" {
		return []string{own}, nil
	}
	var ended []string
	for _, path := range paths {
		file := files[path]
		if file.EpicID != epic {
			continue
		}
		if file.ID != groupID && slices.ContainsFunc(file.Tasks, func(task backlog.GroupTask) bool { return !task.Done }) {
			return nil, nil
		}
		ended = append(ended, path)
	}
	return ended, nil
}

// conflictFixes turns each conflicted file into a fix naming the base it conflicts with.
func conflictFixes(files []string, target string) []string {
	fixes := make([]string, 0, len(files))
	for _, file := range files {
		fixes = append(fixes, fmt.Sprintf(
			"%s: resolve the conflict with %s; edit out every conflict marker, keeping both sides' intent", file, target))
	}
	return fixes
}

// rebaseForRepair rebases the group onto the latest base, or merges it into a pushed branch; a conflict
// leaves its markers in the worktree and returns one fix per conflicted file.
func rebaseForRepair(group, base string) ([]string, error) {
	if hasOrigin(group) {
		// A base origin lacks has nothing newer to catch up to; the push reports an unreachable origin.
		_ = Fetch(group, base)
	}
	target := StartRef(group, base)
	if _, err := git.Run(group, "rev-parse", "--verify", "--quiet", target); err != nil {
		return nil, nil
	}
	if _, err := git.Run(group, "merge-base", "--is-ancestor", target, "HEAD"); err == nil {
		return nil, nil
	}
	args, abort := []string{"rebase", "--autostash", target}, []string{"rebase", "--abort"}
	// A pushed branch is never rewritten, so it takes the base in a merge commit instead.
	if branch, err := git.Run(group, "rev-parse", "--abbrev-ref", "HEAD"); err == nil && onOrigin(group, strings.TrimSpace(branch)) {
		args, abort = []string{"merge", "--autostash", "--no-edit", target}, []string{"merge", "--abort"}
	}
	return settleCatchUp(group, target, abort, args...)
}

// settleCatchUp runs one rebase or merge step, returning its conflicted files as fixes; a failure with
// no conflict aborts the step and is returned.
func settleCatchUp(group, target string, abort []string, args ...string) ([]string, error) {
	_, err := git.Run(group, args...)
	if err == nil {
		return nil, nil
	}
	conflicts, _ := git.Run(group, "diff", "--name-only", "--diff-filter=U")
	if files := strings.Fields(conflicts); len(files) > 0 {
		return conflictFixes(files, target), nil
	}
	_, _ = git.Run(group, abort...)
	return nil, err
}

// errConflictRemains stops a group whose repair round left conflict markers in its files.
var errConflictRemains = errors.New("the conflict remains after its repair round")

// finishCatchUp completes a rebase or merge a conflict left open, once its repair round resolved every
// marker; a marker left behind stops the group, and a later conflict returns as fixes.
func finishCatchUp(group, base string, declared []string) ([]string, error) {
	rebasing := false
	for _, dir := range []string{"rebase-merge", "rebase-apply"} {
		path, err := git.Run(group, "rev-parse", "--git-path", dir)
		if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(group, path)
		}
		if _, err := os.Stat(path); err == nil {
			rebasing = true
		}
	}
	_, mergeErr := git.Run(group, "rev-parse", "--verify", "--quiet", "MERGE_HEAD")
	merging := mergeErr == nil
	if !rebasing && !merging {
		return nil, nil
	}
	target := StartRef(group, base)
	marked, err := markedFiles(group, target)
	if err != nil {
		return nil, err
	}
	if len(marked) > 0 {
		return nil, fmt.Errorf("%w: %s", errConflictRemains, strings.Join(marked, ", "))
	}
	if err := stageWork(group, declared); err != nil {
		return nil, err
	}
	if merging {
		return settleCatchUp(group, target, []string{"merge", "--abort"}, "commit", "--no-edit")
	}
	return settleCatchUp(group, target, []string{"rebase", "--abort"}, "-c", "core.editor=true", "rebase", "--continue")
}

// markedFiles lists each file differing from target that still holds a conflict marker line.
func markedFiles(group, target string) ([]string, error) {
	changed, err := git.Run(group, "diff", "--name-only", target)
	if err != nil {
		return nil, err
	}
	var marked []string
	for _, file := range strings.Fields(changed) {
		data, err := os.ReadFile(filepath.Join(group, file))
		if err != nil {
			continue
		}
		for _, text := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(text, "<<<<<<< ") || strings.HasPrefix(text, ">>>>>>> ") {
				marked = append(marked, file)
				break
			}
		}
	}
	return marked, nil
}

// blockedLabel is the label a stopped group's draft pull request carries.
const blockedLabel = "status: blocked"

// ShipBlocked commits a stopped group's work as WIP, then its blocker note with open tasks BLOCKED, and publishes
// the branch as a draft PR labelled status: blocked; a scrubbed environment keeps both commits local.
func ShipBlocked(root string, plan *Plan, note backlog.BlockerNote, client *pr.Client) (*ShipResult, error) {
	worktree := WorktreePath(root, plan.Worktree)
	result := &ShipResult{Group: plan.Group, Branch: plan.Branch, Base: plan.Base, Draft: true}
	var declared []string
	for _, task := range plan.Tasks {
		declared = append(declared, task.Files...)
	}
	wip := fmt.Sprintf("wip: %s, blocked at %s (%s)", plan.Title, note.State, plan.Group)
	if err := commitStaged(worktree, declared, wip); err != nil {
		return nil, err
	}
	head, err := git.Run(worktree, "rev-parse", "--short", "HEAD")
	if err != nil {
		return nil, err
	}
	note.Saved = fmt.Sprintf("WIP commit `%s` on `%s`", head, plan.Branch)
	blocked, err := addBlockerNote(worktree, plan.Group, note)
	if err != nil {
		return nil, err
	}
	result.Blocked = blocked
	if err := commitStaged(worktree, nil, fmt.Sprintf("docs: %s is blocked (%s)", plan.Title, plan.Group)); err != nil {
		return nil, err
	}
	if scrubbed() {
		result.Warnings = append(result.Warnings, "no credential to publish; the blocker note is committed on "+plan.Branch)
		return result, nil
	}
	result.Base = liveBase(root, plan.Base)
	if err := PushFromWorktree(root, worktree, plan.Branch); err != nil {
		return result, err
	}
	if client == nil {
		return result, nil
	}
	title := fmt.Sprintf("%s: %s (%s), blocked", plan.Type, plan.Title, plan.Group)
	body := note.Render()
	url, err := client.Create(result.Base, plan.Branch, title, body, true)
	if err != nil {
		open, viewErr := client.View(plan.Branch)
		if viewErr != nil || open.State != "OPEN" {
			return result, err
		}
		if err := client.Edit(open.URL, "--title", title, "--body", body); err != nil {
			return result, err
		}
		url = open.URL
	}
	result.URL = url
	result.Labels, result.Warnings = labelBlocked(client, url)
	return result, nil
}

// writeCredentialNote commits a blocker note on the group's branch naming the refused push and komodo ship as the fix.
func writeCredentialNote(root string, plan *Plan, worktree string, cause error) error {
	head, err := git.Run(worktree, "rev-parse", "--short", "HEAD")
	if err != nil {
		return err
	}
	state, _ := RunFor(root, plan.Group)
	if _, err := addBlockerNote(worktree, plan.Group, backlog.BlockerNote{
		At: time.Now().UTC(), Run: state.Run, State: "Shipping", Items: []string{cause.Error()},
		Needs: "a valid forge credential, then `komodo ship " + plan.Group + "`",
		Saved: fmt.Sprintf("commit `%s` on `%s`", head, plan.Branch),
	}); err != nil {
		return err
	}
	return commitStaged(worktree, nil, fmt.Sprintf("docs: %s waits on a forge credential (%s)", plan.Title, plan.Group))
}

// addBlockerNote writes note into the group's own docs/backlog file, returning every task the note
// left BLOCKED.
func addBlockerNote(worktree, groupID string, note backlog.BlockerNote) ([]string, error) {
	path, text, found, err := backlog.FindGroupFile(worktree, groupID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("%s is not in %s", groupID, backlog.GroupFilesDir)
	}
	noted, err := backlog.AddGroupFileNote(text, note)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(noted), 0o644); err != nil {
		return nil, err
	}
	var blocked []string
	for _, task := range backlog.ParseGroupFile(noted).Tasks {
		if !task.Done {
			blocked = append(blocked, task.ID)
		}
	}
	return blocked, nil
}

// FinishShip publishes a group whose ship handed off for want of a credential: it drops the blocker note, pushes,
// opens the PR draft-first and labels it, runs after_publish scrubbed, stamps ship done and removes the handoff.
func FinishShip(root, groupID string, client *pr.Client) (*ShipResult, error) {
	path := HandoffPath(root, groupID)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s has no ship waiting on a credential", groupID)
	}
	if err != nil {
		return nil, err
	}
	var handoff ShipHandoff
	if err := json.Unmarshal(data, &handoff); err != nil {
		return nil, err
	}
	// An agent can write ship.json, so the push it names must be this group's plain branch.
	if handoff.Group != groupID || handoff.Branch == "" || strings.HasPrefix(handoff.Branch, "-") {
		return nil, fmt.Errorf("the handoff under %s names group %q and branch %q; nothing was pushed",
			groupID, handoff.Group, handoff.Branch)
	}
	if _, err := git.Run(root, "check-ref-format", "--branch", handoff.Branch); err != nil {
		return nil, fmt.Errorf("the handoff names %q, which is not a valid branch; nothing was pushed", handoff.Branch)
	}
	worktree := handoff.Worktree
	if worktree == "" {
		worktree = root
	}
	if err := dropCredentialNote(worktree, handoff); err != nil {
		return nil, err
	}
	if err := PushFromWorktree(root, worktree, handoff.Branch); err != nil {
		return nil, err
	}
	result := &ShipResult{Group: groupID, Branch: handoff.Branch, Base: handoff.Base, Draft: true}
	if handoff.AfterPublish != "" {
		published := RunCommandEnv(worktree, handoff.AfterPublish, Scrub(os.Environ()))
		result.Published = &published
		if !published.OK() {
			return result, fmt.Errorf("after_publish: %s", FailureText(published))
		}
	}
	if client != nil {
		url, draft, wip, warnings, err := createEpicPull(client, handoff.Base, handoff.Branch, handoff.Title, handoff.Body)
		if err != nil {
			open, viewErr := client.View(handoff.Branch)
			if viewErr != nil || open.State != "OPEN" {
				return result, err
			}
			url, draft = open.URL, open.Draft
		}
		kept, labelWarnings := ApplyLabelSet(client, url, handoff.Labels, OptionalLabels(root, handoff.Base))
		result.URL, result.Draft = url, draft
		result.Labels, result.Warnings = append(wip, kept...), append(warnings, labelWarnings...)
	}
	Stamp(root, ledger.Entry{Group: groupID, Station: "ship", Outcome: "done"})
	return result, os.Remove(path)
}

// dropCredentialNote removes the group's blocker note from its branch and commits that, when it holds one.
func dropCredentialNote(worktree string, handoff ShipHandoff) error {
	path, text, found, err := backlog.FindGroupFile(worktree, handoff.Group)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	out, removed := backlog.RemoveGroupFileNote(text)
	if !removed {
		return nil
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return err
	}
	return commitStaged(worktree, nil, fmt.Sprintf("docs: %s's forge credential is back", handoff.Group))
}

// commitStaged stages every change in worktree, declared files included, and commits them when any is staged.
func commitStaged(worktree string, declared []string, message string) error {
	if err := stageWork(worktree, declared); err != nil {
		return err
	}
	if staged, err := git.Run(worktree, "diff", "--cached", "--name-only"); err != nil || staged == "" {
		return err
	}
	_, err := git.Run(worktree, "commit", "-m", message)
	return err
}

// labelBlocked adds the status: blocked label the repo defines; a label whose name holds a space never
// matches KeepKnown's rule, so it is matched here by its whole name.
func labelBlocked(client *pr.Client, url string) (labels, warnings []string) {
	known, err := client.Labels()
	if err != nil {
		return nil, []string{fmt.Sprintf("could not list labels: %v", err)}
	}
	for _, label := range known {
		if label != blockedLabel && !strings.HasPrefix(label, blockedLabel+" ") {
			continue
		}
		if err := client.Label(url, []string{label}); err != nil {
			return nil, []string{fmt.Sprintf("could not add label(s): %v", err)}
		}
		return []string{label}, nil
	}
	return nil, []string{"the repo has no " + blockedLabel + " label"}
}

// ApplyLabels adds the repo labels matching wanted to the pull request, warning on each miss or failure.
func ApplyLabels(client *pr.Client, url string, wanted []string) (kept, warnings []string) {
	return ApplyLabelSet(client, url, wanted, nil)
}

// ApplyLabelSet adds the repo labels matching wanted and optional, warning only on a missing wanted one.
func ApplyLabelSet(client *pr.Client, url string, wanted, optional []string) (kept, warnings []string) {
	known, err := client.Labels()
	if err != nil {
		return nil, []string{fmt.Sprintf("could not list labels: %v", err)}
	}
	kept = pr.KeepKnown(append(append([]string(nil), wanted...), optional...), known)
	for _, label := range wanted {
		if !hasWanted(kept, label) {
			warnings = append(warnings, fmt.Sprintf("the repo has no %s label", label))
		}
	}
	if err := client.Label(url, kept); err != nil {
		warnings = append(warnings, fmt.Sprintf("could not add label(s): %v", err))
	}
	return kept, warnings
}

// hasWanted reports whether kept already carries the label wanted asked for, by its name before any space.
func hasWanted(kept []string, wanted string) bool {
	for _, label := range kept {
		name, _, _ := strings.Cut(label, " ")
		if name == wanted {
			return true
		}
	}
	return false
}

// liveBase is base while origin still has it, or the default branch once base is deleted.
func liveBase(root, base string) string {
	heads, err := git.Run(root, "ls-remote", "--heads", "origin", "refs/heads/"+base)
	if err != nil || heads != "" {
		return base
	}
	return DefaultBase(root)
}

// catchUp rebases the group branch onto the latest base, keeping uncommitted edits, so its pull request never
// starts behind; a conflict aborts the rebase and names the files.
func catchUp(group, base string) error {
	if hasOrigin(group) {
		// A base origin lacks has nothing newer to catch up to; the push reports an unreachable origin.
		_ = Fetch(group, base)
	}
	target := StartRef(group, base)
	if _, err := git.Run(group, "rev-parse", "--verify", "--quiet", target); err != nil {
		return nil
	}
	if _, err := git.Run(group, "merge-base", "--is-ancestor", target, "HEAD"); err == nil {
		return nil
	}
	// A pushed branch is never rewritten, so it takes the base in a merge commit instead.
	if branch, err := git.Run(group, "rev-parse", "--abbrev-ref", "HEAD"); err == nil && onOrigin(group, strings.TrimSpace(branch)) {
		if _, err := git.Run(group, "merge", "--autostash", "--no-edit", target); err != nil {
			conflicts, _ := git.Run(group, "diff", "--name-only", "--diff-filter=U")
			_, _ = git.Run(group, "merge", "--abort")
			return fmt.Errorf("%s moved and the group no longer merges it; resolve %s on the group branch, then ship",
				target, strings.Join(strings.Fields(conflicts), ", "))
		}
		return nil
	}
	if _, err := git.Run(group, "rebase", "--autostash", target); err != nil {
		conflicts, _ := git.Run(group, "diff", "--name-only", "--diff-filter=U")
		_, _ = git.Run(group, "rebase", "--abort")
		return fmt.Errorf("%s moved and the group no longer rebases onto it; resolve %s on the group branch, then ship",
			target, strings.Join(strings.Fields(conflicts), ", "))
	}
	return nil
}

var shortstatCount = regexp.MustCompile(`(\d+) (?:insertion|deletion)`)

// ChangedLines is the added plus deleted line count between base and branch, or zero when git cannot say.
func ChangedLines(dir, base, branch string) int {
	out, err := git.Run(dir, "diff", "--shortstat", base+"..."+branch)
	if err != nil {
		return 0
	}
	total := 0
	for _, match := range shortstatCount.FindAllStringSubmatch(out, -1) {
		var count int
		fmt.Sscan(match[1], &count)
		total += count
	}
	return total
}

// ReviewSize counts the files a diff keeps and the lines it adds; deletions and the line's bookkeeping count nothing.
func ReviewSize(dir, base, branch string) (files, added int) {
	out, err := git.Run(dir, "diff", "--numstat", "--diff-filter=d", base+"..."+branch)
	if err != nil {
		return 0, 0
	}
	for _, row := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(row)
		if len(fields) < 3 || bookkeeping(fields[len(fields)-1]) {
			continue
		}
		files++
		if count, err := strconv.Atoi(fields[0]); err == nil {
			added += count
		}
	}
	return files, added
}

// bookkeeping reports a path Ship writes itself: a group file, or a changelog fragment.
func bookkeeping(path string) bool {
	return strings.HasPrefix(path, "docs/backlog/") || strings.HasPrefix(path, "changelog.d/")
}

// checkPRSize refuses a diff over either the kept-file or the added-line ceiling, naming a split as the fix.
// A zero ceiling is unset and never refuses.
func checkPRSize(group string, files, lines, filesCap, linesMax int) error {
	var over []string
	if filesCap > 0 && files > filesCap {
		over = append(over, fmt.Sprintf("%d file(s) (cap %d)", files, filesCap))
	}
	if linesMax > 0 && lines > linesMax {
		over = append(over, fmt.Sprintf("%d added line(s) (cap %d)", lines, linesMax))
	}
	if len(over) == 0 {
		return nil
	}
	return fmt.Errorf("%s's diff has %s; split it into smaller groups before shipping a pull request",
		group, strings.Join(over, " and "))
}

// ChangelogLine is the one line a group adds under its version.
func ChangelogLine(plan *Plan, result *ShipResult) string {
	if plan.Version == "" {
		return ""
	}
	line := fmt.Sprintf("- **%s** %s (%d task(s))", plan.Group, plan.Title, len(result.Done))
	if len(result.Blocked) > 0 {
		line += fmt.Sprintf("; blocked: %s", strings.Join(result.Blocked, ", "))
	}
	return line
}

// PushFromWorktree pushes branch from worktree to the root's origin URL, past its refused pushurl, and sets its upstream.
// It refuses a critical ref, since the push carries the forge credential and landing is the human's merge.
func PushFromWorktree(root, worktree, branch string) error {
	if guard.Load(root, root).IsCritical(branch) {
		return fmt.Errorf("git push to origin %s: a critical ref; landing is the human's merge button", branch)
	}
	pushURL, err := git.Run(root, "remote", "get-url", "--push", "origin")
	if err != nil {
		return fmt.Errorf("git push to origin: the root names no origin: %w", err)
	}
	ref := "refs/heads/" + branch
	clean, username, password := splitCredential(pushURL)
	// The hook runs here without the credential, so the push that holds it skips the hook.
	if err := runPrePush(worktree, clean, ref); err != nil {
		return fmt.Errorf("pre-push hook for %s: %s", branch, redactURL(err.Error(), pushURL))
	}
	args := []string{"push", "--no-verify", clean, ref + ":" + ref}
	env := os.Environ()
	if username != "" || password != "" {
		args = append([]string{"-c", "credential.helper=", "-c", "credential.helper=" + pushCredentialHelper}, args...)
		env = append(env, pushUsernameEnv+"="+username, pushPasswordEnv+"="+password)
	}
	push := exec.Command("git", args...)
	push.Dir = worktree
	push.Env = env
	if out, err := push.CombinedOutput(); err != nil {
		failure := redactURL(fmt.Sprintf("%v: %s", err, strings.TrimSpace(string(out))), pushURL)
		if credentialRefused(string(out)) {
			return fmt.Errorf("git push to origin %s: %w: %s", branch, ErrNoCredential, failure)
		}
		return fmt.Errorf("git push to origin %s: %s", branch, failure)
	}
	// An upstream is a convenience for a person on the branch later; a push that landed never fails on it.
	if _, err := git.Run(worktree, "fetch", "origin", branch); err == nil {
		_, _ = git.Run(worktree, "branch", "--set-upstream-to=origin/"+branch, branch)
	}
	return nil
}

// ErrNoCredential marks a push the forge refused for a missing or expired credential.
var ErrNoCredential = errors.New("the forge credential is missing or expired")

// credentialRefusals are what git prints, lowercased, when a push holds no credential the forge accepts.
var credentialRefusals = []string{
	"authentication failed", "could not read username", "could not read password", "terminal prompts disabled",
	"permission denied (publickey)", "invalid username or password", "bad credentials",
	"returned error: 401", "returned error: 403",
}

// credentialRefused reports whether a push's output reads as a missing or expired credential.
func credentialRefused(output string) bool {
	lower := strings.ToLower(output)
	return slices.ContainsFunc(credentialRefusals, func(refusal string) bool { return strings.Contains(lower, refusal) })
}

// The variables that hand a push its credential, read only by pushCredentialHelper inside that one push.
const (
	pushUsernameEnv = "KOMODO_PUSH_USERNAME"
	pushPasswordEnv = "KOMODO_PUSH_PASSWORD"
)

// pushCredentialHelper answers git's credential get from the push's own environment, so no URL or argument holds it.
const pushCredentialHelper = `!f() { test "$1" = get || exit 0; echo "username=$` + pushUsernameEnv +
	`"; echo "password=$` + pushPasswordEnv + `"; }; f`

// splitCredential returns an http(s) URL without its user and secret, and those two apart; other URLs pass unchanged.
func splitCredential(raw string) (clean, username, password string) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return raw, "", ""
	}
	username = parsed.User.Username()
	password, _ = parsed.User.Password()
	parsed.User = nil
	return parsed.String(), username, password
}

// zeroSHA is the object name a pre-push hook reads for a remote ref it cannot see.
const zeroSHA = "0000000000000000000000000000000000000000"

// runPrePush runs worktree's pre-push hook, if any, on ref as a push to origin at url would, in hookEnv's environment.
func runPrePush(worktree, url, ref string) error {
	// A ref that does not resolve has nothing to gate; the push itself reports it.
	local, err := git.Run(worktree, "rev-parse", "--verify", "--quiet", ref)
	if err != nil {
		return nil
	}
	refs, err := os.CreateTemp("", "komodo-pre-push-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(refs.Name()) }()
	_, err = fmt.Fprintf(refs, "%s %s %s %s\n", ref, local, ref, zeroSHA)
	if closeErr := refs.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	cmd := exec.Command("git", "hook", "run", "--ignore-missing", "--to-stdin="+refs.Name(), "pre-push", "--", "origin", url)
	cmd.Dir = worktree
	cmd.Env = hookEnv(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, Clip(string(out), 4000, "pre-push"))
	}
	return nil
}

// forgeSecrets are the variables that carry a forge credential by name.
var forgeSecrets = []string{
	"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GIT_ASKPASS", "SSH_AUTH_SOCK", "GIT_CONFIG_PARAMETERS",
}

// hookEnv is env with every forge credential dropped, and git's credential helper and prompt switched off.
func hookEnv(env []string) []string {
	overrides := map[string]string{
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_CONFIG_COUNT":    "1",
		"GIT_CONFIG_KEY_0":    "credential.helper",
		"GIT_CONFIG_VALUE_0":  "",
		"GH_CONFIG_DIR":       filepath.Join(os.TempDir(), "komodo-gh-noauth"),
	}
	out := make([]string, 0, len(env)+len(overrides))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if _, set := overrides[key]; set || contains(forgeSecrets, key) {
			continue
		}
		out = append(out, entry)
	}
	for key, value := range overrides {
		out = append(out, key+"="+value)
	}
	return out
}

// credentialRe matches the user and secret a URL can carry before its host.
var credentialRe = regexp.MustCompile(`://[^/@\s]+@`)

// redactURL removes a push URL and any URL credentials from text, so a token never reaches an error.
func redactURL(text, url string) string {
	if url != "" {
		text = strings.ReplaceAll(text, url, "origin")
	}
	return credentialRe.ReplaceAllString(text, "://***@")
}

// refinedTasks returns task IDs whose body differs between root and group backlogs.
// It compares files, done_when, and context fields for each task in the plan.
func refinedTasks(root, group backlog.Backlog, planTasks []PlanTask) []string {
	var refined []string
	for _, pt := range planTasks {
		rootTask, ok := root.Task(pt.ID)
		if !ok {
			continue
		}
		groupTask, ok := group.Task(pt.ID)
		if !ok {
			continue
		}
		if !slicesEqual(rootTask.Files(), groupTask.Files()) ||
			!slicesEqual(rootTask.DoneWhen(), groupTask.DoneWhen()) ||
			!slicesEqual(rootTask.Context(), groupTask.Context()) {
			refined = append(refined, pt.ID)
		}
	}
	return refined
}

// slicesEqual reports whether two string slices are equal in order.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
