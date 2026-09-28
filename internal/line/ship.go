package line

import (
	"encoding/json"
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
	path, err := backlog.Find(group)
	if err != nil {
		return nil, err
	}
	rootParsed, _, err := LoadBacklog(root)
	if err != nil {
		return nil, err
	}
	groupParsed, err := backlog.Load(path)
	if err != nil {
		return nil, err
	}
	if refined := refinedTasks(rootParsed, groupParsed, plan.Tasks); len(refined) > 0 {
		return nil, fmt.Errorf("the root's BACKLOG.md differs from this group's at %s; rebase or edit before ship",
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
	result.Draft = len(result.Blocked) > 0
	for _, taskID := range result.Done {
		if err := writeStatus(path, taskID, "DONE"); err != nil {
			return nil, err
		}
	}
	// This group's ticks and blockers land in BACKLOG.md once, in the ship commit; a status in flight never does.
	var shippedIDs []string
	for _, task := range plan.Tasks {
		shippedIDs = append(shippedIDs, task.ID)
		if status, ok := live[task.ID]; ok && backlogStatus(status.Status) {
			if err := writeStatus(path, task.ID, status.Status); err != nil {
				return nil, err
			}
		}
	}
	// A fragment per group, never an edit to CHANGELOG.md, so two open pull requests never conflict there.
	if line := ChangelogLine(plan, result); line != "" {
		if err := changelog.WriteFragment(group, plan.Version, plan.Group, line); err != nil {
			return nil, err
		}
		result.Changelog = line
	}
	var declared []string
	for _, task := range plan.Tasks {
		declared = append(declared, task.Files...)
	}
	if err := stageWork(group, declared); err != nil {
		return nil, err
	}
	// File findings into BACKLOG.md before push so they are in the ship commit.
	filed, err := FileFindings(group, plan.Group, minor)
	if err != nil {
		return result, err
	}
	result.Filed = filed
	// Stage BACKLOG.md with findings for commit.
	if err := stageWork(group, []string{"BACKLOG.md"}); err != nil {
		return nil, err
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
	if data, err := os.ReadFile(path); err == nil {
		context.Why = groupWhy(string(data), plan.Group)
	}
	context.BlastRadius, context.BlastRadiusWhy = reviewBlast(root, plan.Group)
	context.SizeNote = sizeNote(added, plan.Profile.PRLinesPreferred)
	body := ReportBody(plan, result, waves, context)
	wanted := []string{"@agent", scopeLabel(declared)}
	if scrubbed() {
		outcome = "handoff"
		handoff := ShipHandoff{
			Group: plan.Group, Worktree: group, Branch: plan.Branch, Base: result.Base, Title: title, Body: body,
			Labels: wanted, Draft: result.Draft,
			AfterPublish: AfterPublishCommand(root, group),
		}
		if err := writeShipHandoff(root, handoff); err != nil {
			return nil, err
		}
		return result, nil
	}
	if err := PushFromWorktree(root, group, plan.Branch); err != nil {
		return nil, err
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
	url, err := client.Create(result.Base, plan.Branch, title, body, result.Draft)
	if err != nil {
		// A re-ship after an escalation finds its pull request still open, and refreshes it instead.
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
	result.Labels, result.Warnings = ApplyLabels(client, url, wanted)
	return result, nil
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
	path, err := backlog.Find(worktree)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	noted, err := backlog.AddNote(string(data), plan.Group, note)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(noted), 0o644); err != nil {
		return nil, err
	}
	group, _ := backlog.Parse(noted).Group(plan.Group)
	for _, task := range group.Tasks {
		if task.Status == "BLOCKED" {
			result.Blocked = append(result.Blocked, task.ID)
		}
	}
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
	known, err := client.Labels()
	if err != nil {
		return nil, []string{fmt.Sprintf("could not list labels: %v", err)}
	}
	kept = pr.KeepKnown(wanted, known)
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

// scopeLabel is the one scope label a group's task files earn, its rules checked in order.
func scopeLabel(files []string) string {
	for _, f := range files {
		if strings.HasPrefix(f, "internal/guard/") {
			return "scope/guard"
		}
	}
	for _, f := range files {
		if strings.HasPrefix(f, "internal/mount/") {
			return "scope/mount"
		}
	}
	for _, f := range files {
		if strings.HasPrefix(f, "komodo/skills/") || strings.HasPrefix(f, "komodo/roles/") {
			return "scope/skills"
		}
	}
	for _, f := range files {
		if strings.HasPrefix(f, "internal/profile/") {
			base := strings.ToLower(filepath.Base(f))
			if strings.Contains(base, "tier") || strings.Contains(base, "machine") {
				return "scope/agents"
			}
		}
	}
	return "scope/harness"
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

// bookkeeping reports a path Ship writes itself: task status, a group file, or a changelog fragment.
func bookkeeping(path string) bool {
	return path == "BACKLOG.md" || strings.HasPrefix(path, "docs/backlog/") || strings.HasPrefix(path, "changelog.d/")
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
		failure := fmt.Sprintf("%v: %s", err, strings.TrimSpace(string(out)))
		return fmt.Errorf("git push to origin %s: %s", branch, redactURL(failure, pushURL))
	}
	// An upstream is a convenience for a person on the branch later; a push that landed never fails on it.
	if _, err := git.Run(worktree, "fetch", "origin", branch); err == nil {
		_, _ = git.Run(worktree, "branch", "--set-upstream-to=origin/"+branch, branch)
	}
	return nil
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
