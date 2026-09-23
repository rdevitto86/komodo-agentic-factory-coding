package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSumTranscriptCountsOnlyAssistantTurns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	body := strings.Join([]string{
		`{"type":"user","message":{"usage":{"input_tokens":999}}}`,
		`{"type":"assistant","message":{"usage":{"input_tokens":100,"output_tokens":20,"cache_read_input_tokens":5,"cache_creation_input_tokens":3}}}`,
		`{"type":"assistant","message":{"usage":{"input_tokens":50,"output_tokens":10}}}`,
		`not json`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	usage, ok := sumTranscript(path, time.Time{}, time.Time{})
	if !ok {
		t.Fatal("the transcript did not parse")
	}
	if usage.Turns != 2 {
		t.Fatalf("turns = %d", usage.Turns)
	}
	if usage.TokensIn != 158 || usage.TokensOut != 30 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestSumTranscriptHonoursTheWindow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	body := strings.Join([]string{
		`{"type":"assistant","timestamp":"2026-09-21T10:00:00Z","message":{"usage":{"output_tokens":1}}}`,
		`{"type":"assistant","timestamp":"2026-09-21T12:00:00Z","message":{"usage":{"output_tokens":2}}}`,
		`{"type":"assistant","timestamp":"2026-09-21T14:00:00Z","message":{"usage":{"output_tokens":4}}}`,
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	since, _ := time.Parse(time.RFC3339, "2026-09-21T11:00:00Z")
	until, _ := time.Parse(time.RFC3339, "2026-09-21T13:00:00Z")
	usage, _ := sumTranscript(path, since, until)
	if usage.Turns != 1 || usage.TokensOut != 2 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestTheEntryStructHoldsNoMessageText(t *testing.T) {
	var parsed entry
	body := `{"type":"assistant","message":{"content":[{"type":"text","text":"a secret"}],"usage":{"output_tokens":1}}}`
	if err := unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}
	rendered, err := marshal(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered, "a secret") || strings.Contains(rendered, "content") {
		t.Fatalf("the entry carried message text: %s", rendered)
	}
}

func TestUsageReturnsNothingWithoutTranscripts(t *testing.T) {
	if _, ok := Usage(t.TempDir(), "TSK-01.1.1", time.Time{}, time.Time{}); ok {
		t.Fatal("usage was reported with no transcripts")
	}
}

func TestSlugMatchesThisHostsProjectDirectories(t *testing.T) {
	if got := slug("/Users/rad/komodo/ai/x"); got != "-Users-rad-komodo-ai-x" {
		t.Fatalf("slug = %s", got)
	}
}

// unmarshal decodes one transcript line in a test.
func unmarshal(body string, target *entry) error { return json.Unmarshal([]byte(body), target) }

// marshal renders a decoded entry so a test can prove what it kept.
func marshal(value entry) (string, error) {
	data, err := json.Marshal(value)
	return string(data), err
}
