//go:build windows

package proc

import (
	"os/exec"
	"sync"
	"syscall"
)

// kernel32 holds the job object calls this file needs; the standard library has no higher-level wrapper for them.
var kernel32 = syscall.NewLazyDLL("kernel32.dll")

var (
	procCreateJobObjectW         = kernel32.NewProc("CreateJobObjectW")
	procAssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject       = kernel32.NewProc("TerminateJobObject")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
)

// processTerminate and processSetQuota are the access rights AssignProcessToJobObject needs on a process handle.
const (
	processTerminate = 0x0001
	processSetQuota  = 0x0100
)

// jobEntry is one command's job object: a handle set once at creation, and whether its process has joined it.
type jobEntry struct {
	handle   syscall.Handle
	assigned bool
}

// jobsMu guards jobs and each entry's assigned field; its handle is immutable, so any goroutine may read it.
var (
	jobsMu sync.Mutex
	jobs   = map[*exec.Cmd]*jobEntry{}
)

// Group gives command its own job object, so a later kill reaches every process it starts, not just itself.
func Group(command *exec.Cmd) {
	handle, _, _ := procCreateJobObjectW.Call(0, 0)
	if handle == 0 {
		return
	}
	jobsMu.Lock()
	jobs[command] = &jobEntry{handle: syscall.Handle(handle)}
	jobsMu.Unlock()
}

// Started puts command's now-running process into its job object, so anything it spawns from here on
// inherits membership; Windows can only join a job once the process exists, unlike Unix's atomic fork.
func Started(command *exec.Cmd) {
	jobsMu.Lock()
	entry := jobs[command]
	jobsMu.Unlock()
	if entry != nil {
		joinJob(entry, command)
	}
}

// joinJob assigns command's process to entry's job, once; it claims the attempt before making the
// syscalls, so a concurrent kill never retries it or races it for the same entry.
func joinJob(entry *jobEntry, command *exec.Cmd) {
	jobsMu.Lock()
	if entry.assigned || command.Process == nil {
		jobsMu.Unlock()
		return
	}
	entry.assigned = true
	jobsMu.Unlock()
	handle, _, _ := procOpenProcess.Call(uintptr(processTerminate|processSetQuota), 0, uintptr(command.Process.Pid))
	if handle == 0 {
		return
	}
	defer procCloseHandle.Call(handle)
	procAssignProcessToJobObject.Call(uintptr(entry.handle), handle)
}

// KillGroup terminates every process command's job object holds; a command with no job, or whose
// process never joined one, is killed on its own instead.
func KillGroup(command *exec.Cmd) {
	jobsMu.Lock()
	entry := jobs[command]
	delete(jobs, command)
	jobsMu.Unlock()
	if entry != nil {
		// A caller that skipped Started, such as one driving Run, still gets a late, best-effort join.
		joinJob(entry, command)
		procTerminateJobObject.Call(uintptr(entry.handle), 1)
		procCloseHandle.Call(uintptr(entry.handle))
	}
	if command.Process != nil {
		_ = command.Process.Kill()
	}
}
