//go:build unix

package run

import (
	"os/exec"
	"syscall"
)

// setProcessGroup starts the command in its own process group, so a kill can reach its children too.
func setProcessGroup(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup kills every process in the command's group, not just the host itself.
func killProcessGroup(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
}
