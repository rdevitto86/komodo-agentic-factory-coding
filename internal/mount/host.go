package mount

import (
	"sync"
	"time"
)

// Handle names one session a mount is running, which Stream, Result, Stop and Resume use to find it again.
type Handle string

// StartRequest is what Start needs to run a role headless: its brief, tools, hooks, permissions, model and effort.
type StartRequest struct {
	Role        string
	Brief       string
	Tools       []string
	Hooks       []string
	Permissions []string
	Model       string
	Effort      string
	Schema      []byte
}

// RateLimit is one host account's five-hour and seven-day utilisation and when it next resets.
type RateLimit struct {
	FiveHour float64
	SevenDay float64
	ResetsAt time.Time
}

// Event is one line a running session's stream reports: a turn's usage and cost, or a rate limit.
type Event struct {
	Turns     int
	Usage     TaskUsage
	CostUSD   float64
	RateLimit *RateLimit
}

// liveRateLimit is the freshest rate_limit_event any session reported, pace's only live source.
var (
	liveRateLimitMu sync.Mutex
	liveRateLimit   *RateLimit
)

// ObserveRateLimit records a session's live rate_limit_event, the newest one always wins.
func ObserveRateLimit(limit RateLimit) {
	liveRateLimitMu.Lock()
	defer liveRateLimitMu.Unlock()
	stored := limit
	liveRateLimit = &stored
}

// LatestRateLimit is the freshest rate_limit_event a session reported, or false when none arrived yet.
func LatestRateLimit() (RateLimit, bool) {
	liveRateLimitMu.Lock()
	defer liveRateLimitMu.Unlock()
	if liveRateLimit == nil {
		return RateLimit{}, false
	}
	return *liveRateLimit, true
}

// ClearRateLimit forgets any observed rate_limit_event, which a test calls to isolate itself.
func ClearRateLimit() {
	liveRateLimitMu.Lock()
	defer liveRateLimitMu.Unlock()
	liveRateLimit = nil
}

// Result is a session's final answer: the JSON its role's schema checked.
type Result struct {
	Value map[string]any
}

// Capabilities are the four things a mount's sessions may do (decision 0004).
type Capabilities struct {
	Resume     bool
	Sandbox    bool
	Hooks      bool
	Structured bool
}

// Contract is the host contract every mount implements (decision 0004).
type Contract interface {
	// Preflight reports whether the host is installed, pinned and logged in, or the fix when not.
	Preflight() error
	// Start runs a role headless and returns the handle its session runs under.
	Start(req StartRequest) (Handle, error)
	// Resume continues a stopped session with new input, under the same or a fresh handle.
	Resume(handle Handle, input string) (Handle, error)
	// Stream reports a running session's turns, usage, cost and rate limits as they happen.
	Stream(handle Handle) (<-chan Event, error)
	// Result returns a finished session's schema-checked JSON.
	Result(handle Handle) (Result, error)
	// Stop ends a session and its whole process tree.
	Stop(handle Handle) error
	// Capabilities declares what this mount's sessions can do.
	Capabilities() Capabilities
}
