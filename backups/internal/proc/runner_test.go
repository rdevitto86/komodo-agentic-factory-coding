package proc

import (
	"context"
	"testing"
	"time"
)

// TestDefaultRunnerExecRunsARealProcess proves DefaultRunner.Exec is Exec itself.
func TestDefaultRunnerExecRunsARealProcess(t *testing.T) {
	got := DefaultRunner.Exec(".", time.Second, "echo", "hi")
	if !got.OK() || got.Output != "hi" {
		t.Fatalf("Exec = %+v", got)
	}
}

// TestDefaultRunnerExecContextRunsARealProcess proves DefaultRunner.ExecContext is ExecContext itself.
func TestDefaultRunnerExecContextRunsARealProcess(t *testing.T) {
	got := DefaultRunner.ExecContext(context.Background(), ".", time.Second, "echo", "hi")
	if !got.OK() || got.Output != "hi" {
		t.Fatalf("ExecContext = %+v", got)
	}
}

// TestDefaultRunnerShellRunsARealProcess proves DefaultRunner.Shell is Shell itself.
func TestDefaultRunnerShellRunsARealProcess(t *testing.T) {
	got := DefaultRunner.Shell(".", "echo hi", time.Second)
	if !got.OK() || got.Output != "hi" {
		t.Fatalf("Shell = %+v", got)
	}
}

// TestFakeRunnerAnswersInCallOrder proves a FakeRunner answers each call from Results in order,
// across all three methods, and records every call it answered.
func TestFakeRunnerAnswersInCallOrder(t *testing.T) {
	fake := &FakeRunner{Results: []Result{{ExitCode: 0, Output: "first"}, {ExitCode: 1, Output: "second"}}}
	first := fake.Exec("dir", time.Second, "git", "status")
	second := fake.Shell("dir", "git push", time.Second)
	if first.Output != "first" || second.ExitCode != 1 || second.Output != "second" {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	calls := fake.Calls()
	if len(calls) != 2 || calls[0].Name != "git" || calls[0].Args[0] != "status" {
		t.Fatalf("calls = %+v", calls)
	}
}

// TestFakeRunnerAnswersZeroPastResults proves a call past the end of Results gets a zero Result
// instead of a panic.
func TestFakeRunnerAnswersZeroPastResults(t *testing.T) {
	fake := &FakeRunner{}
	got := fake.Exec("dir", time.Second, "git", "status")
	if got != (Result{}) {
		t.Fatalf("Exec past Results = %+v, want a zero Result", got)
	}
}

// TestFakeRunnerExecContextHonoursACancelledContext proves ExecContext answers a cancelled
// context's error instead of recording a call, since nothing it would kill ever started.
func TestFakeRunnerExecContextHonoursACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake := &FakeRunner{Results: []Result{{ExitCode: 0, Output: "unused"}}}
	got := fake.ExecContext(ctx, "dir", time.Second, "git", "status")
	if got.OK() {
		t.Fatalf("ExecContext on a cancelled context = %+v, want a failure", got)
	}
	if len(fake.Calls()) != 0 {
		t.Fatalf("Calls = %v, want none recorded", fake.Calls())
	}
}
