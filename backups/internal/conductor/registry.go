package conductor

import (
	"sync"
	"time"

	"komodo/internal/profile"
)

// BuilderState is one registry entry's place in its own lifecycle.
type BuilderState string

// The states a registry entry may hold.
const (
	Running     BuilderState = "running"
	Stalled     BuilderState = "stalled"
	RegReviewed BuilderState = "reviewing"
	RegBlocked  BuilderState = "blocked"
	SpunDown    BuilderState = "spunDown"
	RegDone     BuilderState = "done"
)

// reviewerStrikeLimit is how many rounds of reviewer pushback on one builder block it permanently.
const reviewerStrikeLimit = 3

// Clock is the time source a Registry reads, so a test drives its goroutines with no real sleep.
type Clock interface {
	// Now is the clock's current time.
	Now() time.Time
	// After returns a channel that fires once d has passed on the clock's own time.
	After(d time.Duration) <-chan time.Time
}

// realClock is the Clock a live Registry runs on: the OS wall clock.
type realClock struct{}

// Now returns time.Now.
func (realClock) Now() time.Time { return time.Now() }

// After returns time.After.
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// FakeClock is a Clock a test advances by hand, firing every waiter Advance now covers, no sleep.
type FakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []fakeWaiter
}

// fakeWaiter is one call to FakeClock.After still waiting for its due time.
type fakeWaiter struct {
	due time.Time
	ch  chan time.Time
}

// NewFakeClock starts a FakeClock at now.
func NewFakeClock(now time.Time) *FakeClock {
	return &FakeClock{now: now}
}

// Now returns the clock's current, manually set time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// After returns a channel that fires once Advance moves the clock d past now.
func (c *FakeClock) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	c.mu.Lock()
	c.waiters = append(c.waiters, fakeWaiter{due: c.now.Add(d), ch: ch})
	c.mu.Unlock()
	return ch
}

// Advance moves the clock forward by d, firing every waiter now due.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var pending []fakeWaiter
	for _, waiter := range c.waiters {
		if !waiter.due.After(c.now) {
			waiter.ch <- c.now
		} else {
			pending = append(pending, waiter)
		}
	}
	c.waiters = pending
	c.mu.Unlock()
}

// RegistryEntry is one session's whole record: who it is, when it started, and how it has fared.
type RegistryEntry struct {
	BuilderID    string       `json:"builder_id"`
	Group        string       `json:"group"`
	SessionID    string       `json:"session_id"`
	Role         string       `json:"role"`
	StartedAt    time.Time    `json:"started_at"`
	WindowStart  time.Time    `json:"window_start"`
	LastActivity time.Time    `json:"last_activity"`
	Retries      int          `json:"retries"`
	Strikes      int          `json:"strikes"`
	State        BuilderState `json:"state"`
}

// defaultRegistryTick is each entry's own clock-check interval, unless its Registry sets its own.
const defaultRegistryTick = 20 * time.Millisecond

// Registry tracks every builder and reviewer mechanically, one goroutine per entry watching it.
type Registry struct {
	clock Clock
	// tick is how often each entry's goroutine checks its clocks; zero means defaultRegistryTick.
	tick time.Duration
	// Nudge is called once, with the entry's BuilderID, the moment an entry goes idle; nil skips it.
	Nudge func(id string)
	// Kill is called, with the entry's BuilderID, once a window or an idle grace runs out; nil skips it.
	Kill func(id string)

	mu      sync.Mutex
	entries map[string]*RegistryEntry
	stop    map[string]chan struct{}
}

// NewRegistry builds an empty Registry on clock, or the real wall clock when clock is nil.
func NewRegistry(clock Clock) *Registry {
	if clock == nil {
		clock = realClock{}
	}
	return &Registry{clock: clock, entries: map[string]*RegistryEntry{}, stop: map[string]chan struct{}{}}
}

// interval is this registry's own tick, or defaultRegistryTick when it names none.
func (r *Registry) interval() time.Duration {
	if r.tick > 0 {
		return r.tick
	}
	return defaultRegistryTick
}

// Register files a fresh entry for id, starts its ticking goroutine, and returns the entry's own
// copy; an id already registered is replaced, its old goroutine stopped first.
func (r *Registry) Register(id, group, sessionID, role string) RegistryEntry {
	r.mu.Lock()
	r.stopLocked(id)
	now := r.clock.Now()
	entry := &RegistryEntry{
		BuilderID: id, Group: group, SessionID: sessionID, Role: role,
		StartedAt: now, WindowStart: now, LastActivity: now, State: Running,
	}
	r.entries[id] = entry
	stop := make(chan struct{})
	r.stop[id] = stop
	r.mu.Unlock()
	ready := make(chan struct{})
	go r.watch(id, stop, ready)
	<-ready
	return *entry
}

// stopLocked stops id's goroutine and drops its stop channel; the caller must already hold r.mu.
func (r *Registry) stopLocked(id string) {
	if stop, ok := r.stop[id]; ok {
		close(stop)
		delete(r.stop, id)
	}
}

// Resumed records id's latest session handle, so a later Nudge or Kill always reaches the live session.
func (r *Registry) Resumed(id, sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.entries[id]; ok {
		entry.SessionID = sessionID
	}
}

// Touch records id's activity at the clock's now, waking a stalled entry back to running.
func (r *Registry) Touch(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[id]
	if !ok {
		return
	}
	entry.LastActivity = r.clock.Now()
	if entry.State == Stalled || entry.State == RegReviewed {
		entry.State = Running
	}
}

// Pushback resumes id's session after a reviewer's finding, resetting its window; it reports
// blocked once Retries reaches the role's limit, spinning the entry down instead.
func (r *Registry) Pushback(id string) (blocked bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[id]
	if !ok {
		return false
	}
	entry.Retries++
	now := r.clock.Now()
	entry.WindowStart, entry.LastActivity = now, now
	limit := profile.LimitsFor(entry.Role).Retries
	if limit > 0 && entry.Retries >= limit {
		entry.State = SpunDown
		return true
	}
	entry.State = Running
	return false
}

// Strike counts one reviewer's round of pushback against id's builder; three strikes block the
// builder permanently, a state only a person clears.
func (r *Registry) Strike(id string) (blocked bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[id]
	if !ok {
		return false
	}
	entry.Strikes++
	if entry.Strikes >= reviewerStrikeLimit {
		entry.State = RegBlocked
		return true
	}
	return false
}

// Release stops id's goroutine and drops its entry, freeing it for a fresh Register.
func (r *Registry) Release(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked(id)
	delete(r.entries, id)
}

// Done marks id's session finished and stops its goroutine, keeping the entry in Snapshot.
func (r *Registry) Done(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked(id)
	if entry, ok := r.entries[id]; ok {
		entry.State = RegDone
	}
}

// Snapshot returns every entry's own copy, keyed by BuilderID, for persistence into state.json.
func (r *Registry) Snapshot() map[string]RegistryEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]RegistryEntry, len(r.entries))
	for id, entry := range r.entries {
		out[id] = *entry
	}
	return out
}

// Load restores entries from a prior Snapshot and starts each one's goroutine again, so a resumed
// run keeps every window, idle clock and strike count it had before the restart.
func (r *Registry) Load(entries map[string]RegistryEntry) {
	for id, entry := range entries {
		copied := entry
		r.mu.Lock()
		r.stopLocked(id)
		r.entries[id] = &copied
		stop := make(chan struct{})
		r.stop[id] = stop
		r.mu.Unlock()
		switch copied.State {
		case Running, Stalled, RegReviewed:
			ready := make(chan struct{})
			go r.watch(id, stop, ready)
			<-ready
		}
	}
}

// watch enforces one entry's window, idle and grace until stop closes, firing each breach once;
// ready, when set, fires once this goroutine has registered its first tick, so no caller races it.
func (r *Registry) watch(id string, stop <-chan struct{}, ready chan<- struct{}) {
	var windowFired, idleNudged, graceFired bool
	var sawWindowStart, sawActivity time.Time
	tick := r.clock.After(r.interval())
	if ready != nil {
		ready <- struct{}{}
	}
	for {
		select {
		case <-stop:
			return
		case <-tick:
		}
		tick = r.clock.After(r.interval())
		r.mu.Lock()
		entry, ok := r.entries[id]
		if !ok {
			r.mu.Unlock()
			return
		}
		if !entry.WindowStart.Equal(sawWindowStart) {
			sawWindowStart, windowFired = entry.WindowStart, false
		}
		if !entry.LastActivity.Equal(sawActivity) {
			sawActivity, idleNudged, graceFired = entry.LastActivity, false, false
		}
		limits := profile.LimitsFor(entry.Role)
		now := r.clock.Now()
		idleFor := now.Sub(entry.LastActivity)
		switch {
		case limits.Window > 0 && now.Sub(entry.WindowStart) > limits.Window && !windowFired:
			windowFired = true
			entry.State = Stalled
			r.mu.Unlock()
			r.fire(r.Kill, id)
			continue
		case limits.Idle > 0 && idleFor > limits.Idle && !idleNudged:
			idleNudged = true
			entry.State = Stalled
			r.mu.Unlock()
			r.fire(r.Nudge, id)
			continue
		case limits.Idle > 0 && limits.Grace > 0 && idleFor > limits.Idle+limits.Grace && !graceFired:
			graceFired = true
			entry.State = SpunDown
			r.mu.Unlock()
			r.fire(r.Kill, id)
			continue
		}
		r.mu.Unlock()
	}
}

// fire calls hook with id when hook is set.
func (r *Registry) fire(hook func(string), id string) {
	if hook != nil {
		hook(id)
	}
}
