package doctor

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"komodo/internal/backlog"
	"komodo/internal/git"
	"komodo/internal/pr"
)

// GitRunner runs git commands.
type GitRunner func(dir string, args ...string) (string, error)

// CheckEpics reports an epic holding READY groups whose branch or draft PR is missing on origin,
// and an open group PR whose base is main while its epic branch exists (decision 0028).
func CheckEpics(root string, defaultBranch string, run pr.Runner) []Problem {
	return checkEpicsWithRunners(root, defaultBranch, git.Run, run)
}

// checkEpicsWithRunners is the internal implementation accepting git and pr runners for testing.
func checkEpicsWithRunners(root string, defaultBranch string, gitRun GitRunner, prRun pr.Runner) []Problem {
	path := filepath.Join(root, "BACKLOG.md")
	parsed, err := backlog.Load(path)
	if err != nil {
		return nil
	}

	var problems []Problem

	// Build a map of epic ID to groups in that epic
	epicGroups := make(map[string][]backlog.Group)
	for _, group := range parsed.Groups {
		epicGroups[group.EpicID] = append(epicGroups[group.EpicID], group)
	}

	// Check each epic with READY groups
	for epicID, groups := range epicGroups {
		var readyGroups []backlog.Group
		for _, group := range groups {
			for _, task := range group.Tasks {
				if task.Ready() {
					readyGroups = append(readyGroups, group)
					break
				}
			}
		}

		if len(readyGroups) == 0 {
			continue
		}

		// Get the version from the first READY group in the epic
		version := readyGroups[0].Version()
		if version == "" {
			continue
		}

		epicBranch := "feat/" + version

		// Check if epic branch exists on origin
		out, _ := gitRun(root, "branch", "-r", "--list", "origin/"+epicBranch)
		epicBranchExists := strings.TrimSpace(out) != ""

		// Check if draft PR exists for the epic
		draftPRExists := hasDraftPRForBranchWithRunner(root, epicBranch, prRun)

		if !epicBranchExists && !draftPRExists {
			problems = append(problems, Problem{"epic", epicBranch,
				fmt.Sprintf("epic %s has READY groups but its branch and draft PR are missing on origin", epicID)})
		}
	}

	// Check for open group PRs with wrong base
	problems = append(problems, checkGroupPRBasesWithRunners(root, defaultBranch, parsed, gitRun, prRun)...)

	return problems
}

// hasDraftPRForBranch reports whether a draft PR exists for the given branch on the remote.
func hasDraftPRForBranch(root, branch string, run pr.Runner) bool {
	return hasDraftPRForBranchWithRunner(root, branch, run)
}

// hasDraftPRForBranchWithRunner reports whether a draft PR exists for the given branch on the remote.
func hasDraftPRForBranchWithRunner(root, branch string, run pr.Runner) bool {
	out, err := run(root, "pr", "list", "--head", branch, "--state", "open", "--json", "number,isDraft")
	if err != nil {
		return false
	}
	var prs []struct {
		Number int  `json:"number"`
		Draft  bool `json:"isDraft"`
	}
	if json.Unmarshal([]byte(out), &prs) != nil {
		return false
	}
	for _, p := range prs {
		if p.Draft {
			return true
		}
	}
	return false
}

// checkGroupPRBases reports group PRs whose base is main while their epic branch exists.
func checkGroupPRBases(root, defaultBranch string, parsed backlog.Backlog, run pr.Runner) []Problem {
	return checkGroupPRBasesWithRunners(root, defaultBranch, parsed, git.Run, run)
}

// checkGroupPRBasesWithRunners reports group PRs whose base is main while their epic branch exists.
func checkGroupPRBasesWithRunners(root, defaultBranch string, parsed backlog.Backlog, gitRun GitRunner, prRun pr.Runner) []Problem {
	out, err := prRun(root, "pr", "list", "--state", "open", "--json", "number,headRefName,baseRefName,title")
	if err != nil {
		return nil
	}
	var prs []struct {
		Number       int    `json:"number"`
		HeadRefName  string `json:"headRefName"`
		BaseRefName  string `json:"baseRefName"`
		Title        string `json:"title"`
	}
	if json.Unmarshal([]byte(out), &prs) != nil {
		return nil
	}

	// Build a map of branch to group
	branchToGroup := make(map[string]backlog.Group)
	for _, group := range parsed.Groups {
		branchToGroup[group.Branch()] = group
	}

	var problems []Problem
	for _, p := range prs {
		group, ok := branchToGroup[p.HeadRefName]
		if !ok {
			continue
		}

		// Check if this group belongs to an epic
		if group.EpicID == "" {
			continue
		}

		// Check if the base is main
		if p.BaseRefName != defaultBranch {
			continue
		}

		// Get the version to form the epic branch name
		version := group.Version()
		if version == "" {
			continue
		}

		epicBranch := "feat/" + version

		// Check if epic branch exists on origin
		out, _ := gitRun(root, "branch", "-r", "--list", "origin/"+epicBranch)
		if strings.TrimSpace(out) != "" {
			problems = append(problems, Problem{"epic", fmt.Sprintf("PR #%d", p.Number),
				fmt.Sprintf("group PR for %s targets %s but its epic branch %s exists on origin; target the epic branch instead (decision 0028)", group.ID, defaultBranch, epicBranch)})
		}
	}

	return problems
}
