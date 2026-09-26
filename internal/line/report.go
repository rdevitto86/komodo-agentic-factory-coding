package line

import (
	"fmt"
	"sort"
	"strings"

	"komodo/internal/ledger"
)

// Report is what one run did, in the accessibility contract.
type Report struct {
	Group    string   `json:"group"`
	Title    string   `json:"title"`
	Done     []string `json:"done"`
	Blocked  []string `json:"blocked"`
	Repairs  []string `json:"repairs"`
	Accepted bool     `json:"accepted"`
	Tokens   int      `json:"tokens"`
	Text     string   `json:"-"`
}

// BuildReport reads the run's results and blocks and renders the report a human reads.
func BuildReport(root string, plan *Plan) (*Report, error) {
	parsed, _, err := LoadBacklog(root)
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
	report.Accepted = len(report.Blocked) == 0 && len(report.Done) == len(plan.Tasks)
	tokens, err := groupTokens(root, plan.Group)
	if err != nil {
		return nil, err
	}
	report.Tokens = tokens
	report.Text = renderReport(report, blockNotes)
	return report, nil
}

// groupTokens sums a group's input and output tokens across the run's own build and repair
// sessions, excluding brief stamps and ad hoc entries from off-run attempts.
func groupTokens(root, group string) (int, error) {
	entries, err := Book(root).Read(ledger.RunFile)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, entry := range entries {
		if entry.Group == group && entry.Run != "" && entry.Station != "brief" {
			total += entry.TokensIn + entry.TokensOut
		}
	}
	return total, nil
}

// renderReport writes the report under the headings the accessibility contract names.
func renderReport(report *Report, notes map[string]string) string {
	var out []string
	out = append(out, fmt.Sprintf("%s: %d done, %d blocked, %d repaired.",
		report.Group, len(report.Done), len(report.Blocked), len(report.Repairs)))
	if report.Accepted {
		out = append(out, fmt.Sprintf("%d token(s) per accepted group.", report.Tokens))
	}
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
