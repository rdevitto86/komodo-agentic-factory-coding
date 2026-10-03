package line

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/backlog"
	"komodo/internal/backlog/backlogtest"
	"komodo/internal/ledger"
	"komodo/internal/mount"
)

var reportGroup = backlog.GroupFile{
	ID: "TG-09.1", Title: "A group", Priority: "C", Status: "READY", Type: "feat", Version: "2.0.0",
	Tasks: []backlog.GroupTask{
		{ID: "TSK-09.1.1", Title: "Do it", Files: []string{"a/one.go"}, Checks: []string{"test -f a/one.go"}},
	},
}

// reportRepo builds a repo with a backlog whose one task is already DONE in the run's status.
func reportRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	backlogtest.Seed(t, root, reportGroup)
	if err := RecordStatus(root, "TSK-09.1.1", "DONE"); err != nil {
		t.Fatal(err)
	}
	return root
}

// turnUsage parses one recorded stream line: an assistant turn's own token counts.
type turnUsage struct {
	Type  string `json:"type"`
	Usage struct {
		TokensIn  int `json:"input_tokens"`
		TokensOut int `json:"output_tokens"`
	} `json:"usage"`
}

// parseRecordedStream sums a recorded stream's assistant turns into one session's usage, the
// shape a mount's own Usage field returns.
func parseRecordedStream(path string) (mount.TaskUsage, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return mount.TaskUsage{}, false
	}
	var usage mount.TaskUsage
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var turn turnUsage
		if json.Unmarshal([]byte(line), &turn) != nil || turn.Type != "assistant" {
			continue
		}
		usage.TokensIn += turn.Usage.TokensIn
		usage.TokensOut += turn.Usage.TokensOut
	}
	if usage.TokensIn == 0 && usage.TokensOut == 0 {
		return mount.TaskUsage{}, false
	}
	return usage, true
}

// TestReportSumsASessionsRecordedTokens proves a recorded stream's result totals, parsed
// through the mount's own usage path, equal the report's line for that session.
func TestReportSumsASessionsRecordedTokens(t *testing.T) {
	root := reportRepo(t)
	stream := filepath.Join(root, "stream.jsonl")
	body := strings.Join([]string{
		`{"type":"user"}`,
		`{"type":"assistant","usage":{"input_tokens":420,"output_tokens":180}}`,
	}, "\n")
	if err := os.WriteFile(stream, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(snapshot) })
	mount.Register(mount.Host{
		Name: "fakehost-usage",
		Usage: func(string, string, time.Time, time.Time) (mount.TaskUsage, bool) {
			return parseRecordedStream(stream)
		},
	})
	host, ok := mount.Get("fakehost-usage")
	if !ok {
		t.Fatal("the fake host did not register")
	}
	usage, ok := host.Usage(root, "TSK-09.1.1", time.Time{}, time.Time{})
	if !ok {
		t.Fatal("the recorded stream was not parsed into usage")
	}
	entry := ledger.Entry{
		Run: "run-1", Group: "TG-09.1", Station: "build",
		TokensIn: usage.TokensIn, TokensOut: usage.TokensOut,
	}
	if err := Book(root).Stamp(entry); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{Group: "TG-09.1", Title: "A group", Tasks: []PlanTask{{ID: "TSK-09.1.1"}}}
	report, err := BuildReport(root, plan)
	if err != nil {
		t.Fatal(err)
	}
	want := usage.TokensIn + usage.TokensOut
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

var twoTaskGroup = backlog.GroupFile{
	ID: "TG-09.2", Title: "A group", Priority: "C", Status: "READY", Type: "feat", Version: "2.0.0",
	Tasks: []backlog.GroupTask{
		{ID: "TSK-09.2.1", Title: "Do it", Files: []string{"a/one.go"}, Checks: []string{"test -f a/one.go"}},
		{ID: "TSK-09.2.2", Title: "Do it too", Files: []string{"a/two.go"}, Checks: []string{"test -f a/two.go"}},
	},
}

// TestReportRequiresEveryTaskDoneToAccept proves a group with one Done and one still-READY
// task is never accepted, since accepting requires every plan task to be done.
func TestReportRequiresEveryTaskDoneToAccept(t *testing.T) {
	root := t.TempDir()
	backlogtest.Seed(t, root, twoTaskGroup)
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
