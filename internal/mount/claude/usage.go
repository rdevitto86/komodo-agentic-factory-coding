package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/mount"
)

// projectsDir is where this host writes a transcript per session, one directory per repo.
const projectsDir = "projects"

// entry is the only part of a transcript this mount decodes: the counts, never the text.
type entry struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Message   struct {
		Usage struct {
			InputTokens         int `json:"input_tokens"`
			OutputTokens        int `json:"output_tokens"`
			CacheReadTokens     int `json:"cache_read_input_tokens"`
			CacheCreationTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// slug is the directory name this host gives a repo's transcripts.
func slug(root string) string {
	clean := filepath.ToSlash(filepath.Clean(root))
	return strings.ReplaceAll(strings.ReplaceAll(clean, "/", "-"), ":", "-")
}

// TranscriptDir is where this host keeps the transcripts for one repo.
func TranscriptDir(root string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", projectsDir, slug(root))
}

// Usage reports one task's spend, and reports nothing when more than one session wrote in the window.
func Usage(root, task string, since, until time.Time) (mount.TaskUsage, bool) {
	dir := TranscriptDir(root)
	if dir == "" {
		return mount.TaskUsage{}, false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return mount.TaskUsage{}, false
	}
	var usage mount.TaskUsage
	sessions := 0
	for _, item := range entries {
		if item.IsDir() || !strings.HasSuffix(item.Name(), ".jsonl") {
			continue
		}
		info, err := item.Info()
		if err != nil || info.ModTime().Before(since) {
			continue
		}
		counted, ok := sumTranscript(filepath.Join(dir, item.Name()), since, until)
		if !ok {
			continue
		}
		usage = counted
		sessions++
	}
	if sessions != 1 || usage.Turns == 0 {
		return mount.TaskUsage{}, false
	}
	return usage, true
}

// sumTranscript reads one transcript for counts only; no message text is copied anywhere.
func sumTranscript(path string, since, until time.Time) (mount.TaskUsage, bool) {
	handle, err := os.Open(path)
	if err != nil {
		return mount.TaskUsage{}, false
	}
	defer handle.Close()
	var usage mount.TaskUsage
	scanner := bufio.NewScanner(handle)
	scanner.Buffer(make([]byte, 0, 64*1024), 8<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var parsed entry
		if json.Unmarshal(line, &parsed) != nil || parsed.Type != "assistant" {
			continue
		}
		if !parsed.Timestamp.IsZero() {
			if parsed.Timestamp.Before(since) || (!until.IsZero() && parsed.Timestamp.After(until)) {
				continue
			}
		}
		counts := parsed.Message.Usage
		usage.TokensIn += counts.InputTokens + counts.CacheReadTokens + counts.CacheCreationTokens
		usage.TokensOut += counts.OutputTokens
		usage.Turns++
	}
	return usage, scanner.Err() == nil
}
