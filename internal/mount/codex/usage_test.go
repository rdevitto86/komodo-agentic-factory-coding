package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUsageReadsTheCapturedEvents(t *testing.T) {
	root := t.TempDir()
	path := EventsPath(root, "TSK-01.1.1")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Join([]string{
		`{"type":"item.started"}`,
		`{"type":"turn.completed","usage":{"input_tokens":120,"output_tokens":30}}`,
		`{"type":"turn.completed","usage":{"input_tokens":80,"output_tokens":20}}`,
		`not json`,
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	usage, ok := Usage(root, "TSK-01.1.1", time.Time{}, time.Time{})
	if !ok {
		t.Fatal("the events did not parse")
	}
	if usage.TokensIn != 200 || usage.TokensOut != 50 || usage.Turns != 2 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestUsageReturnsNothingInASession(t *testing.T) {
	if _, ok := Usage(t.TempDir(), "TSK-01.1.1", time.Time{}, time.Time{}); ok {
		t.Fatal("usage was reported with no captured events")
	}
}

func TestUsageReturnsNothingWhenNoEventCarriesCounts(t *testing.T) {
	root := t.TempDir()
	path := EventsPath(root, "TSK-01.1.1")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"type":"item.started"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := Usage(root, "TSK-01.1.1", time.Time{}, time.Time{}); ok {
		t.Fatal("an empty count was reported as usage")
	}
}

func TestProbeReportsNothing(t *testing.T) {
	if _, ok := Probe(); ok {
		t.Fatal("this host exposes no plan and must report none")
	}
}
