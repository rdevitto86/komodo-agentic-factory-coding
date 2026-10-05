package proc

import (
	"fmt"
	"sync"
	"time"
)

// SessionWall is the longest one model session may run; past it the session is a failed run.
const SessionWall = 30 * time.Minute

// SessionIdle is the longest a session may go without a byte of output before it counts as stuck.
const SessionIdle = 10 * time.Minute

// BeatInterval is how often a heartbeat checks its clocks.
var BeatInterval = 5 * time.Second

// Heartbeat kills a session that outlives its wall clock or stays silent past its idle limit.
type Heartbeat struct {
	kill func()
	wall time.Duration
	idle time.Duration

	mu     sync.Mutex
	last   time.Time
	breach string
	stop   chan struct{}
	done   chan struct{}
}

// Beat starts a heartbeat that calls kill once when wall or idle runs out; a zero limit is off.
func Beat(kill func(), wall, idle time.Duration) *Heartbeat {
	h := &Heartbeat{kill: kill, wall: wall, idle: idle, last: time.Now(),
		stop: make(chan struct{}), done: make(chan struct{})}
	go h.loop(time.Now())
	return h
}

// Write counts any output as a pulse, so a heartbeat can sit in an io.MultiWriter.
func (h *Heartbeat) Write(p []byte) (int, error) {
	h.mu.Lock()
	h.last = time.Now()
	h.mu.Unlock()
	return len(p), nil
}

// loop checks both clocks every BeatInterval until Stop or a breach.
func (h *Heartbeat) loop(started time.Time) {
	defer close(h.done)
	ticker := time.NewTicker(BeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-h.stop:
			return
		case now := <-ticker.C:
			h.mu.Lock()
			quiet := now.Sub(h.last)
			over := ""
			switch {
			case h.wall > 0 && now.Sub(started) > h.wall:
				over = fmt.Sprintf("ran past its %s wall clock", h.wall)
			case h.idle > 0 && quiet > h.idle:
				over = fmt.Sprintf("went silent for %s", quiet.Round(time.Second))
			}
			h.breach = over
			h.mu.Unlock()
			if over != "" {
				h.kill()
				return
			}
		}
	}
}

// Breach names the limit the session broke, or is empty when it never did.
func (h *Heartbeat) Breach() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.breach
}

// Stop ends the checking.
func (h *Heartbeat) Stop() {
	select {
	case <-h.stop:
	default:
		close(h.stop)
	}
	<-h.done
}
