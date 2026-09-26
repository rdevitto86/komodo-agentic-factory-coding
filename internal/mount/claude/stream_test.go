package claude

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseReportsTheResultEventsTotals(t *testing.T) {
	body := `{"type":"result","num_turns":5,"session_id":"abc-123","total_cost_usd":0.0842,` +
		`"usage":{"input_tokens":1200,"cache_creation_input_tokens":300,"cache_read_input_tokens":8000,"output_tokens":450}}`
	events := drain(t, body)
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	event := events[0]
	if event.Turns != 5 || event.SessionID != "abc-123" || event.CostUSD != 0.0842 {
		t.Fatalf("event = %+v", event)
	}
	if event.Usage.TokensIn != 1500 || event.Usage.TokensOut != 450 || event.Usage.TokensCached != 8000 || event.Usage.Turns != 5 {
		t.Fatalf("usage = %+v", event.Usage)
	}
}

func TestParseReportsARateLimitEventsWindows(t *testing.T) {
	body := `{"type":"rate_limit_event","five_hour":{"utilization":18,"resetsAt":"2026-09-26T18:00:00Z"},` +
		`"seven_day":{"utilization":6,"resetsAt":"2026-10-03T00:00:00Z"}}`
	events := drain(t, body)
	if len(events) != 1 || events[0].RateLimit == nil {
		t.Fatalf("events = %+v", events)
	}
	limit := events[0].RateLimit
	wantReset, _ := time.Parse(time.RFC3339, "2026-09-26T18:00:00Z")
	if limit.FiveHour != 0.18 || limit.SevenDay != 0.06 || !limit.ResetsAt.Equal(wantReset) {
		t.Fatalf("rate limit = %+v", limit)
	}
}

func TestParseSkipsALineItDoesNotDecode(t *testing.T) {
	body := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"abc-123"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"redacted"}]}}`,
		"not json",
		"",
	}, "\n")
	if events := drain(t, body); len(events) != 0 {
		t.Fatalf("events = %+v, want none", events)
	}
}

func TestParseHandlesTheRecordedStartAndResumeStreams(t *testing.T) {
	start := drainFile(t, "testdata/stream_start.jsonl")
	if len(start) != 2 {
		t.Fatalf("start events = %d, want 2", len(start))
	}
	if start[0].RateLimit == nil || start[0].RateLimit.FiveHour != 0.18 || start[0].RateLimit.SevenDay != 0.06 {
		t.Fatalf("start rate limit = %+v", start[0].RateLimit)
	}
	result := start[1]
	if result.Turns != 5 || result.SessionID != "ffffffff-ffff-ffff-ffff-ffffffffffff" {
		t.Fatalf("start result = %+v", result)
	}
	if result.Usage.TokensIn != 1500 || result.Usage.TokensOut != 450 || result.Usage.TokensCached != 8000 {
		t.Fatalf("start usage = %+v", result.Usage)
	}
	if result.CostUSD != 0.0842 {
		t.Fatalf("start cost = %v", result.CostUSD)
	}

	resume := drainFile(t, "testdata/stream_resume.jsonl")
	if len(resume) != 2 {
		t.Fatalf("resume events = %d, want 2", len(resume))
	}
	resumed := resume[1]
	if resumed.Turns != 2 || resumed.SessionID != result.SessionID {
		t.Fatalf("resume did not keep the first session's ID: %+v", resumed)
	}
	if resumed.Usage.TokensIn != 400 || resumed.Usage.TokensOut != 120 || resumed.Usage.TokensCached != 9500 {
		t.Fatalf("resume usage = %+v", resumed.Usage)
	}
}

// drain parses body and collects every event Parse reports.
func drain(t *testing.T, body string) []Event {
	t.Helper()
	var events []Event
	for event := range Parse(strings.NewReader(body)) {
		events = append(events, event)
	}
	return events
}

// drainFile parses one fixture file and collects every event Parse reports.
func drainFile(t *testing.T, path string) []Event {
	t.Helper()
	handle, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	var events []Event
	for event := range Parse(handle) {
		events = append(events, event)
	}
	return events
}
