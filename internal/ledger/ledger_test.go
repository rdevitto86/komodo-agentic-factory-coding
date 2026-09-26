package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStampWritesToTheRunFile(t *testing.T) {
	book := New(t.TempDir())
	if err := book.Stamp(Entry{Run: "r1", Station: "brief", Task: "TSK-01.1.1", Seconds: 2}); err != nil {
		t.Fatal(err)
	}
	entries, err := book.Read(RunFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Station != "brief" || entries[0].At.IsZero() {
		t.Fatalf("entries = %+v", entries)
	}
	if adhoc, _ := book.Read(AdhocFile); len(adhoc) != 0 {
		t.Fatal("a run entry landed in the ad hoc file")
	}
}

func TestStampWithoutARunGoesToTheAdhocFile(t *testing.T) {
	book := New(t.TempDir())
	if err := book.Stamp(Entry{Station: "add"}); err != nil {
		t.Fatal(err)
	}
	entries, _ := book.Read(AdhocFile)
	if len(entries) != 1 || entries[0].Station != "add" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestTruncateRunEmptiesOnlyTheRunFile(t *testing.T) {
	book := New(t.TempDir())
	_ = book.Stamp(Entry{Run: "r1", Station: "intake"})
	_ = book.Stamp(Entry{Station: "add"})
	if err := book.TruncateRun(); err != nil {
		t.Fatal(err)
	}
	if run, _ := book.Read(RunFile); len(run) != 0 {
		t.Fatal("the run file survived truncation")
	}
	if adhoc, _ := book.Read(AdhocFile); len(adhoc) != 1 {
		t.Fatal("the ad hoc file was truncated too")
	}
}

func TestAdhocRotatesWhenItsFirstLineIsStale(t *testing.T) {
	dir := t.TempDir()
	book := New(dir)
	old := Entry{Station: "add", At: time.Now().UTC().Add(-48 * time.Hour)}
	if err := book.Stamp(old); err != nil {
		t.Fatal(err)
	}
	if err := book.Stamp(Entry{Station: "add"}); err != nil {
		t.Fatal(err)
	}
	entries, _ := book.Read(AdhocFile)
	if len(entries) != 1 {
		t.Fatalf("the stale file was not rotated: %+v", entries)
	}
}

func TestAdhocRotatesWhenItIsTooLarge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, AdhocFile)
	if err := os.WriteFile(path, make([]byte, AdhocMaxBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := New(dir).Stamp(Entry{Station: "add"}); err != nil {
		t.Fatal(err)
	}
	entries, _ := New(dir).Read(AdhocFile)
	if len(entries) != 1 {
		t.Fatalf("the oversized file was not rotated: %d entries", len(entries))
	}
}

func TestReadSkipsALineThatIsNotAnEntry(t *testing.T) {
	dir := t.TempDir()
	body := "{\"station\":\"brief\"}\nnot json\n\n{\"station\":\"close\"}\n"
	if err := os.WriteFile(filepath.Join(dir, RunFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := New(dir).Read(RunFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
}

// TestAggregateCountsWhatTheStationsStamped builds its fixture from the entry shapes
// add, machine, build and close actually write, not from shapes no station writes.
func TestAggregateCountsWhatTheStationsStamped(t *testing.T) {
	entries := []Entry{
		{Station: "add", Task: "TSK-1", Outcome: "added"},
		{Station: "machine", Task: "TG-01.1-review", Role: "reviewer", Tier: "heavy",
			Provider: "ollama", Model: "llama3.2", TokensIn: 40, TokensOut: 10, Outcome: "done"},
		{Station: "build", Task: "TSK-1", Seconds: 10, Host: "claude", TokensIn: 100, TokensOut: 50, Turns: 3},
		{Station: "build", Task: "TSK-2", Seconds: 30, Host: "claude", TokensIn: 10, TokensOut: 5, Turns: 1},
		{Station: "close", Task: "TSK-1", Seconds: 2, Outcome: "done"},
		{Station: "close", Task: "TSK-2", Seconds: 8, Outcome: "repair", FailureClass: "done_when"},
	}
	metrics := Aggregate(entries)
	if metrics.MedianSeconds["build"] != 20 || metrics.MedianSeconds["close"] != 5 {
		t.Fatalf("medians = %v", metrics.MedianSeconds)
	}
	if metrics.TokensByModel["llama3.2"] != 50 || metrics.TokensByModel["claude (host, no model)"] != 165 {
		t.Fatalf("tokens = %v", metrics.TokensByModel)
	}
	if metrics.FailuresBy["done_when"] != 1 {
		t.Fatalf("metrics = %+v", metrics)
	}
	if metrics.Tasks != 2 || metrics.Repairs != 1 {
		t.Fatalf("tasks = %d repairs = %d", metrics.Tasks, metrics.Repairs)
	}
}

// TestTokensByModelFallsBackToHostWhenNoStationSetAModel proves a build's tokens,
// stamped only under Host, still surface, labelled as a host tally, not a model.
func TestTokensByModelFallsBackToHostWhenNoStationSetAModel(t *testing.T) {
	entries := []Entry{
		{Station: "build", Task: "TSK-1", Host: "claude", TokensIn: 100, TokensOut: 50},
	}
	metrics := Aggregate(entries)
	if metrics.TokensByModel["claude"] != 0 {
		t.Fatalf("the host tally was keyed as a bare model name: %v", metrics.TokensByModel)
	}
	if metrics.TokensByModel["claude (host, no model)"] != 150 {
		t.Fatalf("tokens = %v", metrics.TokensByModel)
	}
}

// TestRepairRateCountsOnlyTasksThatReachedClose proves add and review-pseudo-task
// stamps, which never reach the close station, do not inflate the denominator.
func TestRepairRateCountsOnlyTasksThatReachedClose(t *testing.T) {
	entries := []Entry{
		{Station: "add", Task: "TSK-1", Outcome: "added"},
		{Station: "add", Task: "TSK-2", Outcome: "added"},
		{Station: "machine", Task: "TG-01.1-review", Role: "reviewer", Model: "llama3.2", Outcome: "done"},
		{Station: "close", Task: "TSK-1", Outcome: "done"},
	}
	metrics := Aggregate(entries)
	if metrics.Tasks != 1 {
		t.Fatalf("tasks = %d, want 1 (only TSK-1 reached close)", metrics.Tasks)
	}
	if metrics.RepairRate != 0 {
		t.Fatalf("repair rate = %v", metrics.RepairRate)
	}
}

func TestMedianAveragesAnEvenCount(t *testing.T) {
	if got := median([]float64{1, 2, 3, 4}); got != 2.5 {
		t.Fatalf("median = %v", got)
	}
	if got := median(nil); got != 0 {
		t.Fatalf("median of nothing = %v", got)
	}
}

func TestRenderOpensWithTheVerdict(t *testing.T) {
	text := Render(Aggregate([]Entry{{Station: "close", Task: "a", Seconds: 4, Outcome: "repair"}}))
	first, _, _ := strings.Cut(text, "\n")
	if !strings.HasPrefix(first, "1 task(s), 1 repaired, repair rate 100%") {
		t.Fatalf("first line = %q", first)
	}
	if !strings.Contains(text, "## Median seconds per station") {
		t.Fatalf("text = %s", text)
	}
}

func TestAnEmptyLedgerStillShowsTheThroughputHeading(t *testing.T) {
	text := Render(Aggregate(nil))
	if !strings.Contains(text, "## Tasks per hour") || !strings.Contains(text, "no shipped group is on record yet") {
		t.Fatalf("text = %s; an empty ledger must still name tasks per hour", text)
	}
}

func TestAnEmptyLedgerAggregatesToNothing(t *testing.T) {
	metrics := Aggregate(nil)
	if metrics.Tasks != 0 || metrics.RepairRate != 0 || len(metrics.MedianSeconds) != 0 {
		t.Fatalf("metrics = %+v", metrics)
	}
}

func TestAnEntryCarriesTokensAndTurnsWhenAMountReportsThem(t *testing.T) {
	book := New(t.TempDir())
	if err := book.Stamp(Entry{Run: "r1", Station: "close", Task: "a",
		Host: "h", TokensIn: 100, TokensOut: 20, Turns: 3}); err != nil {
		t.Fatal(err)
	}
	entries, _ := book.Read(RunFile)
	if len(entries) != 1 {
		t.Fatalf("entries = %d", len(entries))
	}
	got := entries[0]
	if got.TokensIn != 100 || got.TokensOut != 20 || got.Turns != 3 || got.Host != "h" {
		t.Fatalf("entry = %+v", got)
	}
}

func TestAnEntryLeavesTokensEmptyWhenNoMountCanSay(t *testing.T) {
	book := New(t.TempDir())
	if err := book.Stamp(Entry{Run: "r1", Station: "close", Task: "a"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(book.Dir, RunFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"tokens_in", "tokens_out", "turns"} {
		if strings.Contains(string(body), field) {
			t.Fatalf("an unknown count was written as a guess: %s", body)
		}
	}
}

func TestAggregateMeasuresThroughput(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	at := func(minutes int) time.Time { return start.Add(time.Duration(minutes) * time.Minute) }
	entries := []Entry{
		{At: at(0), Run: "r1", Group: "TG-1", Station: "intake", Outcome: "started"},
		{At: at(0), Run: "r1", Group: "TG-1", Task: "TSK-1", Station: "brief"},
		{At: at(10), Run: "r1", Group: "TG-1", Task: "TSK-2", Station: "brief"},
		{At: at(20), Run: "r1", Group: "TG-1", Task: "TSK-1", Station: "build", Tier: "light", Model: "haiku", TokensIn: 300, TokensOut: 100},
		{At: at(20), Run: "r1", Group: "TG-1", Task: "TSK-1", Station: "close", Outcome: "repair"},
		{At: at(25), Run: "r1", Group: "TG-1", Task: "TSK-1", Station: "brief"},
		{At: at(30), Run: "r1", Group: "TG-1", Task: "TSK-1", Station: "close", Outcome: "done"},
		{At: at(30), Run: "r1", Group: "TG-1", Task: "TSK-2", Station: "build", Tier: "standard", Model: "sonnet", TokensIn: 500, TokensOut: 300},
		{At: at(30), Run: "r1", Group: "TG-1", Task: "TSK-2", Station: "close", Outcome: "done"},
		{At: at(60), Run: "r1", Group: "TG-1", Station: "ship", Outcome: "done", Lines: 40},
		{At: at(70), Run: "r2", Group: "TG-2", Task: "TSK-3", Station: "build", Model: "sonnet", TokensIn: 9000},
		{At: at(80), Run: "r2", Group: "TG-2", Station: "ship", Outcome: "done"},
		{At: at(40), Run: "r3", Group: "TG-3", Task: "TSK-4", Station: "brief"},
		{At: at(40), Run: "r3", Group: "TG-3", Task: "TSK-4", Station: "build", Model: "opus", TokensIn: 500},
		{At: at(45), Run: "r3", Group: "TG-3", Station: "ship", Outcome: "failed", Lines: 50},
	}
	metrics := Aggregate(entries)
	if metrics.TasksPerHour != 2.0/(75.0/60.0) {
		t.Fatalf("tasks per hour = %v; two tasks done over seventy-five minutes of run time", metrics.TasksPerHour)
	}
	if metrics.MedianTaskSec != 55*60 {
		t.Fatalf("median = %v; briefs at 0 and 10 shipped at 60, a failed ship never counts", metrics.MedianTaskSec)
	}
	if _, ok := metrics.TokensPerLine["opus"]; ok {
		t.Fatalf("tokens per line = %v; a failed ship's lines never count", metrics.TokensPerLine)
	}
	if metrics.TokensPerLine["haiku"] != 10 || metrics.TokensPerLine["sonnet"] != 20 {
		t.Fatalf("tokens per line = %v; a ship without lines is skipped, never read as zero", metrics.TokensPerLine)
	}
	if metrics.RepairByTier["light"] != 1 || metrics.RepairByTier["standard"] != 0 {
		t.Fatalf("repair by tier = %v", metrics.RepairByTier)
	}
	text := Render(metrics)
	for _, heading := range []string{"## Tasks per hour", "## Median wall seconds per task", "## Tokens per changed line", "## Repair rate by tier"} {
		if !strings.Contains(text, heading) {
			t.Fatalf("text lacks %q:\n%s", heading, text)
		}
	}
}

func TestWriteMetricsCreatesFile(t *testing.T) {
	book := New(t.TempDir())
	entries := []Entry{
		{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Run: "r1", Group: "TG-1", Station: "build", Seconds: 10, Turns: 3, TokensIn: 100, TokensOut: 50, TokensCached: 5, Outcome: "done"},
		{At: time.Date(2026, 1, 1, 0, 0, 10, 0, time.UTC), Run: "r1", Group: "TG-1", Station: "build", Seconds: 20, Turns: 2, TokensIn: 200, TokensOut: 100, TokensCached: 10, Outcome: "done"},
	}
	if err := book.WriteMetrics(entries); err != nil {
		t.Fatal(err)
	}
	metrics, err := book.ReadMetrics()
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) == 0 {
		t.Fatal("metrics.jsonl was not written")
	}
	if metrics[0].Run != "r1" || metrics[0].Group != "TG-1" || metrics[0].Stage != "build" {
		t.Fatalf("metric = %+v", metrics[0])
	}
}

// TestMetricsHasOneLinePerStageAndSession proves two brief sessions of the same run and group
// each keep their own metric line, instead of summing into one line for the stage.
func TestMetricsHasOneLinePerStageAndSession(t *testing.T) {
	book := New(t.TempDir())
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	entries := []Entry{
		{At: start, Run: "r1", Group: "TG-1", Station: "brief", TokensIn: 100, Outcome: "done"},
		{At: start.Add(5 * time.Second), Run: "r1", Group: "TG-1", Station: "brief", TokensIn: 50, Outcome: "done"},
		{At: start, Run: "r1", Group: "TG-2", Station: "build", TokensIn: 200, Outcome: "done"},
	}
	if err := book.WriteMetrics(entries); err != nil {
		t.Fatal(err)
	}
	metrics, _ := book.ReadMetrics()
	if len(metrics) != 3 {
		t.Fatalf("expected 3 metric lines, one per session, got %d", len(metrics))
	}
	var briefCount, buildCount int
	for _, m := range metrics {
		if m.Stage == "brief" {
			briefCount++
		}
		if m.Stage == "build" {
			buildCount++
		}
	}
	if briefCount != 2 || buildCount != 1 {
		t.Fatalf("brief count = %d, build count = %d", briefCount, buildCount)
	}
}

func TestMetricsFieldsArePopulated(t *testing.T) {
	book := New(t.TempDir())
	at := time.Date(2026, 1, 1, 12, 30, 45, 0, time.UTC)
	entries := []Entry{
		{At: at, Run: "r1", Group: "TG-1", Station: "build", Seconds: 15, Turns: 4, TokensIn: 300, TokensOut: 150, TokensCached: 20, Outcome: "done"},
	}
	if err := book.WriteMetrics(entries); err != nil {
		t.Fatal(err)
	}
	metrics, _ := book.ReadMetrics()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	m := metrics[0]
	if m.Start.IsZero() || m.Duration != 15 || m.Turns != 4 || m.Input != 300 || m.Output != 150 || m.CachedTokens != 20 || m.Outcome != "done" {
		t.Fatalf("metric = %+v", m)
	}
}

func TestWriteEventsCreatesFile(t *testing.T) {
	book := New(t.TempDir())
	entries := []Entry{
		{At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Run: "r1", Group: "TG-1", Station: "close", Outcome: "escalated"},
		{At: time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC), Run: "r1", Group: "TG-1", Station: "close", Outcome: "done"},
	}
	if err := book.WriteEvents(entries); err != nil {
		t.Fatal(err)
	}
	events, err := book.ReadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("events.jsonl was not written")
	}
}
