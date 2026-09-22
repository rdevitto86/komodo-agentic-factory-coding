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

func TestAggregateCountsWhatTheStationsStamped(t *testing.T) {
	entries := []Entry{
		{Station: "build", Task: "a", Seconds: 10, Model: "sonnet", TokensIn: 100, TokensOut: 50},
		{Station: "build", Task: "b", Seconds: 20, Model: "sonnet", TokensIn: 100, TokensOut: 50, Outcome: "repair"},
		{Station: "build", Task: "c", Seconds: 30, Model: "opus", TokensIn: 10, TokensOut: 5},
		{Station: "close", Task: "c", Seconds: 2, FailureClass: "done_when"},
		{Station: "review", Group: "TG-01.1", Findings: 3},
	}
	metrics := Aggregate(entries)
	if metrics.MedianSeconds["build"] != 20 || metrics.MedianSeconds["close"] != 2 {
		t.Fatalf("medians = %v", metrics.MedianSeconds)
	}
	if metrics.TokensByModel["sonnet"] != 300 || metrics.TokensByModel["opus"] != 15 {
		t.Fatalf("tokens = %v", metrics.TokensByModel)
	}
	if metrics.FailuresBy["done_when"] != 1 || metrics.FindingsBy["TG-01.1"] != 3 {
		t.Fatalf("metrics = %+v", metrics)
	}
	if metrics.Tasks != 3 || metrics.Repairs != 1 {
		t.Fatalf("tasks = %d repairs = %d", metrics.Tasks, metrics.Repairs)
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
	text := Render(Aggregate([]Entry{{Station: "build", Task: "a", Seconds: 4, Outcome: "repair"}}))
	first, _, _ := strings.Cut(text, "\n")
	if !strings.HasPrefix(first, "1 task(s), 1 repaired, repair rate 100%") {
		t.Fatalf("first line = %q", first)
	}
	if !strings.Contains(text, "## Median seconds per station") {
		t.Fatalf("text = %s", text)
	}
}

func TestAnEmptyLedgerAggregatesToNothing(t *testing.T) {
	metrics := Aggregate(nil)
	if metrics.Tasks != 0 || metrics.RepairRate != 0 || len(metrics.MedianSeconds) != 0 {
		t.Fatalf("metrics = %+v", metrics)
	}
}
