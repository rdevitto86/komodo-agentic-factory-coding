package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// clockRoot is a worktree of its own, so a session's clock lands in it.
func clockRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// Its own .git stops the worktree walk here; a sandbox's temp dir sits inside the real worktree.
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
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
