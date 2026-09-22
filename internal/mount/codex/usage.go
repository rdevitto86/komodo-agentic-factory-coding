package codex

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"komodo/internal/mount"
)

// EventsPath is where the launcher writes this host's JSON events for one task.
func EventsPath(root, task string) string {
	return filepath.Join(root, ".komodo", "codex", task+".jsonl")
}

// event is the only part of this host's JSON stream the mount decodes.
type event struct {
	Type  string `json:"type"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Usage reads the events the launcher captured, and returns nothing in a session.
func Usage(root, task string, _, _ time.Time) (mount.TaskUsage, bool) {
	handle, err := os.Open(EventsPath(root, task))
	if err != nil {
		return mount.TaskUsage{}, false
	}
	defer handle.Close()
	var usage mount.TaskUsage
	scanner := bufio.NewScanner(handle)
	scanner.Buffer(make([]byte, 0, 64*1024), 8<<20)
	for scanner.Scan() {
		var parsed event
		if json.Unmarshal(scanner.Bytes(), &parsed) != nil {
			continue
		}
		if parsed.Usage.InputTokens == 0 && parsed.Usage.OutputTokens == 0 {
			continue
		}
		usage.TokensIn += parsed.Usage.InputTokens
		usage.TokensOut += parsed.Usage.OutputTokens
		usage.Turns++
	}
	if usage.Turns == 0 {
		return mount.TaskUsage{}, false
	}
	return usage, true
}
