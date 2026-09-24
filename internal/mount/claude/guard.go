package claude

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"

	"komodo/internal/mount"
)

// shellTool is this host's tool name for a shell command.
const shellTool = "Bash"

// guardWriteTools are this host's tool names that write a file.
var guardWriteTools = map[string]bool{"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true}

// guardPathFields are the tool_input keys a write tool's path arrives in.
var guardPathFields = []string{"file_path", "notebook_path"}

// guardSpawnTools are this host's tool names that start a second agent session.
var guardSpawnTools = map[string]bool{"Task": true}

// isolationField is the tool_input key a spawn call uses to ask for a separate worktree.
const isolationField = "isolation"

// init registers this host's tool names and denial encoding, so the guard never names it.
func init() {
	mount.RegisterGuard("claude", mount.GuardTools{
		WriteTools:     guardWriteTools,
		PathFields:     guardPathFields,
		ShellTool:      shellTool,
		CommandField:   "command",
		SpawnTools:     guardSpawnTools,
		IsolationField: isolationField,
		ConfigPaths:    []string{filepath.ToSlash(filepath.Join(Dir, "settings.json"))},
		Deny:           denyPayload,
	})
}

// hookMatcher joins the guard's own registered tool names, so the hook never keeps a second list.
func hookMatcher() string {
	names := []string{shellTool}
	for name := range guardWriteTools {
		names = append(names, name)
	}
	for name := range guardSpawnTools {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, "|")
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
