package line

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/ledger"
	"komodo/internal/pr"
)

// ShipResult is what the ship station did with one group.
type ShipResult struct {
	Group     string         `json:"group"`
	Branch    string         `json:"branch"`
	Base      string         `json:"base"`
	URL       string         `json:"url,omitempty"`
	Draft     bool           `json:"draft"`
	Labels    []string       `json:"labels,omitempty"`
	Changelog string         `json:"changelog,omitempty"`
	Done      []string       `json:"done,omitempty"`
	Blocked   []string       `json:"blocked,omitempty"`
	Filed     []string       `json:"filed,omitempty"`
	Published *CommandResult `json:"published,omitempty"`
}

// ShipHandoff is what a scrubbed ship leaves for a credentialed process to push and open.
type ShipHandoff struct {
	Group        string    `json:"group"`
	Worktree     string    `json:"worktree"`
	Branch       string    `json:"branch"`
	Base         string    `json:"base"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`
	Labels       []string  `json:"labels,omitempty"`
	Draft        bool      `json:"draft"`
	Minor        []Finding `json:"minor,omitempty"`
	AfterPublish string    `json:"after_publish,omitempty"`
}

// scrubbed reports whether the headless launcher stripped this process of every push credential.
func scrubbed() bool {
	return os.Getenv("GIT_TERMINAL_PROMPT") == "0" &&
		os.Getenv("GIT_CONFIG_KEY_0") == "credential.helper" &&
		os.Getenv("GIT_CONFIG_VALUE_0") == ""
}

// writeShipHandoff writes the branch, title, body, labels and draft flag a later push finishes.
func writeShipHandoff(root string, handoff ShipHandoff) error {
	dir := filepath.Join(root, StateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(handoff, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "ship.json"), append(data, '\n'), 0o644)
}

// ShipGroup commits, pushes, opens the pull request, writes the changelog, and flips the statuses.
// A scrubbed environment commits and hands the push off instead of failing on a credential it lacks.
func ShipGroup(root string, plan *Plan, waves []*WaveResult, client *pr.Client) (result *ShipResult, err error) {
	if plan.WaitUntil != "" {
		return nil, fmt.Errorf("%s is paused until %s; a plan whose waves a pause blanked cannot ship",
			plan.Group, plan.WaitUntil)
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
	live := LoadStatus(root)
	blocking, minor := SplitFindings(ReviewFindings(root, plan.Group), plan.Profile.SeverityFloor)
	if len(blocking) > 0 {
		return nil, fmt.Errorf("the review left %d finding(s) at or above %s; fix them on %s, then ship",
			len(blocking), plan.Profile.SeverityFloor, plan.Branch)
	}
	started := time.Now()
	result = &ShipResult{Group: plan.Group, Branch: plan.Branch, Base: plan.Base}
	outcome := ""
	defer func() {
		if outcome == "" {
			outcome = "done"
			if err != nil {
				outcome = "failed"
			}
		}
		Stamp(root, ledger.Entry{Group: plan.Group, Station: "ship", Seconds: Since(started), Outcome: outcome})
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
	// The run's live status lands in BACKLOG.md once, in the ship commit.
	for _, taskID := range sortedKeys(live) {
		if err := writeStatus(path, taskID, live[taskID].Status); err != nil {
			return nil, err
		}
	}
	if line := ChangelogLine(plan, result); line != "" {
		changelog := filepath.Join(group, "CHANGELOG.md")
		if err := AppendChangelog(changelog, plan.Version, line); err != nil {
			return nil, err
		}
		result.Changelog = line
	}
	if _, err := git(group, "add", "-A"); err != nil {
		return nil, err
	}
	if status, _ := git(group, "status", "--porcelain"); status != "" {
		message := fmt.Sprintf("%s: %s (%s)", plan.Type, plan.Title, plan.Group)
		if _, err := git(group, "commit", "-m", message); err != nil {
			return nil, err
		}
	}
	if err := ClearStatus(root); err != nil {
		return nil, err
	}
	if isToolkit(root) {
		if err := gateCommand(group); err != nil {
			return nil, fmt.Errorf("gate: %w", err)
		}
	}
	title := fmt.Sprintf("%s: %s (%s)", plan.Type, plan.Title, plan.Group)
	body := ReportBody(plan, result, waves)
	if scrubbed() {
		outcome = "handoff"
		handoff := ShipHandoff{
			Group: plan.Group, Worktree: group, Branch: plan.Branch, Base: plan.Base, Title: title, Body: body,
			Labels: []string{plan.Type, "agent"}, Draft: result.Draft,
			Minor: minor, AfterPublish: AfterPublishCommand(group),
		}
		if err := writeShipHandoff(root, handoff); err != nil {
			return nil, err
		}
		return result, nil
	}
	if err := pushFromWorktree(root, group, plan.Branch); err != nil {
		return nil, err
	}
	filed, err := FileFindings(group, plan.Group, minor)
	if err != nil {
		return result, err
	}
	result.Filed = filed
	if command := AfterPublishCommand(group); command != "" {
		published := RunCommand(group, command)
		result.Published = &published
		if !published.OK() {
			return result, fmt.Errorf("after_publish: %s", FailureText(published))
		}
	}
	if client == nil {
		return result, nil
	}
	url, err := client.Create(plan.Base, plan.Branch, title, body, result.Draft)
	if err != nil {
		return result, err
	}
	result.URL = url
	if known, err := client.Labels(); err == nil {
		result.Labels = pr.KeepKnown([]string{plan.Type, "agent"}, known)
		_ = client.Label(url, result.Labels)
	}
	return result, nil
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

// ReportBody renders the pull request body from the plan and what shipped.
func ReportBody(plan *Plan, result *ShipResult, waves []*WaveResult) string {
	var out []string
	out = append(out, fmt.Sprintf("%s: %s", plan.Group, plan.Title))
	out = append(out, "")
	out = append(out, "## What landed")
	for _, task := range plan.Tasks {
		mark := "x"
		if contains(result.Blocked, task.ID) {
			mark = " "
		}
		out = append(out, fmt.Sprintf("- [%s] **%s** %s", mark, task.ID, task.Title))
	}
	if len(waves) > 0 {
		out = append(out, "", "## QC")
		for _, wave := range waves {
			line := fmt.Sprintf("- **Wave %d** merged %s", wave.Wave, strings.Join(wave.Merged, ", "))
			if wave.Conflict != "" {
				line += "; conflict: " + wave.Conflict
			}
			if wave.Verify != nil {
				line += fmt.Sprintf("; verify exited %d", wave.Verify.ExitCode)
			}
			out = append(out, line)
		}
	}
	if len(result.Blocked) > 0 {
		out = append(out, "", "## Blocked", "- "+strings.Join(result.Blocked, ", "))
	}
	return strings.Join(out, "\n") + "\n"
}

// pushFromWorktree pushes branch to the root's origin URL, past the worktree's refused pushurl, then sets its upstream.
func pushFromWorktree(root, worktree, branch string) error {
	pushURL, err := git(root, "remote", "get-url", "--push", "origin")
	if err != nil {
		return fmt.Errorf("git push to origin: the root names no origin: %w", err)
	}
	ref := "refs/heads/" + branch
	cmd := exec.Command("git", "push", pushURL, ref+":"+ref)
	cmd.Dir = worktree
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git push to origin %s: %v: %s", branch, err, redactURL(strings.TrimSpace(stderr.String()), pushURL))
	}
	// An upstream is a convenience for a person on the branch later; a push that landed never fails on it.
	if _, err := git(worktree, "fetch", "origin", branch); err == nil {
		_, _ = git(worktree, "branch", "--set-upstream-to=origin/"+branch, branch)
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
