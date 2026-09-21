package line

import (
	"fmt"
	"sort"
	"strings"

	"komodo/internal/backlog"
)

// Report is what one run did, in the accessibility contract.
type Report struct {
	Group   string   `json:"group"`
	Title   string   `json:"title"`
	Done    []string `json:"done"`
	Blocked []string `json:"blocked"`
	Repairs []string `json:"repairs"`
	Text    string   `json:"-"`
}

// BuildReport reads the run's results and blocks and renders the report a human reads.
func BuildReport(root string, plan *Plan) (*Report, error) {
	path, err := backlog.Find(root)
	if err != nil {
		return nil, err
	}
	parsed, err := backlog.Load(path)
	if err != nil {
		return nil, err
	}
	report := &Report{Group: plan.Group, Title: plan.Title}
	blockNotes := map[string]string{}
	for _, task := range plan.Tasks {
		current, ok := parsed.Task(task.ID)
		if !ok {
			continue
		}
		if attempt := LoadAttempt(root, task.ID); attempt.Count > 0 {
			report.Repairs = append(report.Repairs, task.ID)
			blockNotes[task.ID] = firstLine(attempt.Failure)
		}
		if current.Status == "BLOCKED" {
			report.Blocked = append(report.Blocked, task.ID)
			continue
		}
		if current.Status == "DONE" {
			report.Done = append(report.Done, task.ID)
		}
	}
	sort.Strings(report.Done)
	sort.Strings(report.Blocked)
	report.Text = renderReport(report, blockNotes)
	return report, nil
}

// renderReport writes the report under the headings the accessibility contract names.
func renderReport(report *Report, notes map[string]string) string {
	var out []string
	out = append(out, fmt.Sprintf("%s: %d done, %d blocked, %d repaired.",
		report.Group, len(report.Done), len(report.Blocked), len(report.Repairs)))
	if len(report.Done) > 0 {
		out = append(out, "", "## ✅ Successful Changes")
		for _, id := range report.Done {
			out = append(out, "- **"+id+"** closed on its own checks.")
		}
	}
	if len(report.Blocked) > 0 {
		out = append(out, "", "## ❌ Blocked Changes")
		for _, id := range report.Blocked {
			note := notes[id]
			if note == "" {
				note = "blocked with no note."
			}
			out = append(out, "- **"+id+"** "+note)
		}
	}
	if len(report.Repairs) > 0 {
		out = append(out, "", "## 📌 Callouts")
		out = append(out, fmt.Sprintf("- **%d task(s) needed a repair:** %s", len(report.Repairs), strings.Join(report.Repairs, ", ")))
	}
	return strings.Join(out, "\n") + "\n"
}

// firstLine is a failure's opening line, which is what a report shows.
func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return Clip(line, 160, "note")
}
