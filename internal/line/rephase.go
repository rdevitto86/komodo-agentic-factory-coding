package line

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/git"
	"komodo/internal/guard"
	"komodo/internal/pr"
)

// RephaseResult is what moving an open epic to a new version did.
type RephaseResult struct {
	Epic       string   `json:"epic"`
	OldVersion string   `json:"old_version"`
	NewVersion string   `json:"new_version"`
	OldBranch  string   `json:"old_branch"`
	NewBranch  string   `json:"new_branch"`
	Groups     []string `json:"groups"`
	GroupFiles []string `json:"group_files,omitempty"`
	Retargeted []string `json:"retargeted,omitempty"`
}

// Rephase moves an epic's groups to a new version, pushes the epic's branch under it, and
// retargets its open group pull requests; the caller asks before deleting the old branch.
func Rephase(root, epicID, newVersion string, client *pr.Client) (*RephaseResult, error) {
	parsed, path, err := LoadBacklog(root)
	if err != nil {
		return nil, err
	}
	epic, ok := parsed.Epic(epicID)
	if !ok {
		return nil, fmt.Errorf("epic %s not found", epicID)
	}
	oldVersion := epic.Version()
	var groups []string
	for _, group := range parsed.Groups {
		if group.EpicID != epicID {
			continue
		}
		groups = append(groups, group.ID)
		if oldVersion == "" {
			oldVersion = group.Version()
		}
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("epic %s has no open groups", epicID)
	}
	if oldVersion == "" {
		return nil, fmt.Errorf("epic %s carries no version to rephase", epicID)
	}
	if oldVersion == newVersion {
		return nil, fmt.Errorf("epic %s already ships %s", epicID, newVersion)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(rewriteVersions(string(data), epicID, oldVersion, newVersion)), 0o644); err != nil {
		return nil, err
	}
	groupFiles, err := rewriteGroupFileVersions(root, epicID, newVersion)
	if err != nil {
		return nil, err
	}

	oldBranch, newBranch := EpicBranchName(oldVersion), EpicBranchName(newVersion)
	if err := pushRephasedBranch(root, oldBranch, newBranch); err != nil {
		return nil, err
	}

	result := &RephaseResult{
		Epic: epicID, OldVersion: oldVersion, NewVersion: newVersion,
		OldBranch: oldBranch, NewBranch: newBranch, Groups: groups, GroupFiles: groupFiles,
	}
	if client == nil {
		return result, nil
	}
	retargeted, err := retargetPulls(client, oldBranch, newBranch)
	result.Retargeted = retargeted
	if err != nil {
		return result, err
	}
	return result, nil
}

// DeleteBranch removes a branch from origin, refusing a critical ref since only a person deletes one.
func DeleteBranch(root, branch string) error {
	if guard.Load(root, root).IsCritical(branch) {
		return fmt.Errorf("git push origin --delete %s: a critical ref; landing is the human's merge button", branch)
	}
	_, err := git.Run(root, "push", "origin", "--delete", branch)
	return err
}

// pushRephasedBranch points newBranch at old's tip on origin and pushes it, cutting a local branch
// only when none exists yet, so a retry never loses a branch already recut.
func pushRephasedBranch(root, oldBranch, newBranch string) error {
	if err := Fetch(root, oldBranch); err != nil {
		return err
	}
	if _, err := git.Run(root, "rev-parse", "--verify", "--quiet", newBranch); err != nil {
		if _, err := git.Run(root, "branch", newBranch, "origin/"+oldBranch); err != nil {
			return err
		}
	}
	return PushFromWorktree(root, root, newBranch)
}

// retargetPulls moves every open pull request based on oldBranch onto newBranch, returning each
// pull request's URL it moved.
func retargetPulls(client *pr.Client, oldBranch, newBranch string) ([]string, error) {
	out, err := runGH(client, "pr", "list", "--state", "open", "--base", oldBranch, "--json", "number,url")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return nil, err
	}
	var retargeted []string
	for _, row := range rows {
		if err := client.Edit(strconv.Itoa(row.Number), "--base", newBranch); err != nil {
			return retargeted, err
		}
		retargeted = append(retargeted, row.URL)
	}
	return retargeted, nil
}

// runGH runs one gh call through client's runner, falling back to the real gh binary the same way
// the client itself does, since its runner field is private to the pr package.
func runGH(client *pr.Client, args ...string) (string, error) {
	runner := client.Run
	if runner == nil {
		runner = pr.Run
	}
	return runner(client.Dir, args...)
}

// fenceOpen and fenceClose bound a group's fenced yaml block.
var (
	fenceOpen   = regexp.MustCompile("^```(?:yaml|yml)\\s*$")
	fenceClose  = regexp.MustCompile("^```\\s*$")
	versionLine = regexp.MustCompile(`^version:\s*\S*\s*$`)
)

// rewriteVersions rewrites BACKLOG.md's text: every group under epicID gets newVersion in its
// fenced block, and the epic's goal line's "Ships as `<version>`" text follows it.
func rewriteVersions(text, epicID, oldVersion, newVersion string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	parsed := backlog.Parse(text)
	for _, group := range parsed.Groups {
		if group.EpicID == epicID {
			setGroupVersion(lines, group.Heading, newVersion)
		}
	}
	out := strings.Join(lines, "\n")
	return rewriteEpicGoal(out, epicID, oldVersion, newVersion)
}

// setGroupVersion replaces the version line in the fenced block directly under heading, reporting
// whether it found one to replace.
func setGroupVersion(lines []string, heading int, newVersion string) bool {
	index := heading + 1
	for index < len(lines) && strings.TrimSpace(lines[index]) == "" {
		index++
	}
	if index >= len(lines) || !fenceOpen.MatchString(lines[index]) {
		return false
	}
	index++
	for index < len(lines) && !fenceClose.MatchString(lines[index]) {
		if versionLine.MatchString(lines[index]) {
			lines[index] = "version: " + newVersion
			return true
		}
		index++
	}
	return false
}

// rewriteEpicGoal replaces "Ships as `<oldVersion>`" with newVersion on the goal line just under
// epicID's heading, leaving the text alone when the phrase is not there.
func rewriteEpicGoal(text, epicID, oldVersion, newVersion string) string {
	if oldVersion == "" || oldVersion == newVersion {
		return text
	}
	heading := regexp.MustCompile(`^##\s+\[` + regexp.QuoteMeta(epicID) + `\]`)
	old, replacement := "Ships as `"+oldVersion+"`", "Ships as `"+newVersion+"`"
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if !heading.MatchString(line) {
			continue
		}
		for cursor := index + 1; cursor < len(lines) && cursor < index+5; cursor++ {
			if strings.HasPrefix(strings.TrimSpace(lines[cursor]), "#") {
				break
			}
			if strings.Contains(lines[cursor], old) {
				lines[cursor] = strings.ReplaceAll(lines[cursor], old, replacement)
				return strings.Join(lines, "\n")
			}
		}
		break
	}
	return text
}

// groupFileHeading matches a docs/backlog group file's own heading line.
var groupFileHeading = regexp.MustCompile(`^##\s+\[TG-`)

// rewriteGroupFileVersions rewrites newVersion into every docs/backlog group file whose epic
// field names epicID, returning the paths it changed; a repo with no docs/backlog changes nothing.
func rewriteGroupFileVersions(root, epicID, newVersion string) ([]string, error) {
	dir := filepath.Join(root, "docs", "backlog")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var touched []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return touched, err
		}
		text := string(data)
		if backlog.ParseGroupFile(text).EpicID != epicID {
			continue
		}
		lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
		heading := -1
		for index, line := range lines {
			if groupFileHeading.MatchString(line) {
				heading = index
				break
			}
		}
		if heading < 0 || !setGroupVersion(lines, heading, newVersion) {
			continue
		}
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			return touched, err
		}
		touched = append(touched, path)
	}
	return touched, nil
}
