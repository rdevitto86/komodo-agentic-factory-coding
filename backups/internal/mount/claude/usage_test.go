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
	if usage.TokensIn != 153 || usage.TokensCached != 5 || usage.TokensOut != 30 {
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

func TestUsageSumsOnlyTheSpawnedTranscriptsThatWereHandedTheBrief(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, "repo")
	dir := TranscriptDir(root)
	spawned := filepath.Join(dir, "session", "subagents")
	if err := os.MkdirAll(spawned, 0o755); err != nil {
		t.Fatal(err)
	}
	turn := `{"type":"assistant","message":{"usage":{"input_tokens":10,"output_tokens":1,"cache_read_input_tokens":100}}}`
	files := map[string]string{
		filepath.Join(dir, "session.jsonl"):       `{"type":"user","message":{"content":"komodo brief TSK-01.1.1 wrote .komodo/briefs/TSK-01.1.1.md"}}` + "\n" + turn,
		filepath.Join(spawned, "agent-a.jsonl"):   `{"type":"user","message":{"content":"Your brief is .komodo/briefs/TSK-01.1.1.md"}}` + "\n" + turn + "\n" + turn,
		filepath.Join(spawned, "agent-b.jsonl"):   `{"type":"user","message":{"content":"Your brief is .komodo/briefs/TSK-01.1.2.md"}}` + "\n" + turn,
		filepath.Join(spawned, "agent-fix.jsonl"): `{"type":"user","message":{"content":"Repair: .komodo/briefs/TSK-01.1.1.md"}}` + "\n" + turn,
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	usage, ok := Usage(root, "TSK-01.1.1", time.Time{}, time.Time{})
	if !ok {
		t.Fatal("no usage was attributed")
	}
	if usage.Turns != 3 || usage.TokensIn != 30 || usage.TokensCached != 300 || usage.TokensOut != 3 {
		t.Fatalf("usage = %+v; the session's own turn or another task's agent was counted", usage)
	}
	if _, ok := Usage(root, "TSK-01.1.9", time.Time{}, time.Time{}); ok {
		t.Fatal("usage was attributed to a task no agent was handed")
	}
}
