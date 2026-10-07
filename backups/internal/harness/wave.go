package harness

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

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

// RecordedWaves rebuilds the group's wave results from the QC rows its current run stamped, the last row per wave.
func RecordedWaves(root, group string) []*WaveResult {
	state, err := LoadRunFor(root, group)
	if err != nil || state.Run == "" {
		return nil
	}
	entries, err := Book(root).Read(ledger.RunFile)
	if err != nil {
		return nil
	}
	byWave := map[int]*WaveResult{}
	var order []int
	for _, entry := range entries {
		if entry.Station != "qc" || entry.Group != group || entry.Run != state.Run || entry.Wave == 0 {
			continue
		}
		if _, seen := byWave[entry.Wave]; !seen {
			order = append(order, entry.Wave)
		}
		wave := &WaveResult{Wave: entry.Wave, OK: entry.Outcome == "done"}
		for _, check := range entry.Checks {
			ran := CommandResult{Command: check.Command, ExitCode: check.ExitCode, Seconds: check.Seconds}
			if check.Kind == "verify" {
				wave.Verify = &ran
			} else {
				wave.Gates = append(wave.Gates, ran)
			}
		}
		byWave[entry.Wave] = wave
	}
	sort.Ints(order)
	out := make([]*WaveResult, 0, len(order))
	for _, number := range order {
		out = append(out, byWave[number])
	}
	return out
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

// FileFindings files each finding under the floor as a READY task file in the group's folder, by severity.
func FileFindings(root, groupID string, findings []Finding) ([]string, error) {
	if len(findings) == 0 {
		return nil, nil
	}
	dir, found, err := backlog.Locate(root, groupID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("%s is not in %s", groupID, backlog.GroupFilesDir)
	}
	return fileGroupFileFindings(root, dir, findings)
}

// findingPriority is the task priority each review severity files at.
var findingPriority = map[string]string{"critical": "C", "high": "H", "medium": "M", "low": "L"}

// findingProof is the command that proves a filed finding fixed: its Go package's tests, else the
// repo's verify command, else komodo lint.
func findingProof(root, file string) string {
	if strings.HasSuffix(file, ".go") {
		if dir := filepath.ToSlash(filepath.Dir(file)); dir != "." {
			return "go test ./" + dir + "/..."
		}
		return "go test ."
	}
	if verify := VerifyCommand(root, root); verify != "" {
		return verify
	}
	return "komodo lint"
}

// fileGroupFileFindings writes every unfiled finding as its own READY task file with its file and proof.
func fileGroupFileFindings(root string, dir backlog.GroupDir, findings []Finding) ([]string, error) {
	titles := map[string]bool{}
	for _, task := range dir.File.Tasks {
		titles[task.Title] = true
	}
	var added []string
	for _, finding := range findings {
		title := fmt.Sprintf("%s:%d %s", finding.File, finding.Line, finding.Title)
		if titles[title] {
			continue
		}
		priority := findingPriority[finding.Severity]
		if priority == "" {
			priority = "L"
		}
		task := backlog.GroupTask{
			Title: title, Files: []string{finding.File},
			Checks:   []string{findingProof(root, finding.File)},
			Context:  []string{strings.TrimSpace(finding.Detail + " " + finding.Fix)},
			Priority: priority, Status: "READY",
		}
		id, _, err := dir.AppendTask(task)
		if err != nil {
			return added, err
		}
		task.ID = id
		dir.File.Tasks = append(dir.File.Tasks, task)
		added = append(added, id)
		titles[title] = true
	}
	return added, nil
}
