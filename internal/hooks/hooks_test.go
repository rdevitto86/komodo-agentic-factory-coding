package hooks

import (
	"bytes"
	"encoding/json"
	"komodo/internal/testhome"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog/backlogtest"
	"komodo/internal/mount"
)

// testHost is the mount name the tests register an encoder and write tools under.
const testHost = "hooks-test"

func TestMain(m *testing.M) {
	// A task check that runs these tests carries the nesting marker, which makes every hook allow.
	_ = os.Unsetenv(nestedEnv)
	RegisterEncoder(testHost, func(event Event, out Outcome) []byte {
		data, _ := json.Marshal(map[string]string{"event": string(event), "verdict": string(out.Verdict), "message": out.Message})
		return data
	})
	mount.RegisterGuard(testHost, mount.GuardTools{
		WriteTools: map[string]bool{"Write": true},
		PathFields: []string{"file_path"},
	})
	cleanup := testhome.Isolate()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// dispatchResult is what one Dispatch call printed and returned.
type dispatchResult struct {
	stdout string
	stderr string
	code   int
}

// dispatch runs one hook on a payload built from fields.
func dispatch(t *testing.T, root, name string, fields map[string]any, args ...string) dispatchResult {
	t.Helper()
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Dispatch(root, name, args, bytes.NewReader(data), &stdout, &stderr)
	return dispatchResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// groupRoot writes a worktree named for a group whose one task's check is command.
func groupRoot(t *testing.T, command string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "TG-90.1")
	// Its own .git stops the worktree walk here; a sandbox's temp dir sits inside the real worktree.
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	backlog := "# Backlog\n\n### [TG-90.1] A group\n```yaml\ntype: feat\nversion: 3.0.0\n```\n\n" +
		"#### [TSK-90.1.1] A task [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when: [\"" + command + "\"]\n```\n"
	backlogtest.SeedText(t, root, backlog)
	return root
}

func TestEveryHookHasOneJobOneStageAndAFailOpenBehaviour(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, hook := range Table() {
		if seen[hook.Name] {
			t.Fatalf("hook %s is in the table twice", hook.Name)
		}
		seen[hook.Name] = true
		if hook.Job == "" || hook.Event == "" || len(hook.Sessions) == 0 || hook.Timeout <= 0 {
			t.Fatalf("hook %s lacks a job, stage, session or timeout: %+v", hook.Name, hook)
		}
		if hook.OnFailure != AllowAndLog && hook.OnFailure != Skip {
			t.Fatalf("hook %s does not fail open: %q", hook.Name, hook.OnFailure)
		}
	}
}

func TestLimitsMatchTheContract(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		event Event
		limit int
	}{
		{"guard", PreToolUse, 3},
		{"format", PostToolUse, 0},
		{"taskchecks", Stop, 3},
		{"evidence", Stop, 2},
		{"timewarn", PostToolUse, 0},
		{"prune", SessionStart, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hook, ok := Lookup(tc.name)
			if !ok {
				t.Fatalf("no hook %s", tc.name)
			}
			if hook.Event != tc.event || hook.Limit != tc.limit {
				t.Fatalf("%s = %s limit %d, want %s limit %d", tc.name, hook.Event, hook.Limit, tc.event, tc.limit)
			}
		})
	}
}

func TestForSessionListsOnlyThatSessionsOwnHooks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		session Session
		want    []string
	}{
		{SessionBuilder, []string{"format", "taskchecks", "timewarn"}},
		{SessionLens, []string{"evidence", "timewarn"}},
		{SessionEvery, []string{"guard"}},
	}
	for _, tc := range cases {
		t.Run(string(tc.session), func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, hook := range ForSession(tc.session) {
				got = append(got, hook.Name)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("ForSession(%s) = %v, want %v", tc.session, got, tc.want)
			}
		})
	}
}

func TestDispatchFailsOpen(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		hook    string
		payload string
		logs    bool
	}{
		{"unknown hook", "nosuch", "{}", true},
		{"unparseable payload, allow and log", "taskchecks", "not json", true},
		{"unparseable payload, skip", "format", "not json", false},
		{"no backlog to read", "taskchecks", `{"session_id":"s"}`, true},
		{"the guard runs under its own command", "guard", "{}", true},
		{"no clock to read", "timewarn", `{"cwd":"/"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := Dispatch(t.TempDir(), tc.hook, nil, strings.NewReader(tc.payload), &stdout, &stderr)
			if code != 0 || stdout.Len() != 0 {
				t.Fatalf("code %d, stdout %q; want an allow", code, stdout.String())
			}
			if logged := strings.Contains(stderr.String(), "allowing"); logged != tc.logs {
				t.Fatalf("stderr = %q, want logged %v", stderr.String(), tc.logs)
			}
		})
	}
}

func TestARefusalAllowsOnceTheLimitIsReached(t *testing.T) {
	t.Parallel()
	root := groupRoot(t, "false")
	fields := map[string]any{"session_id": "limit", "cwd": root}
	for i := 1; i <= 3; i++ {
		refused := dispatch(t, root, "taskchecks", fields)
		if refused.code != ExitRefuse || !strings.Contains(refused.stderr, "`false` failed") {
			t.Fatalf("refusal %d: %+v", i, refused)
		}
	}
	allowed := dispatch(t, root, "taskchecks", fields)
	if allowed.code != 0 || !strings.Contains(allowed.stderr, "3 refusals reached; allowing") {
		t.Fatalf("the fourth stop was refused: %+v", allowed)
	}
}

func TestARegisteredEncoderRendersTheOutcome(t *testing.T) {
	t.Parallel()
	root := groupRoot(t, "false")
	got := dispatch(t, root, "taskchecks", map[string]any{"session_id": "encoded", "cwd": root}, "--host", testHost)
	if got.code != 0 {
		t.Fatalf("code = %d, want 0 with the encoder's payload", got.code)
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(got.stdout), &decoded); err != nil {
		t.Fatalf("stdout %q: %v", got.stdout, err)
	}
	if decoded["event"] != string(Stop) || decoded["verdict"] != string(Refuse) {
		t.Fatalf("decoded = %v", decoded)
	}
}

func TestDispatchAllowsOnAnUnknownFlag(t *testing.T) {
	t.Parallel()
	got := dispatch(t, t.TempDir(), "taskchecks", map[string]any{}, "--nosuch")
	if got.code != 0 || got.stdout != "" || !strings.Contains(got.stderr, "allowing") {
		t.Fatalf("got %+v, want a logged allow", got)
	}
}

func TestARefusalCountThatCannotBeKeptAllows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(t *testing.T, state string)
		logs  bool
	}{
		{"a corrupt count starts again", func(t *testing.T, state string) {
			if err := os.MkdirAll(filepath.Dir(state), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(state, []byte("not json"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"an unreadable count", func(t *testing.T, state string) {
			if err := os.MkdirAll(state, 0o755); err != nil {
				t.Fatal(err)
			}
		}, true},
		{"no room for the count", func(t *testing.T, state string) {
			if err := os.MkdirAll(filepath.Dir(filepath.Dir(state)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Dir(state), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := groupRoot(t, "false")
			tc.setup(t, filepath.Join(root, ".komodo", "runs", "hook-refusals", "count.json"))
			got := dispatch(t, root, "taskchecks", map[string]any{"session_id": "count", "cwd": root})
			if tc.logs {
				if got.code != 0 || !strings.Contains(got.stderr, "allowing") {
					t.Fatalf("got %+v, want a logged allow", got)
				}
				return
			}
			if got.code != ExitRefuse {
				t.Fatalf("got %+v, want a refusal counted afresh", got)
			}
		})
	}
}
