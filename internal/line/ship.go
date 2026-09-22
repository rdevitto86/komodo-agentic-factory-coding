package line

import (
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

// ShipGroup commits, pushes, opens the pull request, writes the changelog, and flips the statuses.
func ShipGroup(root string, plan *Plan, body string, client *pr.Client) (*ShipResult, error) {
	group := WorktreePath(root, plan.Worktree)
	path, err := backlog.Find(group)
	if err != nil {
		return nil, err
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return nil, err
	}
	blocking, minor := SplitFindings(ReviewFindings(root, plan.Group), plan.Profile.SeverityFloor)
	if len(blocking) > 0 {
		return nil, fmt.Errorf("the review left %d finding(s) at or above %s; fix them on %s, then ship",
			len(blocking), plan.Profile.SeverityFloor, plan.Branch)
	}
	filed, err := FileFindings(group, plan.Group, minor)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	result := &ShipResult{Group: plan.Group, Branch: plan.Branch, Base: plan.Base, Filed: filed}
	defer func() {
		Stamp(root, ledger.Entry{Group: plan.Group, Station: "ship", Seconds: Since(started), Outcome: "done"})
	}()
	for _, task := range plan.Tasks {
		current, ok := parsed.Task(task.ID)
		if !ok {
			continue
		}
		switch current.Status {
		case "BLOCKED":
			result.Blocked = append(result.Blocked, task.ID)
		default:
			result.Done = append(result.Done, task.ID)
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
	if _, err := git(group, "push", "-u", "origin", plan.Branch); err != nil {
		return nil, err
	}
	if command := AfterPublishCommand(group); command != "" {
		published := RunCommand(group, command)
		result.Published = &published
	}
	if client == nil {
		return result, nil
	}
	title := fmt.Sprintf("%s: %s (%s)", plan.Type, plan.Title, plan.Group)
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
func AppendChangelog(path, version, line string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := string(data)
	heading := "## " + version
	if index := strings.Index(text, heading+"\n"); index >= 0 {
		cut := index + len(heading) + 1
		return os.WriteFile(path, []byte(text[:cut]+"\n"+line+"\n"+strings.TrimPrefix(text[cut:], "\n")), 0o644)
	}
	entry := fmt.Sprintf("%s — %s\n\n%s\n", heading, time.Now().UTC().Format("2006-01-02"), line)
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
