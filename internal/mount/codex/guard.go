package codex

import (
	"path/filepath"

	"komodo/internal/mount"
)

// init registers this host's shell tool and denial encoding, so the guard never names it.
// Its file-editing tool is not registered yet: the guard cannot decode its patch body.
func init() {
	mount.RegisterGuard("codex", mount.GuardTools{
		ShellTool:    guardShellTool,
		CommandField: "command",
		ConfigPaths:  []string{filepath.ToSlash(filepath.Join(Dir, "hooks.json"))},
		Deny:         denyPayload,
	})
}

// denyPayload returns nil: this host reads a denial from the exit code and stderr, not a
// JSON payload on stdout.
func denyPayload(string) []byte {
	return nil
}

// guardShellTool is the tool name this host reports for a shell call, which the hook matches.
const guardShellTool = "Bash"
