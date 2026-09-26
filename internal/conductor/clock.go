package conductor

import "time"

// Clock tracks time spent in a group and per session, enforcing limits by session type.
type Clock struct {
	groupLimit     time.Duration
	sessionLimits  map[string]time.Duration
	groupUsed      time.Duration
	sessionUsed    map[string]time.Duration
	sessionStarted map[string]time.Time
	sessionType    map[string]string
}

// NewClock creates a clock with a 60-minute group limit and per-session type limits.
func NewClock() *Clock {
	return &Clock{
		groupLimit: 60 * time.Minute,
		sessionLimits: map[string]time.Duration{
			"build":     25 * time.Minute,
			"review":    8 * time.Minute,
			"repair":    10 * time.Minute,
			"re-review": 5 * time.Minute,
		},
		sessionUsed:    make(map[string]time.Duration),
		sessionStarted: make(map[string]time.Time),
		sessionType:    make(map[string]string),
	}
}

// StartSession records a session's start time under its own ID, apart from any other session of its type.
func (c *Clock) StartSession(id, sessionType string) {
	c.sessionStarted[id] = time.Now()
	c.sessionType[id] = sessionType
}

// EndSession adds a session's elapsed time to its own total and the group's, and stops timing it.
func (c *Clock) EndSession(id string) {
	start, ok := c.sessionStarted[id]
	if !ok {
		return
	}
	elapsed := time.Since(start)
	c.sessionUsed[id] += elapsed
	c.groupUsed += elapsed
	delete(c.sessionStarted, id)
}

// SessionPastLimit reports whether a session, still running or already ended, is past its type's limit.
func (c *Clock) SessionPastLimit(id string) bool {
	sessionType, ok := c.sessionType[id]
	if !ok {
		return false
	}
	limit, ok := c.sessionLimits[sessionType]
	if !ok {
		return false
	}
	return c.elapsed(id) > limit
}

// GroupPastLimit reports whether the group, counting every session still running, is past its
// 60-minute limit.
func (c *Clock) GroupPastLimit() bool {
	return c.GroupUsed() > c.groupLimit
}

// GroupUsed returns the group's total time, its ended sessions plus every session still running.
func (c *Clock) GroupUsed() time.Duration {
	total := c.groupUsed
	for _, start := range c.sessionStarted {
		total += time.Since(start)
	}
	return total
}

// SessionUsed returns a session's own elapsed time, counting time still running.
func (c *Clock) SessionUsed(id string) time.Duration {
	return c.elapsed(id)
}

// elapsed is a session's accumulated time plus, while it is still running, time since its start.
func (c *Clock) elapsed(id string) time.Duration {
	total := c.sessionUsed[id]
	if start, running := c.sessionStarted[id]; running {
		total += time.Since(start)
	}
	return total
}
