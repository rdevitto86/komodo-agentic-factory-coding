//go:build windows

package proc

import "os/exec"

// Group is a no-op on this platform; there is no process-group signal to arm.
func Group(command *exec.Cmd) {}

// KillGroup kills the process itself; this platform has no group signal to reach its children.
func KillGroup(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	_ = command.Process.Kill()
}
