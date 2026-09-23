//go:build windows

package run

import "os/exec"

// setProcessGroup is a no-op on this platform; there is no process-group signal to arm.
func setProcessGroup(command *exec.Cmd) {}

// killProcessGroup kills the host process; this platform has no group signal to reach its children.
func killProcessGroup(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	_ = command.Process.Kill()
}
