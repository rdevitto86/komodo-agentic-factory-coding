package mount

import (
	"errors"
	"testing"
	"time"
)

// fakeSession is one run a fakeHost is driving: its handle, its events and its result.
type fakeSession struct {
	events  []Event
	result  Result
	stopped bool
}

// fakeHost is the fake mount every conductor test drives, canned instead of a real host.
type fakeHost struct {
	preflightErr error
	capabilities Capabilities
	sessions     map[Handle]*fakeSession
	next         int
}

// newFakeHost returns a fake host with no sessions and every capability declared.
func newFakeHost() *fakeHost {
	return &fakeHost{
		capabilities: Capabilities{Resume: true, Sandbox: true, Hooks: true, Structured: true},
		sessions:     map[Handle]*fakeSession{},
	}
}

func (f *fakeHost) Preflight() error { return f.preflightErr }

func (f *fakeHost) Start(req StartRequest) (Handle, error) {
	f.next++
	handle := Handle(req.Role)
	f.sessions[handle] = &fakeSession{
		events: []Event{{Turns: 1, Usage: TaskUsage{TokensIn: 10, TokensOut: 5, Turns: 1}, CostUSD: 0.01}},
		result: Result{Value: map[string]any{"result": "DONE"}},
	}
	return handle, nil
}

func (f *fakeHost) Resume(handle Handle, input string) (Handle, error) {
	session, ok := f.sessions[handle]
	if !ok {
		return "", errors.New("no such session")
	}
	session.events = append(session.events, Event{Turns: 1, Usage: TaskUsage{Turns: 1}, CostUSD: 0.01})
	session.result = Result{Value: map[string]any{"result": "DONE", "input": input}}
	return handle, nil
}

func (f *fakeHost) Stream(handle Handle) (<-chan Event, error) {
	session, ok := f.sessions[handle]
	if !ok {
		return nil, errors.New("no such session")
	}
	out := make(chan Event, len(session.events))
	for _, event := range session.events {
		out <- event
	}
	close(out)
	return out, nil
}

func (f *fakeHost) Result(handle Handle) (Result, error) {
	session, ok := f.sessions[handle]
	if !ok {
		return Result{}, errors.New("no such session")
	}
	return session.result, nil
}

func (f *fakeHost) Stop(handle Handle) error {
	session, ok := f.sessions[handle]
	if !ok {
		return errors.New("no such session")
	}
	session.stopped = true
	return nil
}

func (f *fakeHost) Capabilities() Capabilities { return f.capabilities }

// asContract confirms the type satisfies the interface every mount implements.
var _ Contract = (*fakeHost)(nil)

func TestFakeHostDrivesTheWholeContract(t *testing.T) {
	host := newFakeHost()
	if err := host.Preflight(); err != nil {
		t.Fatalf("preflight = %v", err)
	}
	got := host.Capabilities()
	want := Capabilities{Resume: true, Sandbox: true, Hooks: true, Structured: true}
	if got != want {
		t.Fatalf("capabilities = %+v, want %+v", got, want)
	}

	handle, err := host.Start(StartRequest{Role: "builder", Model: "sonnet", Effort: "medium"})
	if err != nil {
		t.Fatalf("start = %v", err)
	}

	events, err := host.Stream(handle)
	if err != nil {
		t.Fatalf("stream = %v", err)
	}
	var turns int
	for event := range events {
		turns += event.Turns
	}
	if turns != 1 {
		t.Fatalf("turns = %d, want 1", turns)
	}

	result, err := host.Result(handle)
	if err != nil || result.Value["result"] != "DONE" {
		t.Fatalf("result = %+v, %v", result, err)
	}

	resumed, err := host.Resume(handle, "fix the lint error")
	if err != nil {
		t.Fatalf("resume = %v", err)
	}
	result, err = host.Result(resumed)
	if err != nil || result.Value["input"] != "fix the lint error" {
		t.Fatalf("resumed result = %+v, %v", result, err)
	}

	if err := host.Stop(resumed); err != nil {
		t.Fatalf("stop = %v", err)
	}
	if !host.sessions[resumed].stopped {
		t.Fatal("stop never marked the session stopped")
	}
}

func TestFakeHostReportsAPreflightFailure(t *testing.T) {
	host := newFakeHost()
	host.preflightErr = errors.New("not logged in")
	if err := host.Preflight(); err == nil {
		t.Fatal("a failed login passed preflight")
	}
}

func TestFakeHostRefusesAnUnknownHandle(t *testing.T) {
	host := newFakeHost()
	if _, err := host.Stream("missing"); err == nil {
		t.Fatal("streaming an unknown handle succeeded")
	}
	if _, err := host.Result("missing"); err == nil {
		t.Fatal("reading the result of an unknown handle succeeded")
	}
	if err := host.Stop("missing"); err == nil {
		t.Fatal("stopping an unknown handle succeeded")
	}
	if _, err := host.Resume("missing", "input"); err == nil {
		t.Fatal("resuming an unknown handle succeeded")
	}
}

func TestAMountMayDeclareNoCapabilities(t *testing.T) {
	var zero Capabilities
	host := &fakeHost{capabilities: zero, sessions: map[Handle]*fakeSession{}}
	if got := host.Capabilities(); got != zero {
		t.Fatalf("capabilities = %+v, want the zero value", got)
	}
}

func TestRateLimitCarriesFiveHourSevenDayAndReset(t *testing.T) {
	reset := time.Now().Add(time.Hour)
	limit := RateLimit{FiveHour: 0.4, SevenDay: 0.1, ResetsAt: reset}
	event := Event{RateLimit: &limit}
	if event.RateLimit.FiveHour != 0.4 || event.RateLimit.SevenDay != 0.1 || !event.RateLimit.ResetsAt.Equal(reset) {
		t.Fatalf("rate limit = %+v", event.RateLimit)
	}
}
