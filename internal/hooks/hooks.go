// Package hooks is every agent hook but the guard: one table of jobs, stages and limits, and one entry
// point that fails open.
package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"komodo/internal/guard"
)

// Event is the stage of a session a hook runs at.
type Event string

// The stages a hook can run at.
const (
	PreToolUse   Event = "PreToolUse"
	PostToolUse  Event = "PostToolUse"
	Stop         Event = "Stop"
	SessionStart Event = "SessionStart"
)

// Session names the role of the sessions a hook is mounted in.
type Session string

// The session kinds a hook can be mounted in; SessionEvery is every session, mounted once for all.
const (
	SessionEvery   Session = "every"
	SessionBuilder Session = "builder"
	SessionLens    Session = "lens"
)

// Tools is which tool calls a tool-stage hook runs after.
type Tools string

// The tool scopes a hook can match.
const (
	AnyTool   Tools = "any"
	EditTools Tools = "edit"
)

// Failure is what a hook does when the hook itself errors: both let the call through.
type Failure string

// The two ways a hook fails open.
const (
	AllowAndLog Failure = "allow"
	Skip        Failure = "skip"
)

// Verdict is what a hook concluded about one call.
type Verdict string

// The three verdicts: let it through, refuse it, or let it through with context for the model.
const (
	Allow  Verdict = "allow"
	Refuse Verdict = "refuse"
	Inform Verdict = "inform"
)

// ExitRefuse is the exit code a host reads as a refusal when no encoder renders the outcome.
const ExitRefuse = 2

// Outcome is a hook's verdict and the message the model reads with it.
type Outcome struct {
	Verdict Verdict
	Message string
}

// Budget is a session's wall clock and turn cap, zero when unknown.
type Budget struct {
	Minutes int
	Turns   int
}

// Input is one hook call: the host's payload, the worktree it runs in, and the session's budget.
type Input struct {
	SessionID string         `json:"session_id"`
	Cwd       string         `json:"cwd"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
	Root      string         `json:"-"`
	Budget    Budget         `json:"-"`
}

// Hook is one row of the table: one job, one stage, the sessions it runs in, and its limits.
type Hook struct {
	Name     string
	Job      string
	Event    Event
	Tools    Tools
	Sessions []Session
	// Limit is the refusals one session gets before the hook allows; zero never refuses.
	Limit     int
	Timeout   time.Duration
	OnFailure Failure
	run       func(ctx context.Context, in Input) (Outcome, error)
}

// Encoder renders an outcome as the JSON a host reads for one stage, or nil for the exit-code fallback.
type Encoder func(event Event, out Outcome) []byte

// encoders are the registered hosts' outcome renderers, by mount name, under encoderLock.
var (
	encoderLock sync.Mutex
	encoders    = map[string]Encoder{}
)

// ErrUnknownHook is a hook name the table does not hold.
var ErrUnknownHook = errors.New("no such hook")

// errNoRunner is a table row this entry point does not serve.
var errNoRunner = errors.New("served by its own command")

// RegisterEncoder records how one mount's host reads a hook's outcome.
func RegisterEncoder(host string, encode Encoder) {
	encoderLock.Lock()
	defer encoderLock.Unlock()
	encoders[host] = encode
}

// encoderFor returns the outcome renderer one mount registered.
func encoderFor(host string) (Encoder, bool) {
	encoderLock.Lock()
	defer encoderLock.Unlock()
	encode, ok := encoders[host]
	return encode, ok
}

// Table is every hook, its session, its limit and its failure behaviour.
func Table() []Hook {
	return []Hook{
		{
			Name: "guard", Job: "the security rules and the builder's file scope",
			Event: PreToolUse, Tools: AnyTool, Sessions: []Session{SessionEvery},
			Limit: 3, Timeout: 30 * time.Second, OnFailure: AllowAndLog,
		},
		{
			Name: "format", Job: "formats the edited file and lints only that file",
			Event: PostToolUse, Tools: EditTools, Sessions: []Session{SessionBuilder},
			Timeout: 2 * time.Minute, OnFailure: Skip, run: formatFile,
		},
		{
			Name: "taskchecks", Job: "the group's checks pass",
			Event: Stop, Sessions: []Session{SessionBuilder},
			Limit: 3, Timeout: 15 * time.Minute, OnFailure: AllowAndLog, run: runTaskChecks,
		},
		{
			Name: "timewarn", Job: "time and turns used",
			Event: PostToolUse, Tools: AnyTool, Sessions: []Session{SessionBuilder, SessionLens},
			Timeout: 10 * time.Second, OnFailure: Skip, run: warnTime,
		},
	}
}

// Lookup returns the table row with this name.
func Lookup(name string) (Hook, bool) {
	for _, hook := range Table() {
		if hook.Name == name {
			return hook, true
		}
	}
	return Hook{}, false
}

// ForSession returns the hooks a session kind mounts itself, leaving out those every session shares.
func ForSession(session Session) []Hook {
	var out []Hook
	for _, hook := range Table() {
		for _, each := range hook.Sessions {
			if each == session {
				out = append(out, hook)
			}
		}
	}
	return out
}

// Dispatch runs one hook on the payload in stdin, writes what the host reads, and returns the exit code.
// Any error, the hook's own included, allows the call.
func Dispatch(root, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	hook, ok := Lookup(name)
	if !ok {
		fmt.Fprintf(stderr, "hook %s: %v; allowing\n", name, ErrUnknownHook)
		return 0
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			failOpen(hook, fmt.Errorf("panic: %v", recovered), stderr)
			code = 0
		}
	}()
	in, host, err := parseInput(root, args, stdin)
	if err != nil {
		return failOpen(hook, err, stderr)
	}
	if hook.run == nil {
		return failOpen(hook, errNoRunner, stderr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), hook.Timeout)
	defer cancel()
	out, err := hook.run(ctx, in)
	if err != nil {
		return failOpen(hook, err, stderr)
	}
	if out.Verdict == Refuse {
		within, err := countRefusal(in.Root, in.SessionID, hook)
		if err != nil {
			return failOpen(hook, err, stderr)
		}
		if !within {
			fmt.Fprintf(stderr, "hook %s: %d refusals reached; allowing\n", hook.Name, hook.Limit)
			return 0
		}
	}
	return write(hook.Event, host, out, stdout, stderr)
}

// parseInput reads the flags and the payload, and resolves the worktree the call runs in.
func parseInput(root string, args []string, stdin io.Reader) (Input, string, error) {
	set := flag.NewFlagSet("hook", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	host := set.String("host", "", "the mount whose host reads the outcome")
	minutes := set.Int("minutes", 0, "the session's wall clock in minutes")
	turns := set.Int("turns", 0, "the session's turn cap")
	if err := set.Parse(args); err != nil {
		return Input{}, "", err
	}
	raw, err := io.ReadAll(stdin)
	if err != nil {
		return Input{}, "", err
	}
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return Input{}, "", fmt.Errorf("unparseable payload: %w", err)
	}

	in.Root = root
	if found := guard.WorktreeRoot(in.Cwd); found != "" {
		in.Root = found
	}
	if in.Cwd == "" {
		in.Cwd = in.Root
	}
	in.Budget = Budget{Minutes: *minutes, Turns: *turns}
	return in, *host, nil
}

// failOpen logs a hook's own error when its row says so and allows the call.
func failOpen(hook Hook, err error, stderr io.Writer) int {
	if hook.OnFailure == AllowAndLog {
		fmt.Fprintf(stderr, "hook %s: %v; allowing\n", hook.Name, err)
	}
	return 0
}

// write renders an outcome through the host's encoder, else as stderr and ExitRefuse for a refusal.
func write(event Event, host string, out Outcome, stdout, stderr io.Writer) int {
	if out.Verdict == Allow || out.Message == "" {
		return 0
	}
	if encode, ok := encoderFor(host); ok {
		if payload := encode(event, out); payload != nil {
			if _, err := stdout.Write(payload); err == nil {
				return 0
			}
		}
	}
	if out.Verdict == Refuse {
		fmt.Fprintln(stderr, out.Message)
		return ExitRefuse
	}
	fmt.Fprintln(stdout, out.Message)
	return 0
}

// countRefusal adds one refusal by hook to the count for sessionID, reporting whether it is within the limit.
func countRefusal(root, sessionID string, hook Hook) (bool, error) {
	if root == "" || sessionID == "" {
		return true, nil
	}
	path := filepath.Join(root, ".komodo", "runs", "hook-refusals", filepath.Base(sessionID)+".json")
	counts := map[string]int{}
	if data, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(data, &counts) != nil {
			counts = map[string]int{}
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	counts[hook.Name]++
	if err := writeState(path, counts); err != nil {
		return false, err
	}
	return counts[hook.Name] <= hook.Limit, nil
}

// writeState writes one hook state file, creating its directory.
func writeState(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
