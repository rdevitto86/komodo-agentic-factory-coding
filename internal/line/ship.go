package line

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/git"
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

// scrubbed reports whether the headless launcher stripped this process of every push credential.
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
	// This group's live status lands in BACKLOG.md once, in the ship commit.
	var shippedIDs []string
	for _, task := range plan.Tasks {
		shippedIDs = append(shippedIDs, task.ID)
		if status, ok := live[task.ID]; ok && status.Status != "" {
			if err := writeStatus(path, task.ID, status.Status); err != nil {
				return nil, err
			}
		}
	}
	if line := ChangelogLine(plan, result); line != "" {
		changelog := filepath.Join(group, "CHANGELOG.md")
		if err := AppendChangelog(changelog, plan.Version, line); err != nil {
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
	if err := pushFromWorktree(root, group, plan.Branch); err != nil {
		return nil, err
	}
	if command := AfterPublishCommand(root, group); command != "" {
		published := RunCommand(group, command)
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
		return result, err
	}
	result.URL = url
	result.Labels, result.Warnings = ApplyLabels(client, url, wanted)
	return result, nil
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

var versionHeading = regexp.MustCompile(`(?m)^## \[?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)`)

// AppendChangelog puts a line under the version's heading, creating the heading when it is new.
// The heading is matched with or without its date suffix, so a ship never duplicates its own.
func AppendChangelog(path, version, line string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := string(data)
	if strings.Contains(text, "\n"+line+"\n") {
		return nil
	}
	own := regexp.MustCompile(`(?m)^## ` + regexp.QuoteMeta(version) + `(?: .*)?\n`)
	if match := own.FindStringIndex(text); match != nil {
		return os.WriteFile(path, []byte(intoSection(text, match[1], line)), 0o644)
	}
	entry := fmt.Sprintf("## %s — %s\n\n%s\n", version, time.Now().UTC().Format("2006-01-02"), line)
	if match := versionHeading.FindStringIndex(text); match != nil {
		return os.WriteFile(path, []byte(text[:match[0]]+entry+"\n"+text[match[0]:]), 0o644)
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if text == "" {
		text = "# Changelog\n\n"
	}
	return os.WriteFile(path, []byte(text+"\n"+entry), 0o644)
}

var groupBullet = regexp.MustCompile(`(?m)^- \*\*(TG-[^*]+)\*\*.*$`)

// intoSection writes line into the version section starting at start: over the group's own
// bullet when one exists, else above the first group bullet, else right under the heading.
func intoSection(text string, start int, line string) string {
	end := len(text)
	if next := versionHeading.FindStringIndex(text[start:]); next != nil {
		end = start + next[0]
	}
	section := text[start:end]
	bullets := groupBullet.FindAllStringSubmatchIndex(section, -1)
	if own := groupBullet.FindStringSubmatch(line); own != nil {
		for _, bullet := range bullets {
			if section[bullet[2]:bullet[3]] == own[1] {
				return text[:start+bullet[0]] + line + text[start+bullet[1]:]
			}
		}
	}
	if len(bullets) > 0 {
		cut := start + bullets[0][0]
		return text[:cut] + line + "\n" + text[cut:]
	}
	return text[:start] + "\n" + line + "\n" + strings.TrimPrefix(text[start:], "\n")
}

// pushFromWorktree pushes branch to the root's origin URL, past the worktree's refused pushurl, then sets its upstream.
func pushFromWorktree(root, worktree, branch string) error {
	pushURL, err := git.Run(root, "remote", "get-url", "--push", "origin")
	if err != nil {
		return fmt.Errorf("git push to origin: the root names no origin: %w", err)
	}
	ref := "refs/heads/" + branch
	if _, err := git.Run(worktree, "push", pushURL, ref+":"+ref); err != nil {
		return fmt.Errorf("git push to origin %s: %s", branch, redactURL(err.Error(), pushURL))
	}
	// An upstream is a convenience for a person on the branch later; a push that landed never fails on it.
	if _, err := git.Run(worktree, "fetch", "origin", branch); err == nil {
		_, _ = git.Run(worktree, "branch", "--set-upstream-to=origin/"+branch, branch)
	}
	return nil
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
