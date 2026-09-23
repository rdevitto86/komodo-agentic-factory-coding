//go:build unix

package proc

import (
	"os/exec"
	"syscall"
)

// Group starts the command in its own process group, so a kill can reach its children too.
func Group(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// KillGroup kills every process in the command's group, not just the process itself.
func KillGroup(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
}
