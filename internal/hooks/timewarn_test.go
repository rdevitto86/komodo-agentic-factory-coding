package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// clockRoot is a worktree of its own, so a session's clock lands in it.
func clockRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// Its own .git stops the worktree walk here, and git's too; a sandbox's temp dir sits inside the real worktree.
	for _, dir := range []string{"objects", "refs"} {
		if err := os.MkdirAll(filepath.Join(root, ".git", dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestTimeWarnWarnsOnceAt80PercentOfTurns(t *testing.T) {
	t.Parallel()
	root := clockRoot(t)
	fields := map[string]any{"session_id": "turns", "cwd": root, "tool_name": "Read"}
	var warnings int
	for turn := 1; turn <= 5; turn++ {
		got := dispatch(t, root, "timewarn", fields, "--turns", "5")
		if got.code != 0 {
			t.Fatalf("turn %d refused: %+v", turn, got)
		}
		if got.stdout != "" {
			warnings++
			if turn != 4 || !strings.Contains(got.stdout, "4 of 5 turns") {
				t.Fatalf("turn %d warned %q, want only turn 4", turn, got.stdout)
			}
		}
	}
	if warnings != 1 {
		t.Fatalf("warned %d times, want once", warnings)
	}
}

func TestTimeWarnWarnsAt80PercentOfTime(t *testing.T) {
	t.Parallel()
	root := clockRoot(t)
	clock, err := json.Marshal(sessionClock{Started: time.Now().Add(-21 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeState(filepath.Join(root, ".komodo", "runs", "hook-clock", "late.json"), json.RawMessage(clock)); err != nil {
		t.Fatal(err)
	}
	got := dispatch(t, root, "timewarn", map[string]any{"session_id": "late", "cwd": root}, "--minutes", "25")
	if got.code != 0 || !strings.Contains(got.stdout, "21 of 25 minutes") {
		t.Fatalf("no time warning: %+v", got)
	}
}

// TestTimeWarnMessageOmitsAnUnsetBudget proves a turn-only call never reads "0 of 0 minutes";
// the message names only the budget that is actually set.
func TestTimeWarnMessageOmitsAnUnsetBudget(t *testing.T) {
	t.Parallel()
	root := clockRoot(t)
	fields := map[string]any{"session_id": "unset", "cwd": root, "tool_name": "Read"}
	var got dispatchResult
	for turn := 1; turn <= 4; turn++ {
		got = dispatch(t, root, "timewarn", fields, "--turns", "5")
	}
	if got.code != 0 || !strings.Contains(got.stdout, "4 of 5 turns") {
		t.Fatalf("no turn warning: %+v", got)
	}
	if strings.Contains(got.stdout, "minutes") {
		t.Fatalf("message names minutes with no minutes budget set: %q", got.stdout)
	}
}

// writeTranscript writes a fixture transcript whose assistant entries name turns turns, the way
// countTurns reads one.
func writeTranscript(t *testing.T, path string, turns int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var lines []string
	for i := 0; i < turns; i++ {
		lines = append(lines, `{"type":"assistant","message":{"role":"assistant","content":[]}}`)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestTimeWarnCountsTranscriptTurnsNotParallelToolCalls proves four parallel tool calls inside one
// assistant turn count as one turn, and a later turn grows the count by one, not by its own calls.
func TestTimeWarnCountsTranscriptTurnsNotParallelToolCalls(t *testing.T) {
	t.Parallel()
	root := clockRoot(t)
	transcript := filepath.Join(root, "transcript.jsonl")
	writeTranscript(t, transcript, 1)
	fields := map[string]any{"session_id": "parallel", "cwd": root, "tool_name": "Read", "transcript_path": transcript}
	for i := 0; i < 4; i++ {
		if got := dispatch(t, root, "timewarn", fields, "--turns", "5"); got.code != 0 {
			t.Fatalf("call %d refused: %+v", i, got)
		}
	}
	readClock := func() sessionClock {
		data, err := os.ReadFile(filepath.Join(root, ".komodo", "runs", "hook-clock", "parallel.json"))
		if err != nil {
			t.Fatal(err)
		}
		var clock sessionClock
		if err := json.Unmarshal(data, &clock); err != nil {
			t.Fatal(err)
		}
		return clock
	}
	if clock := readClock(); clock.Turns != 1 {
		t.Fatalf("turns = %d, want 1 after 4 parallel tool calls in one turn", clock.Turns)
	}

	writeTranscript(t, transcript, 2)
	if got := dispatch(t, root, "timewarn", fields, "--turns", "5"); got.code != 0 {
		t.Fatalf("turn 2 refused: %+v", got)
	}
	if clock := readClock(); clock.Turns != 2 {
		t.Fatalf("turns = %d, want 2 once the transcript's second turn lands", clock.Turns)
	}
}

// TestTimeWarnKeepsTheClockConsistentUnderConcurrentParallelToolCalls fires one turn's parallel
// calls as concurrent hooks, proving the lock and atomic write keep its turn count at one.
func TestTimeWarnKeepsTheClockConsistentUnderConcurrentParallelToolCalls(t *testing.T) {
	t.Parallel()
	root := clockRoot(t)
	transcript := filepath.Join(root, "transcript.jsonl")
	writeTranscript(t, transcript, 1)
	payload, err := json.Marshal(map[string]any{
		"session_id": "race", "cwd": root, "tool_name": "Read", "transcript_path": transcript,
	})
	if err != nil {
		t.Fatal(err)
	}
	const calls = 20
	var wg sync.WaitGroup
	codes := make([]int, calls)
	for i := range calls {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var stdout, stderr bytes.Buffer
			codes[i] = Dispatch(root, "timewarn", []string{"--turns", "100"}, bytes.NewReader(payload), &stdout, &stderr)
		}(i)
	}
	wg.Wait()
	for i, code := range codes {
		if code != 0 {
			t.Fatalf("call %d refused: %d", i, code)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, ".komodo", "runs", "hook-clock", "race.json"))
	if err != nil {
		t.Fatal(err)
	}
	var clock sessionClock
	if err := json.Unmarshal(data, &clock); err != nil {
		t.Fatalf("the clock file was corrupted by a concurrent write: %v (%s)", err, data)
	}
	if clock.Turns != 1 {
		t.Fatalf("turns = %d, want 1: twenty parallel calls in one turn must never inflate the count", clock.Turns)
	}
}

func TestTimeWarnStaysQuietUnder80Percent(t *testing.T) {
	t.Parallel()
	root := clockRoot(t)
	got := dispatch(t, root, "timewarn", map[string]any{"session_id": "early", "cwd": root}, "--minutes", "25", "--turns", "150")
	if got.code != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("an early turn warned: %+v", got)
	}
}

func TestTimeWarnSkipsWhenItCannotReadTheClock(t *testing.T) {
	t.Parallel()
	root := clockRoot(t)
	path := filepath.Join(root, ".komodo", "runs", "hook-clock", "broken.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		fields map[string]any
		args   []string
	}{
		{"unreadable clock", map[string]any{"session_id": "broken", "cwd": root}, []string{"--turns", "1"}},
		{"no session", map[string]any{"cwd": root}, []string{"--turns", "1"}},
		{"no budget", map[string]any{"session_id": "free", "cwd": root}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := dispatch(t, root, "timewarn", tc.fields, tc.args...)
			if got.code != 0 || got.stdout != "" || got.stderr != "" {
				t.Fatalf("got %+v, want a silent skip", got)
			}
		})
	}
}
