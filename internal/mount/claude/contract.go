package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"komodo/internal/mount"
	"komodo/internal/proc"
)

// errNotLoggedIn reports that the CLI's auth status holds no login.
var errNotLoggedIn = errors.New("claude is not logged in")

// versionOutput runs this host's own --version report; a test swaps it.
var versionOutput = func() (string, error) {
	out, err := exec.Command("claude", "--version").Output()
	return strings.TrimSpace(string(out)), err
}

// Mount runs Claude Code sessions and implements the host contract (decision 0004).
type Mount struct {
	root, worktree string
	maxTurns       int
	maxBudgetUSD   float64

	mu       sync.Mutex
	sessions map[mount.Handle]*session
	counter  int64
	// prefix is unique per mount, so a restarted process never reuses, or overwrites the log of, an earlier handle.
	prefix string
}

// session is one running or finished claude process, and what Result and Resume read once it ends.
type session struct {
	cmd    *exec.Cmd
	buf    *bytes.Buffer
	reader io.Reader
	req    mount.StartRequest

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

// NewMount builds a Claude Code mount that starts every session in worktree, with root naming the
// repo its plugins and settings render into, capped at maxTurns and, when positive, maxBudgetUSD.
func NewMount(root, worktree string, maxTurns int, maxBudgetUSD float64) *Mount {
	return &Mount{
		root: root, worktree: worktree, maxTurns: maxTurns, maxBudgetUSD: maxBudgetUSD, sessions: map[mount.Handle]*session{},
		prefix: strconv.FormatInt(time.Now().UnixNano(), 36),
	}
}

// Preflight checks the installed CLI's pinned version and its login, or names which one fails.
func (m *Mount) Preflight() error {
	installed, err := versionOutput()
	if err != nil {
		return fmt.Errorf("checking claude --version: %w", err)
	}
	if !strings.Contains(installed, HostVersion) {
		return fmt.Errorf("claude reports %s; the line is pinned to %s", installed, HostVersion)
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
// process group, and returns the handle its session runs under.
func (m *Mount) Start(req mount.StartRequest) (mount.Handle, error) {
	argv, env, prompt := Session(m.root, m.worktree, req, "", "", req.Model, req.Effort, m.maxTurns, m.maxBudgetUSD)
	return m.spawn(argv, env, prompt, req)
}

// Resume continues a finished session with new input, passing --resume with its session ID.
func (m *Mount) Resume(handle mount.Handle, input string) (mount.Handle, error) {
	prior, ok := m.get(handle)
	if !ok {
		return "", fmt.Errorf("resume: unknown session %q", handle)
	}
	prior.mu.Lock()
	streamed := prior.streamed
	prior.mu.Unlock()
	if !streamed {
		return "", fmt.Errorf("resume: %q has not been streamed to completion", handle)
	}
	<-prior.done
	prior.mu.Lock()
	sessionID := prior.sessionID
	prior.mu.Unlock()
	if sessionID == "" {
		return "", fmt.Errorf("resume: %q ended with no session id to resume", handle)
	}
	req := prior.req
	argv, env, prompt := Session(m.root, m.worktree, req, mount.Handle(sessionID), input, req.Model, req.Effort, m.maxTurns, m.maxBudgetUSD)
	return m.spawn(argv, env, prompt, req)
}

// spawn starts claude with argv and env, its prompt on stdin, and records the session under a fresh handle.
func (m *Mount) spawn(argv, env []string, prompt string, req mount.StartRequest) (mount.Handle, error) {
	cmd := exec.Command("claude", argv...)
	cmd.Dir = m.worktree
	cmd.Env = env
	cmd.Stdin = strings.NewReader(prompt)
	proc.Group(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("piping claude's stdout: %w", err)
	}
	m.mu.Lock()
	m.counter++
	handle := mount.Handle(fmt.Sprintf("claude-%s-%d", m.prefix, m.counter))
	m.mu.Unlock()
	// Each session's stream and errors stay on disk, so a failed session can be diagnosed afterwards.
	logs := filepath.Join(m.worktree, ".komodo", "sessions")
	var record io.Writer = io.Discard
	if err := os.MkdirAll(logs, 0o755); err == nil {
		if out, err := os.Create(filepath.Join(logs, string(handle)+".jsonl")); err == nil {
			record = out
		}
		if errs, err := os.Create(filepath.Join(logs, string(handle)+".err")); err == nil {
			cmd.Stderr = errs
		}
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("starting claude: %w", err)
	}

	buf := &bytes.Buffer{}
	sess := &session{cmd: cmd, buf: buf, reader: io.TeeReader(stdout, io.MultiWriter(buf, record)), req: req, done: make(chan struct{})}

	m.mu.Lock()
	m.sessions[handle] = sess
	m.mu.Unlock()
	return handle, nil
}

// Stream parses stdout with Parse and reports the session's turns, usage, cost and rate limits as
// they happen; once the stream ends, it waits for claude to exit and readies Result and Resume.
func (m *Mount) Stream(handle mount.Handle) (<-chan mount.Event, error) {
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
		for event := range Parse(sess.reader) {
			out <- mount.Event{Turns: event.Turns, Usage: event.Usage, CostUSD: event.CostUSD, RateLimit: event.RateLimit}
		}
		waitErr := sess.wait()
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
func (m *Mount) Stop(handle mount.Handle) error {
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
