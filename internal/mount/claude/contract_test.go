package claude

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/mount"
	"komodo/internal/proc"
)

// fakeClaudeScript replays the start or resume fixture, reports --version and auth status, or hangs.
const fakeClaudeScript = `#!/bin/sh
pwd >> "$FAKE_CLAUDE_LOG"
echo "$@" >> "$FAKE_CLAUDE_LOG"
if [ "$1" = "--version" ]; then
  echo "$FAKE_CLAUDE_VERSION"
  exit 0
fi
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  echo "$FAKE_CLAUDE_AUTH"
  exit 0
fi
if [ "$FAKE_CLAUDE_FORK" = "1" ]; then
  for i in $(seq 1 30); do sleep "$FAKE_CLAUDE_FORK_SLEEP" & done
  sleep 20
fi
if [ "$FAKE_CLAUDE_HANG" = "1" ]; then
  sleep 30 &
  wait
  exit 0
fi
for arg in "$@"; do
  if [ "$arg" = "--resume" ]; then
    cat "$FAKE_CLAUDE_RESUME_FIXTURE"
    exit 0
  fi
done
cat "$FAKE_CLAUDE_START_FIXTURE"
`

// setupFakeClaude drops the fake claude script on a fresh PATH entry and points its fixtures and
// log at absolute paths, so the mount's own exec.Command("claude", ...) finds it.
func setupFakeClaude(t *testing.T) (logPath string) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	if err := os.WriteFile(script, []byte(fakeClaudeScript), 0o755); err != nil {
		t.Fatal(err)
	}
	logPath = filepath.Join(t.TempDir(), "argv.log")
	start, err := filepath.Abs("testdata/stream_start.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	resume, err := filepath.Abs("testdata/stream_resume.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("FAKE_CLAUDE_LOG", logPath)
	t.Setenv("FAKE_CLAUDE_VERSION", "2.1.283 (Claude Code)")
	t.Setenv("FAKE_CLAUDE_AUTH", `{"loggedIn":true}`)
	t.Setenv("FAKE_CLAUDE_START_FIXTURE", start)
	t.Setenv("FAKE_CLAUDE_RESUME_FIXTURE", resume)
	t.Setenv("FAKE_CLAUDE_HANG", "0")
	t.Setenv("FAKE_CLAUDE_FORK", "0")
	return logPath
}

// drainMountEvents collects every event a Contract's Stream reports.
func drainMountEvents(t *testing.T, events <-chan mount.Event) []mount.Event {
	t.Helper()
	var out []mount.Event
	for event := range events {
		out = append(out, event)
	}
	return out
}

func newFakeRequest() mount.StartRequest {
	return mount.StartRequest{
		Role:   "builder",
		Brief:  "fix the parser bug",
		Tools:  []string{"read", "edit"},
		Model:  "claude-sonnet-5",
		Effort: "extended",
		Schema: []byte(`{"type":"object"}`),
	}
}

func TestContractStartRunsClaudeAndStreamsTheStartFixture(t *testing.T) {
	ctx := context.Background()
	setupFakeClaude(t)
	root, worktree := t.TempDir(), t.TempDir()
	withLineSettings(t, root)
	m := NewMount(root, worktree, 10, 0)

	handle, err := m.Start(ctx, newFakeRequest())
	if err != nil {
		t.Fatalf("Start = %v", err)
	}
	events, err := m.Stream(ctx, handle)
	if err != nil {
		t.Fatalf("Stream = %v", err)
	}
	got := drainMountEvents(t, events)
	if len(got) != 2 {
		t.Fatalf("events = %+v, want 2", got)
	}
	result := got[1]
	if result.Turns != 5 || result.CostUSD != 0.0842 {
		t.Fatalf("result event = %+v", result)
	}
	if result.SessionID != "ffffffff-ffff-ffff-ffff-ffffffffffff" {
		t.Fatalf("result event's session id = %q; the contract's Stream must carry it", result.SessionID)
	}

	value, err := m.Result(handle)
	if err != nil {
		t.Fatalf("Result = %v", err)
	}
	if value.Value["result"] != "DONE" {
		t.Fatalf("structured output = %+v", value.Value)
	}
	changed, ok := value.Value["changed"].([]any)
	if !ok || len(changed) != 1 {
		t.Fatalf("changed = %+v", value.Value["changed"])
	}
	// The stream stays on disk, so a failed session can be read afterwards.
	saved, err := os.ReadFile(filepath.Join(worktree, ".komodo", "sessions", string(handle)+".jsonl"))
	if err != nil || !strings.Contains(string(saved), `"type":"result"`) {
		t.Fatalf("saved stream = %q, %v; the session's stream must be kept", saved, err)
	}
}

// TestStreamClosesTheSessionsLogFiles proves the stream and stderr log files a session opens are
// closed once it ends, instead of left open for the mount's whole lifetime.
func TestStreamClosesTheSessionsLogFiles(t *testing.T) {
	setupFakeClaude(t)
	root, worktree := t.TempDir(), t.TempDir()
	withLineSettings(t, root)
	m := NewMount(root, worktree, 10, 0)

	handle, err := m.Start(context.Background(), newFakeRequest())
	if err != nil {
		t.Fatalf("Start = %v", err)
	}
	sess, ok := m.get(handle)
	if !ok {
		t.Fatal("no session recorded for the handle")
	}
	if len(sess.logs) == 0 {
		t.Fatal("spawn opened no log files to close")
	}
	drainMountEvents(t, mustStream(t, context.Background(), m, handle))
	for _, closer := range sess.logs {
		file, ok := closer.(*os.File)
		if !ok {
			continue
		}
		if _, err := file.Write([]byte("x")); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("write to %s after the stream ended = %v, want it already closed", file.Name(), err)
		}
	}
}

// TestTheSessionTempRootGoesWithTheLastSession proves a worktree's private temp root is removed
// once its last session ends, so finished worktrees leave nothing in the temp dir.
func TestTheSessionTempRootGoesWithTheLastSession(t *testing.T) {
	setupFakeClaude(t)
	root, worktree := t.TempDir(), t.TempDir()
	withLineSettings(t, root)
	m := NewMount(root, worktree, 10, 0)
	ctx := context.Background()
	first, err := m.Start(ctx, newFakeRequest())
	if err != nil {
		t.Fatalf("Start = %v", err)
	}
	second, err := m.Start(ctx, newFakeRequest())
	if err != nil {
		t.Fatalf("Start = %v", err)
	}
	drainMountEvents(t, mustStream(t, ctx, m, first))
	if _, err := os.Stat(SessionTmp(worktree)); err != nil {
		t.Fatalf("stat = %v; the temp root must stay while a session still runs", err)
	}
	drainMountEvents(t, mustStream(t, ctx, m, second))
	if _, err := os.Stat(SessionTmp(worktree)); !os.IsNotExist(err) {
		t.Fatalf("stat = %v; the last session's end must remove the temp root", err)
	}
}

func TestContractResumePassesResumeWithTheFirstSessionsID(t *testing.T) {
	ctx := context.Background()
	logPath := setupFakeClaude(t)
	root, worktree := t.TempDir(), t.TempDir()
	withLineSettings(t, root)
	m := NewMount(root, worktree, 10, 0)

	first, err := m.Start(ctx, newFakeRequest())
	if err != nil {
		t.Fatalf("Start = %v", err)
	}
	events, err := m.Stream(ctx, first)
	if err != nil {
		t.Fatalf("Stream = %v", err)
	}
	drainMountEvents(t, events)

	resumed, err := m.Resume(ctx, first, "fix the failing test")
	if err != nil {
		t.Fatalf("Resume = %v", err)
	}
	if resumed == first {
		t.Fatal("Resume should hand back a fresh handle")
	}
	resumedEvents, err := m.Stream(ctx, resumed)
	if err != nil {
		t.Fatalf("Stream(resumed) = %v", err)
	}
	got := drainMountEvents(t, resumedEvents)
	if len(got) != 2 || got[1].Turns != 2 {
		t.Fatalf("resumed events = %+v, want the resume fixture's turns", got)
	}

	value, err := m.Result(resumed)
	if err != nil {
		t.Fatalf("Result = %v", err)
	}
	if value.Value["result"] != "DONE" {
		t.Fatalf("resumed structured output = %+v", value.Value)
	}

	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	wants := []string{
		"--session-id " + string(first), "--resume " + string(first), "--fork-session", "--session-id " + string(resumed),
	}
	for _, want := range wants {
		if !strings.Contains(string(log), want) {
			t.Fatalf("argv log missing %q:\n%s", want, log)
		}
	}
}

func TestContractResumesAHandleFromAnEarlierProcess(t *testing.T) {
	ctx := context.Background()
	logPath := setupFakeClaude(t)
	root, worktree := t.TempDir(), t.TempDir()
	withLineSettings(t, root)
	earlier := NewMount(root, worktree, 10, 0)
	first, err := earlier.Start(ctx, newFakeRequest())
	if err != nil {
		t.Fatalf("Start = %v", err)
	}
	events, err := earlier.Stream(ctx, first)
	if err != nil {
		t.Fatalf("Stream = %v", err)
	}
	drainMountEvents(t, events)

	restarted := NewMount(root, worktree, 10, 0)
	resumed, err := restarted.Resume(ctx, first, "fix the failing test")
	if err != nil {
		t.Fatalf("Resume after a restart = %v; the handle is the host's session ID", err)
	}
	drainMountEvents(t, mustStream(t, ctx, restarted, resumed))
	if value, err := restarted.Result(resumed); err != nil || value.Value["result"] != "DONE" {
		t.Fatalf("Result = %+v, %v", value, err)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	argv := string(log)
	if !strings.Contains(argv, "--resume "+string(first)) || !strings.Contains(argv, "--model claude-sonnet-5") {
		t.Fatalf("argv log lacks the earlier session and its request:\n%s", log)
	}
	// Each session keeps its own stream; a new process never overwrites an earlier one's log.
	for _, handle := range []mount.Handle{first, resumed} {
		saved, err := os.ReadFile(filepath.Join(worktree, ".komodo", "sessions", string(handle)+".jsonl"))
		if err != nil || !strings.Contains(string(saved), `"type":"result"`) {
			t.Fatalf("%s's stream = %q, %v", handle, saved, err)
		}
	}
}

func TestContractResumeOfAHandleNoProcessStartedFails(t *testing.T) {
	ctx := context.Background()
	setupFakeClaude(t)
	m := NewMount(t.TempDir(), t.TempDir(), 10, 0)
	if _, err := m.Resume(ctx, mount.Handle("no-such-session"), "input"); err == nil {
		t.Fatal("Resume should fail on a handle with no saved request")
	}
}

// mustStream opens handle's stream on m, failing the test on error.
func mustStream(t *testing.T, ctx context.Context, m *Mount, handle mount.Handle) <-chan mount.Event {
	t.Helper()
	events, err := m.Stream(ctx, handle)
	if err != nil {
		t.Fatalf("Stream = %v", err)
	}
	return events
}

func TestContractResumeWithoutADrainedStreamFails(t *testing.T) {
	ctx := context.Background()
	setupFakeClaude(t)
	root, worktree := t.TempDir(), t.TempDir()
	withLineSettings(t, root)
	m := NewMount(root, worktree, 10, 0)

	handle, err := m.Start(ctx, newFakeRequest())
	if err != nil {
		t.Fatalf("Start = %v", err)
	}
	if _, err := m.Resume(ctx, handle, "input"); err == nil {
		t.Fatal("Resume should fail when the prior session's stream was never drained")
	}
}

func TestContractStopKillsTheProcessGroup(t *testing.T) {
	ctx := context.Background()
	setupFakeClaude(t)
	t.Setenv("FAKE_CLAUDE_HANG", "1")
	root, worktree := t.TempDir(), t.TempDir()
	withLineSettings(t, root)
	m := NewMount(root, worktree, 10, 0)

	handle, err := m.Start(ctx, newFakeRequest())
	if err != nil {
		t.Fatalf("Start = %v", err)
	}
	events, err := m.Stream(ctx, handle)
	if err != nil {
		t.Fatalf("Stream = %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	started := time.Now()
	if err := m.Stop(ctx, handle); err != nil {
		t.Fatalf("Stop = %v", err)
	}
	done := make(chan struct{})
	go func() {
		drainMountEvents(t, events)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream never closed; Stop did not kill the process group")
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("Stop took %s; the group was not killed promptly", time.Since(started))
	}
}

// TestVersionOutputKillsAHungClaudeAndNamesTheTimeout proves claude --version past cliTimeout is
// killed, process group included, and the error names the command and the timeout.
func TestVersionOutputKillsAHungClaudeAndNamesTheTimeout(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\nsleep 30 &\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	saved := cliTimeout
	cliTimeout = 200 * time.Millisecond
	t.Cleanup(func() { cliTimeout = saved })
	started := time.Now()
	if _, err := versionOutput(); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want it to name the timeout", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("the kill took %s; the group was not killed", time.Since(started))
	}
}

func TestContractPreflightPassesOnAnyVersionWithALogin(t *testing.T) {
	ctx := context.Background()
	setupFakeClaude(t)
	for _, version := range []string{"2.1.283 (Claude Code)", "9.9.9 (Claude Code)"} {
		t.Setenv("FAKE_CLAUDE_VERSION", version)
		m := NewMount(t.TempDir(), t.TempDir(), 10, 0)
		if err := m.Preflight(ctx); err != nil {
			t.Fatalf("Preflight on %s = %v", version, err)
		}
	}
}

func TestContractPreflightFailsWhenNotLoggedIn(t *testing.T) {
	ctx := context.Background()
	setupFakeClaude(t)
	t.Setenv("FAKE_CLAUDE_AUTH", `{"loggedIn":false}`)
	m := NewMount(t.TempDir(), t.TempDir(), 10, 0)
	if !errorsIsNotLoggedIn(m.Preflight(ctx)) {
		t.Fatalf("Preflight = %v, want errNotLoggedIn", m.Preflight(ctx))
	}
}

// errorsIsNotLoggedIn reports whether err is the not-logged-in sentinel.
func errorsIsNotLoggedIn(err error) bool { return err == errNotLoggedIn }

func TestContractCapabilitiesDeclaresAllFour(t *testing.T) {
	m := NewMount(t.TempDir(), t.TempDir(), 10, 0)
	caps := m.Capabilities()
	if !caps.Resume || !caps.Sandbox || !caps.Hooks || !caps.Structured {
		t.Fatalf("capabilities = %+v, want every one true", caps)
	}
}

func TestContractStreamOnAnUnknownHandleFails(t *testing.T) {
	m := NewMount(t.TempDir(), t.TempDir(), 10, 0)
	if _, err := m.Stream(context.Background(), mount.Handle("no-such-session")); err == nil {
		t.Fatal("Stream should fail on an unknown handle")
	}
}

func TestContractStopOnAnUnknownHandleFails(t *testing.T) {
	m := NewMount(t.TempDir(), t.TempDir(), 10, 0)
	if err := m.Stop(context.Background(), mount.Handle("no-such-session")); err == nil {
		t.Fatal("Stop should fail on an unknown handle")
	}
}

func TestContractResultOnAnUnknownHandleFails(t *testing.T) {
	m := NewMount(t.TempDir(), t.TempDir(), 10, 0)
	if _, err := m.Result(mount.Handle("no-such-session")); err == nil {
		t.Fatal("Result should fail on an unknown handle")
	}
}

func TestContractStartRunsInTheWorktree(t *testing.T) {
	ctx := context.Background()
	logPath := setupFakeClaude(t)
	root, worktree := t.TempDir(), t.TempDir()
	withLineSettings(t, root)
	m := NewMount(root, worktree, 10, 0)

	handle, err := m.Start(ctx, newFakeRequest())
	if err != nil {
		t.Fatalf("Start = %v", err)
	}
	events, err := m.Stream(ctx, handle)
	if err != nil {
		t.Fatalf("Stream = %v", err)
	}
	drainMountEvents(t, events)

	if _, err := m.Result(handle); err != nil {
		t.Fatalf("Result = %v", err)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	realWorktree, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), realWorktree) {
		t.Fatalf("claude did not run in the worktree %q:\n%s", realWorktree, log)
	}
}

func TestContractStopsASessionWhoseProcessTreeRunsAway(t *testing.T) {
	ctx := context.Background()
	setupFakeClaude(t)
	t.Setenv("FAKE_CLAUDE_FORK", "1")
	sleep := uniqueSleep(53)
	t.Setenv("FAKE_CLAUDE_FORK_SLEEP", sleep)
	saved, savedInterval := proc.DefaultLimits, proc.WatchInterval
	proc.DefaultLimits, proc.WatchInterval = proc.Limits{Procs: 10}, 50*time.Millisecond
	t.Cleanup(func() { proc.DefaultLimits, proc.WatchInterval = saved, savedInterval })
	root := t.TempDir()
	withLineSettings(t, root)
	m := NewMount(root, t.TempDir(), 10, 0)

	handle, err := m.Start(ctx, newFakeRequest())
	if err != nil {
		t.Fatalf("Start = %v", err)
	}
	events, err := m.Stream(ctx, handle)
	if err != nil {
		t.Fatalf("Stream = %v", err)
	}
	drainMountEvents(t, events)
	if _, err := m.Result(handle); err == nil || !strings.Contains(err.Error(), "ran away") {
		t.Fatalf("Result = %v; a session over the process cap must be killed and say so", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		// pgrep exits zero only while a process still matches.
		out := proc.Exec("", 5*time.Second, "pgrep", "-lf", "^sleep "+sleep+"$")
		if !out.OK() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("a runaway session's children outlived it: %s", out.Output)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// uniqueSleep is a sleep duration no other test process uses, so pgrep finds only this test's children.
func uniqueSleep(seconds int) string {
	return fmt.Sprintf("%d.%d", seconds, os.Getpid())
}

func TestContractStartLeavesNoLogFilesWhenSavingTheRequestFails(t *testing.T) {
	setupFakeClaude(t)
	root, worktree := t.TempDir(), t.TempDir()
	m := NewMount(root, worktree, 10, 0)

	const fixedID = "11111111-1111-1111-1111-111111111111"
	old := newSessionID
	newSessionID = func() (string, error) { return fixedID, nil }
	t.Cleanup(func() { newSessionID = old })

	logs := filepath.Join(worktree, ".komodo", "sessions")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	// A directory sitting at the request's own path makes the save fail without touching the log files.
	if err := os.Mkdir(filepath.Join(logs, fixedID+".request.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := m.Start(context.Background(), newFakeRequest()); err == nil {
		t.Fatal("Start should fail when saving the session's request fails")
	}
	if _, err := os.Stat(filepath.Join(logs, fixedID+".jsonl")); err == nil {
		t.Fatal("the stream log must not be created when saving the request fails")
	}
	if _, err := os.Stat(filepath.Join(logs, fixedID+".err")); err == nil {
		t.Fatal("the stderr log must not be created when saving the request fails")
	}
}
