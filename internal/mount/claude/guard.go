package claude

import (
	"encoding/json"
	"path/filepath"

	"komodo/internal/mount"
)

// guardWriteTools are this host's tool names that write a file.
var guardWriteTools = map[string]bool{"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true}

// guardPathFields are the tool_input keys a write tool's path arrives in.
var guardPathFields = []string{"file_path", "notebook_path"}

// init registers this host's tool names and denial encoding, so the guard never names it.
func init() {
	mount.RegisterGuard("claude", mount.GuardTools{
		WriteTools:   guardWriteTools,
		PathFields:   guardPathFields,
		ShellTool:    "Bash",
		CommandField: "command",
		ConfigPaths:  []string{filepath.ToSlash(filepath.Join(Dir, "settings.json"))},
		Deny:         denyPayload,
	})
}

// denyPayload renders the JSON on stdout this host reads as a PreToolUse denial.
func denyPayload(reason string) []byte {
	payload := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		},
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return out
}
