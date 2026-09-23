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
	Branch string   `json:"branch"`
	Base   string   `json:"base"`
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels,omitempty"`
	Draft  bool     `json:"draft"`
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
	rootPath, err := backlog.Find(root)
	if err != nil {
		return nil, err
	}
	rootParsed, err := backlog.Load(rootPath)
	if err != nil {
		return nil, err
	}
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
			Branch: plan.Branch, Base: plan.Base, Title: title, Body: body,
			Labels: []string{plan.Type, "agent"}, Draft: result.Draft,
		}
		if err := writeShipHandoff(root, handoff); err != nil {
			return nil, err
		}
		return result, nil
	}
	if _, err := git(group, "push", "-u", "origin", plan.Branch); err != nil {
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

var versionHeading = regexp.MustCompile(`(?m)^## (\d+\.\d+\.\d+)`)

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
		cut := match[1]
		return os.WriteFile(path, []byte(text[:cut]+"\n"+line+"\n"+strings.TrimPrefix(text[cut:], "\n")), 0o644)
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
