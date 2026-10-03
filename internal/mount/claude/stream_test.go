package claude

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"komodo/internal/mount"
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

func TestParseReportsARateLimitEventWithAnEpochSecondsReset(t *testing.T) {
	body := `{"type":"rate_limit_event","five_hour":{"utilization":18,"resetsAt":1758909600},` +
		`"seven_day":{"utilization":6,"resetsAt":1759449600}}`
	events := drain(t, body)
	if len(events) != 1 || events[0].RateLimit == nil {
		t.Fatalf("events = %+v", events)
	}
	limit := events[0].RateLimit
	want := time.Unix(1758909600, 0)
	if limit.FiveHour != 0.18 || limit.SevenDay != 0.06 || !limit.ResetsAt.Equal(want) {
		t.Fatalf("rate limit = %+v", limit)
	}
}

func TestParseKeepsUtilisationWhenResetsAtFailsToParse(t *testing.T) {
	body := `{"type":"rate_limit_event","five_hour":{"utilization":18,"resetsAt":"not a time"},` +
		`"seven_day":{"utilization":6,"resetsAt":"not a time"}}`
	events := drain(t, body)
	if len(events) != 1 || events[0].RateLimit == nil {
		t.Fatalf("events = %+v", events)
	}
	limit := events[0].RateLimit
	if limit.FiveHour != 0.18 || limit.SevenDay != 0.06 || !limit.ResetsAt.IsZero() {
		t.Fatalf("rate limit = %+v, want the utilisation kept and a zero reset", limit)
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

// boomReader yields body once, then fails every further read with err instead of io.EOF.
type boomReader struct {
	body []byte
	err  error
}

func (r *boomReader) Read(p []byte) (int, error) {
	if len(r.body) > 0 {
		n := copy(p, r.body)
		r.body = r.body[n:]
		return n, nil
	}
	return 0, r.err
}

func TestParseReportsAScannerErrorAsTheFinalEvent(t *testing.T) {
	boom := errors.New("boom")
	body := `{"type":"result","num_turns":1,"session_id":"abc","total_cost_usd":0.1,"usage":{}}` + "\n"
	events := drainReader(t, &boomReader{body: []byte(body), err: boom})
	if len(events) != 2 {
		t.Fatalf("events = %+v, want the result event and the scanner error", events)
	}
	if events[0].SessionID != "abc" {
		t.Fatalf("result event = %+v", events[0])
	}
	if !errors.Is(events[1].Err, boom) {
		t.Fatalf("final event err = %v, want %v", events[1].Err, boom)
	}
}

func TestParseStopsItsGoroutineOnceCtxIsCancelled(t *testing.T) {
	body := strings.Join([]string{
		`{"type":"result","num_turns":1,"session_id":"a","total_cost_usd":0,"usage":{}}`,
		`{"type":"result","num_turns":2,"session_id":"b","total_cost_usd":0,"usage":{}}`,
	}, "\n")
	ctx, cancel := context.WithCancel(context.Background())
	before := runtime.NumGoroutine()
	out := Parse(ctx, strings.NewReader(body))
	if event := <-out; event.SessionID != "a" {
		t.Fatalf("first event = %+v", event)
	}
	cancel()
	// The consumer never reads again; a leaked goroutine would stay blocked sending the second event.
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before {
		if time.Now().After(deadline) {
			t.Fatal("Parse's goroutine outlived a cancelled ctx with no reader left; it leaked")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// drain parses body and collects every event Parse reports.
func drain(t *testing.T, body string) []mount.Event {
	t.Helper()
	return drainReader(t, strings.NewReader(body))
}

// drainReader parses r and collects every event Parse reports.
func drainReader(t *testing.T, r io.Reader) []mount.Event {
	t.Helper()
	var events []mount.Event
	for event := range Parse(context.Background(), r) {
		events = append(events, event)
	}
	return events
}

// drainFile parses one fixture file and collects every event Parse reports.
func drainFile(t *testing.T, path string) []mount.Event {
	t.Helper()
	handle, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	var events []mount.Event
	for event := range Parse(context.Background(), handle) {
		events = append(events, event)
	}
	return events
}
