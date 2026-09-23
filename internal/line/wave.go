package line

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"time"

	"komodo/internal/backlog"
	"komodo/internal/ledger"
)

// WaveResult is what QC decided about one wave.
type WaveResult struct {
	Wave     int             `json:"wave"`
	Merged   []string        `json:"merged"`
	Conflict string          `json:"conflict,omitempty"`
	Gates    []CommandResult `json:"gates,omitempty"`
	Verify   *CommandResult  `json:"verify,omitempty"`
	OK       bool            `json:"ok"`
}

// TaskBranch is the branch one task's worktree carries.
func TaskBranch(taskID string) string { return "task/" + strings.ToLower(taskID) }

// CloseWave merges a wave's worktrees into the group branch, then runs the gates and verify.
func CloseWave(root string, plan *Plan, index int) (*WaveResult, error) {
	if index < 0 || index >= len(plan.Waves) {
		return nil, fmt.Errorf("no wave %d in %s", index+1, plan.Group)
	}
	group := WorktreePath(root, plan.Worktree)
	started := time.Now()
	result := &WaveResult{Wave: index + 1}
	defer func() {
		entry := ledger.Entry{Group: plan.Group, Wave: result.Wave, Station: "qc", Seconds: Since(started)}
		if result.Conflict != "" {
			entry.FailureClass = "conflict"
		}
		if result.OK {
			entry.Outcome = "done"
		}
		Stamp(root, entry)
	}()
	if plan.Mode == "single" {
		// A single-mode group already committed every task on the group branch; nothing to merge.
		result.Merged = append(result.Merged, plan.Waves[index]...)
	} else {
		var previous string
		for _, taskID := range plan.Waves[index] {
			branch := TaskBranch(taskID)
			if _, err := git(group, "merge", "--no-ff", "-m", "merge "+taskID, branch); err != nil {
				result.Conflict = conflictMessage(previous, taskID, err)
				_, _ = git(group, "merge", "--abort")
				return result, nil
			}
			result.Merged = append(result.Merged, taskID)
			previous = taskID
		}
	}
	result.Gates = RunGate(group, CompileCommands(group))
	if _, failed := FirstFailure(result.Gates); failed {
		return result, nil
	}
	if command := VerifyCommand(group); command != "" {
		verify := RunCommand(group, command)
		result.Verify = &verify
		if !verify.OK() {
			return result, nil
		}
	}
	result.OK = true
	return result, nil
}

// conflictMessage names both tasks in a conflict, which is where a human takes over.
func conflictMessage(previous, taskID string, err error) string {
	if previous == "" {
		return fmt.Sprintf("%s conflicts with the group branch: %v", taskID, err)
	}
	return fmt.Sprintf("%s conflicts with %s; QC stops and a human resolves it", taskID, previous)
}

// Severities are the review severities, worst first.
var Severities = []string{"critical", "high", "medium", "low"}

// Finding is one review finding as the reviewer's schema describes it.
type Finding struct {
	Severity string `json:"severity"`
	Class    string `json:"class"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Fix      string `json:"fix"`
}

// AtOrAbove reports whether a severity is at or above the floor.
func AtOrAbove(severity, floor string) bool {
	rank := func(name string) int {
		for index, candidate := range Severities {
			if candidate == name {
				return index
			}
		}
		return len(Severities)
	}
	return rank(severity) <= rank(floor)
}

// ReviewFindings reads the findings the reviewer returned for a group, or none when it has no review.
func ReviewFindings(root, groupID string) []Finding {
	data, _, err := ReadResultFile(root, groupID+"-review")
	if err != nil {
		return nil
	}
	var result struct {
		Findings []Finding `json:"findings"`
	}
	if json.Unmarshal(data, &result) != nil {
		return nil
	}
	return result.Findings
}

// SplitFindings separates what becomes one repair brief from what is filed as a task.
func SplitFindings(findings []Finding, floor string) (repair, file []Finding) {
	for _, finding := range findings {
		if AtOrAbove(finding.Severity, floor) {
			repair = append(repair, finding)
			continue
		}
		file = append(file, finding)
	}
	return repair, file
}

// classType maps a finding class to the conventional-commit type its task carries.
var classType = map[string]string{
	"bug": "fix", "security": "fix", "simplify": "refactor",
	"narrative-comment": "docs", "undocumented-nonobvious": "docs", "test-gap": "test",
}

// FileFindings appends the findings under the floor as low-priority tasks, newest last.
func FileFindings(root, groupID string, findings []Finding) ([]string, error) {
	if len(findings) == 0 {
		return nil, nil
	}
	path, err := backlog.Find(root)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	var added []string
	for _, finding := range findings {
		taskType := classType[finding.Class]
		if taskType == "" {
			taskType = "chore"
		}
		var fields backlog.Fields
		fields.Set("files", []any{finding.File})
		fields.Set("done_when", []any{"test -f " + finding.File})
		fields.Set("type", taskType)
		fields.Set("context", []any{strings.TrimSpace(finding.Detail + " " + finding.Fix)})
		title := fmt.Sprintf("%s:%d %s", finding.File, finding.Line, finding.Title)
		next, id, err := backlog.AppendTask(text, groupID, title, fields, "L", "REFINEMENT")
		if err != nil {
			return added, err
		}
		text = next
		added = append(added, id)
	}
	return added, os.WriteFile(path, []byte(text), 0o644)
}
