package line

import (
	"context"
	"fmt"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/changelog"
	"komodo/internal/git"
	"komodo/internal/pr"
)

// EpicResult is what opening an epic's branch and draft pull request did.
type EpicResult struct {
	Branch   string   `json:"branch"`
	URL      string   `json:"url,omitempty"`
	Draft    bool     `json:"draft"`
	Labels   []string `json:"labels,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// EpicBranchName is feat/ plus a version exactly, the branch its epic ships on.
func EpicBranchName(version string) string {
	if version == "" {
		return ""
	}
	return "feat/" + version
}

// OpenEpicBranch is the branch of the newest epic on disk whose branch origin holds, else empty.
func OpenEpicBranch(root string) string {
	parsed, err := backlog.LoadRoot(root)
	if err != nil {
		return ""
	}
	newest := ""
	for _, epic := range parsed.Epics {
		version := epic.Version()
		if version == "" || (newest != "" && changelog.Compare(version, newest) <= 0) {
			continue
		}
		if onEpicOrigin(root, EpicBranchName(version)) {
			newest = version
		}
	}
	return EpicBranchName(newest)
}

// OpenEpic cuts the plan's epic branch from main when origin lacks it, and opens its draft pull
// request to main when none is open; an open pull request is left alone.
func OpenEpic(root string, plan *Plan, client *pr.Client) (*EpicResult, error) {
	parsed, _, err := LoadBacklog(root)
	if err != nil {
		return nil, err
	}
	return openEpic(root, parsed, plan, client)
}

// openEpic is OpenEpic's own work once its backlog is parsed, so a caller already holding one
// need not have it read from root again.
func openEpic(root string, parsed backlog.Backlog, plan *Plan, client *pr.Client) (*EpicResult, error) {
	branch := EpicBranchName(plan.Version)
	if branch == "" {
		return nil, nil
	}
	group, ok := parsed.Group(plan.Group)
	if !ok || group.EpicID == "" {
		return nil, nil
	}
	epic, ok := parsed.Epic(group.EpicID)
	if !ok {
		return nil, nil
	}
	result := &EpicResult{Branch: branch}
	if onEpicOrigin(root, branch) {
		if client == nil {
			return nil, nil
		}
		open, err := client.OpenHead(branch, "main")
		if err != nil {
			result.Warnings = []string{fmt.Sprintf("could not list %s's pull requests: %v", branch, err)}
			return result, nil
		}
		if open {
			return nil, nil
		}
	} else {
		if err := cutEpicBranch(root, branch); err != nil {
			return nil, err
		}
		if client == nil {
			return result, nil
		}
	}
	title := fmt.Sprintf("feat: %s (%s)", epicTitle(epic), plan.Version)
	url, draft, labels, warnings, err := createEpicPull(client, "main", branch, title, epicGoal(epic))
	if err != nil {
		return result, err
	}
	result.URL, result.Draft, result.Labels, result.Warnings = url, draft, labels, warnings
	return result, nil
}

// onEpicOrigin reports whether origin already holds branch, checked live rather than from a
// stale fetch, since an epic branch may be cut by a run this repo copy never fetched.
func onEpicOrigin(root, branch string) bool {
	heads, err := git.Run(root, "ls-remote", "--heads", "origin", "refs/heads/"+branch)
	return err == nil && heads != ""
}

// cutEpicBranch fetches main and pushes origin/main to origin as branch, with no local branch cut.
func cutEpicBranch(root, branch string) error {
	if err := Fetch(root, "main"); err != nil {
		return err
	}
	return pushRef(context.Background(), root, root, "origin/main", branch)
}

// ReadyEpic marks the epic pull request at url ready, refusing while its diff adds a backlog file
// or the changelog names no heading for its version.
func ReadyEpic(root, version string, client *pr.Client, url string, draft bool) error {
	if err := epicReadyProblem(root, version); err != nil {
		return err
	}
	return markReady(client, url, draft)
}

// epicReadyProblem names, in one error, the task files the epic branch adds over main and a
// missing heading for version in that branch's change log.
func epicReadyProblem(root, version string) error {
	branch := EpicBranchName(version)
	head, base := StartRef(root, branch), StartRef(root, "main")
	added, err := git.Run(root, "diff", "--name-only", "--diff-filter=A", base+"..."+head, "--", backlog.GroupFilesDir)
	if err != nil {
		return err
	}
	var problems []string
	if files := strings.Fields(added); len(files) > 0 {
		problems = append(problems, "its diff adds "+strings.Join(files, ", "))
	}
	text, _ := git.Run(root, "show", head+":"+changelog.File)
	if !namesVersion(text, version) {
		problems = append(problems, fmt.Sprintf("%s has no heading for %s", changelog.File, version))
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s stays a draft: %s", branch, strings.Join(problems, "; "))
}

// namesVersion reports whether a changelog holds a heading for exactly version.
func namesVersion(text, version string) bool {
	for _, match := range changelog.Heading.FindAllStringSubmatch(text, -1) {
		if match[1] == version {
			return true
		}
	}
	return false
}

// createEpicPull opens head's pull request to base as a draft, or a normal one labelled
// status/wip where the forge refuses a draft.
func createEpicPull(client *pr.Client, base, head, title, body string) (url string, draft bool, labels, warnings []string, err error) {
	url, err = client.Create(base, head, title, body, true)
	if err == nil {
		return url, true, nil, nil, nil
	}
	draftErr := err
	url, err = client.Create(base, head, title, body, false)
	if err != nil {
		return "", false, nil, nil, draftErr
	}
	labels, warnings = labelWip(client, url)
	return url, false, labels, warnings, nil
}

// labelWip adds the status/wip label the repo already defines.
func labelWip(client *pr.Client, url string) (labels, warnings []string) {
	return labelNamed(client, url, wipName)
}

// labelNamed adds the repo's label called name, since KeepKnown's rule of a label's name before
// its first space cannot match one whose own name holds a space.
func labelNamed(client *pr.Client, url, name string) (labels, warnings []string) {
	known, err := client.Labels()
	if err != nil {
		return nil, []string{fmt.Sprintf("could not list labels: %v", err)}
	}
	label := knownLabel(known, name)
	if label == "" {
		return nil, []string{"the repo has no " + name + " label"}
	}
	if err := client.Label(url, []string{label}); err != nil {
		return nil, []string{fmt.Sprintf("could not add label(s): %v", err)}
	}
	return []string{label}, nil
}

// wipName is the label a PR carries in place of a draft the forge refused.
const wipName = "status/wip"

// wipLabel is the repo's status/wip label, matched by its whole name before any emoji, else empty.
func wipLabel(known []string) string {
	return knownLabel(known, wipName)
}

// knownLabel is the repo's label called name, matched by its whole name before any emoji, else empty.
func knownLabel(known []string, name string) string {
	for _, label := range known {
		if label == name || strings.HasPrefix(label, name+" ") {
			return label
		}
	}
	return ""
}

// epicTitle is the epic's the epic index file title.
func epicTitle(epic backlog.Epic) string {
	return strings.TrimSpace(epic.Title)
}

// epicGoal is the epic's goal paragraph, its title when the epic index file states none.
func epicGoal(epic backlog.Epic) string {
	if goal := strings.TrimSpace(epic.Goal); goal != "" {
		return goal
	}
	return epicTitle(epic)
}
