package line

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/ledger"
)

const reportBacklog = "### [TG-09.1] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-09.1.1] Do it [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when:\n  - test -f a/one.go\n```\n"

// reportRepo builds a repo with a backlog whose one task is already DONE in the run's status.
func reportRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(reportBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RecordStatus(root, "TSK-09.1.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestReportSumsASessionsRecordedTokens proves REQ-28: a recorded stream's result totals equal the
// report's line for that session.
func TestReportSumsASessionsRecordedTokens(t *testing.T) {
	root := reportRepo(t)
	entry := ledger.Entry{
		Run: "run-1", Group: "TG-09.1", Station: "build",
		TokensIn: 420, TokensOut: 180, TokensCached: 50,
	}
	if err := Book(root).Stamp(entry); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{Group: "TG-09.1", Title: "A group", Tasks: []PlanTask{{ID: "TSK-09.1.1"}}}
	report, err := BuildReport(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	want := entry.TokensIn + entry.TokensOut
	if report.Tokens != want {
		t.Fatalf("tokens = %d, want %d (the session's own recorded totals)", report.Tokens, want)
	}
	line := fmt.Sprintf("%d token(s) per accepted group.", want)
	if !strings.Contains(report.Text, line) {
		t.Fatalf("report text = %q, want it to hold %q", report.Text, line)
	}
}

// TestReportOmitsTheHeadlineWhenNothingIsAccepted proves the headline never claims tokens for a
// group that has a blocked task, since only an accepted group earns the figure.
func TestReportOmitsTheHeadlineWhenNothingIsAccepted(t *testing.T) {
	root := reportRepo(t)
	if err := RecordStatus(root, "TSK-09.1.1", "BLOCKED"); err != nil {
		t.Fatal(err)
	}
	if err := Book(root).Stamp(ledger.Entry{Run: "run-1", Group: "TG-09.1", Station: "build", TokensIn: 100}); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{Group: "TG-09.1", Title: "A group", Tasks: []PlanTask{{ID: "TSK-09.1.1"}}}
	report, err := BuildReport(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	if report.Accepted {
		t.Fatal("a blocked task must never mark its group accepted")
	}
	if strings.Contains(report.Text, "token(s) per accepted group") {
		t.Fatalf("report text = %q, must not carry the headline with nothing accepted", report.Text)
	}
}

// TestReportExcludesBriefAndAdhocTokens proves a brief stamp's estimate and a group-tagged ad hoc
// entry never inflate the headline, since only a run's own session entries count.
func TestReportExcludesBriefAndAdhocTokens(t *testing.T) {
	root := reportRepo(t)
	session := ledger.Entry{Run: "run-1", Group: "TG-09.1", Station: "build", TokensIn: 420, TokensOut: 180}
	if err := Book(root).Stamp(session); err != nil {
		t.Fatal(err)
	}
	if err := Book(root).Stamp(ledger.Entry{Run: "run-1", Group: "TG-09.1", Station: "brief", TokensIn: 999}); err != nil {
		t.Fatal(err)
	}
	if err := Book(root).Stamp(ledger.Entry{Group: "TG-09.1", Station: "build", TokensIn: 999}); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{Group: "TG-09.1", Title: "A group", Tasks: []PlanTask{{ID: "TSK-09.1.1"}}}
	report, err := BuildReport(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	want := session.TokensIn + session.TokensOut
	if report.Tokens != want {
		t.Fatalf("tokens = %d, want %d (a brief stamp and an ad hoc entry must not count)", report.Tokens, want)
	}
}

const twoTaskBacklog = "### [TG-09.2] A group\n```yaml\ntype: feat\nversion: 2.0.0\n```\n\n" +
	"#### [TSK-09.2.1] Do it [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when:\n  - test -f a/one.go\n```\n\n" +
	"#### [TSK-09.2.2] Do it too [P: C] [READY]\n```yaml\nfiles: [a/two.go]\ndone_when:\n  - test -f a/two.go\n```\n"

// TestReportRequiresEveryTaskDoneToAccept proves a group with one Done and one still-READY
// task is never accepted, since accepting requires every plan task to be done.
func TestReportRequiresEveryTaskDoneToAccept(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(twoTaskBacklog), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RecordStatus(root, "TSK-09.2.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	if err := Book(root).Stamp(ledger.Entry{Run: "run-1", Group: "TG-09.2", Station: "build", TokensIn: 100}); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{
		Group: "TG-09.2", Title: "A group",
		Tasks: []PlanTask{{ID: "TSK-09.2.1"}, {ID: "TSK-09.2.2"}},
	}
	report, err := BuildReport(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	if report.Accepted {
		t.Fatal("one done task and one still-READY task must never mark the group accepted")
	}
	if strings.Contains(report.Text, "token(s) per accepted group") {
		t.Fatalf("report text = %q, must not carry the headline with an incomplete group", report.Text)
	}
}
