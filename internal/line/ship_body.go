package line

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BodyContext is what the pull request body reads beyond the plan: why, section order, review, and default branch.
type BodyContext struct {
	Why            string
	Sections       []string
	DefaultBase    string
	BlastRadius    string
	BlastRadiusWhy string
}

// defaultSections are the pull request body's sections when the repo has no template.
var defaultSections = []string{"Summary", "Changes", "Validation", "Dependencies"}

// templatePath is where a repo keeps the pull request template whose headings order the body.
const templatePath = ".github/PULL_REQUEST_TEMPLATE.md"

// templateSections reads the template's ## headings among the known sections, then any known one it left out.
func templateSections(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, templatePath))
	if err != nil {
		return defaultSections
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		heading, ok := strings.CutPrefix(strings.TrimSpace(line), "## ")
		if !ok {
			continue
		}
		for _, known := range defaultSections {
			if strings.EqualFold(strings.TrimSpace(heading), known) && !contains(out, known) {
				out = append(out, known)
			}
		}
	}
	for _, known := range defaultSections {
		if !contains(out, known) {
			out = append(out, known)
		}
	}
	return out
}

// groupWhy returns the reason line under a group's heading, or the empty string.
func groupWhy(text, groupID string) string {
	inGroup := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "### ") {
			inGroup = strings.HasPrefix(line, "### ["+groupID+"]")
			continue
		}
		if why, ok := strings.CutPrefix(strings.TrimSpace(line), "* **Why:**"); ok && inGroup {
			return strings.TrimSpace(why)
		}
	}
	return ""
}

// reviewBlast reads the review's blast radius tier and its reason, or empty strings.
func reviewBlast(root, groupID string) (string, string) {
	data, _, err := ReadResultFile(root, groupID+"-review")
	if err != nil {
		return "", ""
	}
	var review struct {
		BlastRadius    string `json:"blast_radius"`
		BlastRadiusWhy string `json:"blast_radius_why"`
	}
	if json.Unmarshal(data, &review) != nil {
		return "", ""
	}
	return review.BlastRadius, review.BlastRadiusWhy
}

// commandOutcome names a command's result for the Validation section.
func commandOutcome(result CommandResult) string {
	if result.OK() {
		return fmt.Sprintf("- `%s` passed", result.Command)
	}
	return fmt.Sprintf("- `%s` exited %d", result.Command, result.ExitCode)
}

// ReportBody renders the pull request body in the template's sections from the plan and what shipped.
func ReportBody(plan *Plan, result *ShipResult, waves []*WaveResult, context BodyContext) string {
	sections := context.Sections
	if len(sections) == 0 {
		sections = defaultSections
	}
	var out []string
	for _, section := range sections {
		var lines []string
		switch section {
		case "Summary":
			lines = summaryLines(plan, result, context)
		case "Changes":
			lines = changeLines(plan)
		case "Validation":
			lines = validationLines(plan, result, waves, context)
		case "Dependencies":
			lines = dependencyLines(result, context)
		}
		if len(lines) == 0 {
			continue
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, "## "+section, "")
		out = append(out, lines...)
	}
	return strings.Join(out, "\n") + "\n"
}

// summaryLines are the group's Why line, or its title, and any switch away from a deleted base.
func summaryLines(plan *Plan, result *ShipResult, context BodyContext) []string {
	summary := context.Why
	if summary == "" {
		summary = fmt.Sprintf("%s: %s.", plan.Group, plan.Title)
	}
	lines := []string{summary}
	if result.StaleBase != "" {
		lines = append(lines, "", fmt.Sprintf("Base %s is gone from origin, so this targets %s.", result.StaleBase, result.Base))
	}
	return lines
}

// changeLines are one bullet per task: its id, title, and the files it declared.
func changeLines(plan *Plan) []string {
	var lines []string
	for _, task := range plan.Tasks {
		line := fmt.Sprintf("- **%s** — %s", task.ID, task.Title)
		if len(task.Files) > 0 {
			line += " (`" + strings.Join(task.Files, "`, `") + "`)"
		}
		lines = append(lines, line)
	}
	return lines
}

// validationLines are each wave's gates and verify with outcomes, the blast radius, and every unproven step.
func validationLines(plan *Plan, result *ShipResult, waves []*WaveResult, context BodyContext) []string {
	var lines []string
	ran := false
	for _, wave := range waves {
		if wave.Conflict != "" {
			lines = append(lines, fmt.Sprintf("- **Wave %d** conflict: %s", wave.Wave, wave.Conflict))
		}
		for _, gate := range wave.Gates {
			lines, ran = append(lines, commandOutcome(gate)), true
		}
		if wave.Verify != nil {
			lines, ran = append(lines, commandOutcome(*wave.Verify)), true
		}
	}
	if context.BlastRadius != "" {
		line := "- **Blast radius** " + context.BlastRadius
		if context.BlastRadiusWhy != "" {
			line += ": " + context.BlastRadiusWhy
		}
		lines = append(lines, line)
	}
	if !ran {
		lines = append(lines, "- **Unproven** no QC gate or verify command ran")
	}
	for _, task := range plan.Tasks {
		if contains(result.Blocked, task.ID) {
			lines = append(lines, fmt.Sprintf("- **Unproven** %s %s is blocked and did not ship", task.ID, task.Title))
		}
	}
	return lines
}

// dependencyLines name the base branch when the pull request stacks on one other than the default.
func dependencyLines(result *ShipResult, context BodyContext) []string {
	if result.Base == "" || context.DefaultBase == "" || result.Base == context.DefaultBase {
		return nil
	}
	return []string{fmt.Sprintf("1. Stacks on `%s`; land it first.", result.Base)}
}
