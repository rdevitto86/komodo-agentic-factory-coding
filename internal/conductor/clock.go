package conductor

import "time"

// Clock tracks time spent in a group and per session type, enforcing limits.
type Clock struct {
	groupLimit      time.Duration
	sessionLimits   map[string]time.Duration
	groupUsed       time.Duration
	sessionUsed     map[string]time.Duration
	sessionStarted  map[string]time.Time
}

// NewClock creates a clock with a 60-minute group limit and per-session type limits.
func NewClock() *Clock {
	return &Clock{
		groupLimit: 60 * time.Minute,
		sessionLimits: map[string]time.Duration{
			"build":    25 * time.Minute,
			"review":   8 * time.Minute,
			"repair":   10 * time.Minute,
			"re-review": 5 * time.Minute,
		},
		sessionUsed:    make(map[string]time.Duration),
		sessionStarted: make(map[string]time.Time),
	}
}

// StartSession records the start time of a session by type.
func (c *Clock) StartSession(sessionType string) {
	c.sessionStarted[sessionType] = time.Now()
}

// EndSession records time used for a session by type.
func (c *Clock) EndSession(sessionType string) {
	if start, ok := c.sessionStarted[sessionType]; ok {
		elapsed := time.Since(start)
		c.sessionUsed[sessionType] += elapsed
		c.groupUsed += elapsed
		delete(c.sessionStarted, sessionType)
	}
}

// SessionPastLimit reports whether a session type has exceeded its limit.
func (c *Clock) SessionPastLimit(sessionType string) bool {
	limit, ok := c.sessionLimits[sessionType]
	if !ok {
		return false
	}
	return c.sessionUsed[sessionType] > limit
}

// GroupPastLimit reports whether the group has exceeded its 60-minute limit.
func (c *Clock) GroupPastLimit() bool {
	return c.groupUsed > c.groupLimit
}

// GroupUsed returns the total time used by the group.
func (c *Clock) GroupUsed() time.Duration {
	return c.groupUsed
}

// SessionUsed returns the total time used by a session type.
func (c *Clock) SessionUsed(sessionType string) time.Duration {
	return c.sessionUsed[sessionType]
}
