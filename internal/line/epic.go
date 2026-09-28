package line

import (
	"fmt"
	"strings"

	"komodo/internal/backlog"
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

// OpenEpic cuts the plan's epic branch from main, pushes it, and opens its draft pull request
// to main, the first time a group of that epic cuts; an existing branch is left alone.
func OpenEpic(root string, plan *Plan, client *pr.Client) (*EpicResult, error) {
	branch := EpicBranchName(plan.Version)
	if branch == "" {
		return nil, nil
	}
	parsed, _, err := LoadBacklog(root)
	if err != nil {
		return nil, err
	}
	group, ok := parsed.Group(plan.Group)
	if !ok || group.EpicID == "" {
		return nil, nil
	}
	epic, ok := parsed.Epic(group.EpicID)
	if !ok {
		return nil, nil
	}
	if onEpicOrigin(root, branch) {
		return nil, nil
	}
	if err := cutEpicBranch(root, branch); err != nil {
		return nil, err
	}
	result := &EpicResult{Branch: branch}
	if client == nil {
		return result, nil
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

// cutEpicBranch fetches main, points branch at it locally when branch does not exist yet, and
// pushes branch to origin.
func cutEpicBranch(root, branch string) error {
	if err := Fetch(root, "main"); err != nil {
		return err
	}
	if _, err := git.Run(root, "rev-parse", "--verify", "--quiet", branch); err != nil {
		if _, err := git.Run(root, "branch", branch, "origin/main"); err != nil {
			return err
		}
	}
	return PushFromWorktree(root, root, branch)
}

// createEpicPull opens head's pull request to base as a draft, or a normal one labelled
// status: wip where the forge refuses a draft.
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

// labelWip adds the status: wip label the repo already defines, since KeepKnown's rule of a
// label's name before its first space cannot match one whose own name holds a space.
func labelWip(client *pr.Client, url string) (labels, warnings []string) {
	known, err := client.Labels()
	if err != nil {
		return nil, []string{fmt.Sprintf("could not list labels: %v", err)}
	}
	label := wipLabel(known)
	if label == "" {
		return nil, []string{"the repo has no " + wipName + " label"}
	}
	if err := client.Label(url, []string{label}); err != nil {
		return nil, []string{fmt.Sprintf("could not add label(s): %v", err)}
	}
	return []string{label}, nil
}

// wipName is the label a PR carries in place of a draft the forge refused.
const wipName = "status: wip"

// wipLabel is the repo's status: wip label, matched by its whole name before any emoji, else empty.
func wipLabel(known []string) string {
	for _, label := range known {
		if label == wipName || strings.HasPrefix(label, wipName+" ") {
			return label
		}
	}
	return ""
}

// epicTitle is the epic's heading text with its goal line stripped.
func epicTitle(epic backlog.Epic) string {
	if idx := strings.Index(epic.Title, " *Goal:"); idx >= 0 {
		return epic.Title[:idx]
	}
	return epic.Title
}

// epicGoal is the sentence the epic's Goal line states, without its Ships as clause or markup.
func epicGoal(epic backlog.Epic) string {
	idx := strings.Index(epic.Title, "*Goal:")
	if idx < 0 {
		return ""
	}
	rest := epic.Title[idx+len("*Goal:"):]
	if i := strings.Index(rest, "Ships as `"); i >= 0 {
		rest = rest[:i]
	}
	return strings.TrimSpace(rest)
}
