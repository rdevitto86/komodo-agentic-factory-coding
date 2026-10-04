package claude

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"komodo/internal/mount"
	"komodo/internal/proc"
)

// cliTimeout bounds how long a claude status command may run before it is killed; a test lowers it.
var cliTimeout = 30 * time.Second

// waitDelay bounds how long a killed command's output pipes may stay open after it exits.
const waitDelay = 5 * time.Second

// errNotLoggedIn reports that the CLI's auth status holds no login.
var errNotLoggedIn = errors.New("claude is not logged in")

// versionOutput runs this host's own --version report, or an error naming the command and its stderr.
var versionOutput = func() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cliTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "claude", "--version")
	proc.Group(cmd)
	cmd.Cancel = func() error {
		proc.KillGroup(cmd)
		return nil
	}
	cmd.WaitDelay = waitDelay
	stdout, stderr := proc.NewBoundedWriter(proc.MaxOutput), proc.NewBoundedWriter(proc.MaxOutput)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	proc.KillGroup(cmd)
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("claude --version: timed out after %s: %s", cliTimeout, strings.TrimSpace(stderr.String()))
	}
	if err != nil {
		return "", fmt.Errorf("claude --version: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Mount runs Claude Code sessions and implements the host contract.
type Mount struct {
	root, worktree string
	maxTurns       int
	maxBudgetUSD   float64

	mu       sync.Mutex
	sessions map[mount.Handle]*session
}

// session is one running or finished claude process, and what Result and Resume read once it ends.
type session struct {
	cmd    *exec.Cmd
	buf    *bytes.Buffer
	reader io.Reader
	req    mount.StartRequest
	// logs are the session's own stream and stderr files, closed once it ends.
	logs []io.Closer
	// watcher stops the session's process tree past proc.DefaultLimits and reaps what it leaves behind.
	watcher *proc.Watcher

	waitOnce sync.Once
	waitErr  error

	done chan struct{}

	mu        sync.Mutex
	streamed  bool
	sessionID string
	result    mount.Result
	resultErr error
}

// wait calls the process's Wait exactly once, so Stream's drain and a concurrent Stop never race on it.
func (s *session) wait() error {
	s.waitOnce.Do(func() { s.waitErr = s.cmd.Wait() })
	return s.waitErr
}

// closeLogs closes every log file spawn opened; a failed close is not actionable once the
// session has already produced its result.
func (s *session) closeLogs() {
	for _, closer := range s.logs {
		_ = closer.Close()
	}
}

// NewMount builds a Claude Code mount that starts every session in worktree, with root naming the
// repo its plugins and settings render into, capped at maxTurns and, when positive, maxBudgetUSD.
func NewMount(root, worktree string, maxTurns int, maxBudgetUSD float64) *Mount {
	return &Mount{
		root: root, worktree: worktree, maxTurns: maxTurns, maxBudgetUSD: maxBudgetUSD, sessions: map[mount.Handle]*session{},
	}
}

// Preflight checks that the installed CLI runs and holds a login, or names which one fails.
func (m *Mount) Preflight(ctx context.Context) error {
	if _, err := versionOutput(); err != nil {
		return fmt.Errorf("checking claude --version: %w", err)
	}
	loggedIn, err := LoggedIn()
	if err != nil {
		return fmt.Errorf("checking claude auth status: %w", err)
	}
	if !loggedIn {
		return errNotLoggedIn
	}
	return nil
}

// Start runs a role headless with Session's argv and environment, in the worktree, in its own
// process group, and returns the handle its session runs under: the host's own session ID.
func (m *Mount) Start(ctx context.Context, req mount.StartRequest) (mount.Handle, error) {
	argv, env, prompt, err := Session(m.root, m.worktree, req, "", "", req.Model, req.Effort, m.maxTurns, m.maxBudgetUSD)
	if err != nil {
		return "", err
	}
	return m.spawn(ctx, argv, env, prompt, req)
}

// Resume forks a finished session with new input under a fresh session ID; a handle from an earlier
// process resumes from the request its spawn saved.
func (m *Mount) Resume(ctx context.Context, handle mount.Handle, input string) (mount.Handle, error) {
	req, err := m.priorRequest(handle)
	if err != nil {
		return "", err
	}
	argv, env, prompt, err := Session(m.root, m.worktree, req, handle, input, req.Model, req.Effort, m.maxTurns, m.maxBudgetUSD)
	if err != nil {
		return "", err
	}
	return m.spawn(ctx, append(argv, "--fork-session"), env, prompt, req)
}

// priorRequest returns the request handle's session ran under, once that session has ended with a result.
func (m *Mount) priorRequest(handle mount.Handle) (mount.StartRequest, error) {
	prior, ok := m.get(handle)
	if !ok {
		data, err := os.ReadFile(m.requestPath(handle))
		if err != nil {
			return mount.StartRequest{}, fmt.Errorf("resume: unknown session %q: %w", handle, err)
		}
		var req mount.StartRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return mount.StartRequest{}, fmt.Errorf("resume: reading %q's request: %w", handle, err)
		}
		return req, nil
	}
	prior.mu.Lock()
	streamed := prior.streamed
	prior.mu.Unlock()
	if !streamed {
		return mount.StartRequest{}, fmt.Errorf("resume: %q has not been streamed to completion", handle)
	}
	<-prior.done
	prior.mu.Lock()
	sessionID := prior.sessionID
	prior.mu.Unlock()
	if sessionID == "" {
		return mount.StartRequest{}, fmt.Errorf("resume: %q ended with no session id to resume", handle)
	}
	return prior.req, nil
}

// requestPath is where a session's request is kept, so a later process can resume it.
func (m *Mount) requestPath(handle mount.Handle) string {
	return filepath.Join(m.worktree, ".komodo", "sessions", string(handle)+".request.json")
}

// newSessionID returns a random version 4 UUID, the form the host's --session-id accepts; a test swaps it.
var newSessionID = func() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// spawn starts claude with argv and env, its prompt on stdin, under a fresh session ID that is its handle;
// a cancelled ctx kills a start that never returns.
func (m *Mount) spawn(ctx context.Context, argv, env []string, prompt string, req mount.StartRequest) (mount.Handle, error) {
	sessionID, err := newSessionID()
	if err != nil {
		return "", fmt.Errorf("making a session id: %w", err)
	}
	handle := mount.Handle(sessionID)
	cmd := exec.CommandContext(ctx, "claude", append(argv, "--session-id", sessionID)...)
	cmd.Dir = m.worktree
	cmd.Env = env
	cmd.Stdin = strings.NewReader(prompt)
	proc.Group(cmd)
	// The session's temp root must exist and be private before the host puts its TMPDIR there.
	if err := os.MkdirAll(SessionTmp(m.worktree), 0o700); err != nil {
		return "", fmt.Errorf("making the session's temp root: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("piping claude's stdout: %w", err)
	}
	logs := filepath.Join(m.worktree, ".komodo", "sessions")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		return "", fmt.Errorf("making the session's log directory: %w", err)
	}
	saved, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("saving the session's request: %w", err)
	}
	// The request is saved before the log files open, so a failed save never leaks their handles.
	if err := os.WriteFile(m.requestPath(handle), saved, 0o600); err != nil {
		return "", fmt.Errorf("saving the session's request: %w", err)
	}
	// Each session's stream and errors stay on disk, so a failed session can be diagnosed afterwards.
	var record io.Writer = io.Discard
	var logFiles []io.Closer
	if out, err := os.Create(filepath.Join(logs, string(handle)+".jsonl")); err == nil {
		record, logFiles = out, append(logFiles, out)
	}
	if errs, err := os.Create(filepath.Join(logs, string(handle)+".err")); err == nil {
		cmd.Stderr, logFiles = errs, append(logFiles, errs)
	}
	if err := cmd.Start(); err != nil {
		for _, file := range logFiles {
			file.Close()
		}
		return "", fmt.Errorf("starting claude: %w", err)
	}

	buf := &bytes.Buffer{}
	sess := &session{cmd: cmd, buf: buf, reader: io.TeeReader(stdout, io.MultiWriter(buf, record)), req: req, done: make(chan struct{}),
		logs: logFiles, watcher: proc.Watch(cmd.Process.Pid, proc.DefaultLimits)}

	m.mu.Lock()
	m.sessions[handle] = sess
	m.mu.Unlock()
	return handle, nil
}

// Stream parses stdout with Parse and reports the session's turns, usage, cost and rate limits as
// they happen; once the stream ends, it waits for claude to exit and readies Result and Resume.
func (m *Mount) Stream(ctx context.Context, handle mount.Handle) (<-chan mount.Event, error) {
	sess, ok := m.get(handle)
	if !ok {
		return nil, fmt.Errorf("stream: unknown session %q", handle)
	}
	sess.mu.Lock()
	sess.streamed = true
	sess.mu.Unlock()
	out := make(chan mount.Event)
	go func() {
		defer close(out)
		for event := range Parse(ctx, sess.reader) {
			out <- event
		}
		waitErr := sess.wait()
		// Nothing the session started may outlive it; a tree that ran away is the session's error.
		proc.KillGroup(sess.cmd)
		sess.watcher.Stop()
		sess.closeLogs()
		if breach := sess.watcher.Breach(); breach != "" {
			waitErr = fmt.Errorf("the session's process tree ran away (%s) and was killed", breach)
		}
		output, sessionID := structuredResult(sess.buf.Bytes())
		sess.mu.Lock()
		sess.sessionID = sessionID
		sess.result = mount.Result{Value: output}
		sess.resultErr = waitErr
		sess.mu.Unlock()
		close(sess.done)
	}()
	return out, nil
}

// Result returns the result event's structured_output, once the session's stream has ended.
func (m *Mount) Result(handle mount.Handle) (mount.Result, error) {
	sess, ok := m.get(handle)
	if !ok {
		return mount.Result{}, fmt.Errorf("result: unknown session %q", handle)
	}
	<-sess.done
	sess.mu.Lock()
	defer sess.mu.Unlock()
	return sess.result, sess.resultErr
}

// Stop kills the session's whole process group.
func (m *Mount) Stop(ctx context.Context, handle mount.Handle) error {
	sess, ok := m.get(handle)
	if !ok {
		return fmt.Errorf("stop: unknown session %q", handle)
	}
	proc.KillGroup(sess.cmd)
	go func() { _ = sess.wait() }()
	return nil
}

// Capabilities declares this mount's four: resume, sandbox, hooks and structured output.
func (m *Mount) Capabilities() mount.Capabilities {
	return mount.Capabilities{Resume: true, Sandbox: true, Hooks: true, Structured: true}
}

// get returns the session recorded under handle.
func (m *Mount) get(handle mount.Handle) (*session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[handle]
	return sess, ok
}

// structuredResult reads the last result line in raw for its structured_output and session ID.
func structuredResult(raw []byte) (map[string]any, string) {
	var output map[string]any
	var sessionID string
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), streamLineCap)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var kind kindLine
		if json.Unmarshal(line, &kind) != nil || kind.Type != "result" {
			continue
		}
		var parsed struct {
			StructuredOutput map[string]any `json:"structured_output"`
			SessionID        string         `json:"session_id"`
		}
		if json.Unmarshal(line, &parsed) == nil {
			output, sessionID = parsed.StructuredOutput, parsed.SessionID
		}
	}
	return output, sessionID
}
