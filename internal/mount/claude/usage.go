package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/fs"
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

// Usage sums the spawned agents' transcripts that were handed the named brief, never the
// parent's, and reports nothing when no spawned transcript names it.
func Usage(root, task string, since, until time.Time) (mount.TaskUsage, bool) {
	dir := TranscriptDir(root)
	if dir == "" {
		return mount.TaskUsage{}, false
	}
	marker := []byte("briefs/" + task + ".md")
	var usage mount.TaskUsage
	matched := 0
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") || !isSpawned(dir, path) {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().Before(since) {
			return nil
		}
		if !mentions(path, marker) {
			return nil
		}
		counted, ok := sumTranscript(path, since, until)
		if !ok || counted.Turns == 0 {
			return nil
		}
		usage.TokensIn += counted.TokensIn
		usage.TokensOut += counted.TokensOut
		usage.TokensCached += counted.TokensCached
		usage.Turns += counted.Turns
		matched++
		return nil
	})
	if matched == 0 {
		return mount.TaskUsage{}, false
	}
	return usage, true
}

// isSpawned reports whether a transcript sits below a session directory, which is where this
// host writes a spawned agent's transcript, as opposed to the session's own file beside it.
func isSpawned(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && strings.Contains(filepath.ToSlash(rel), "/")
}

// mentions reports whether a transcript carries the marker anywhere, reading it as bytes only.
func mentions(path string, marker []byte) bool {
	data, err := os.ReadFile(path)
	return err == nil && bytes.Contains(data, marker)
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
		usage.TokensIn += counts.InputTokens + counts.CacheCreationTokens
		usage.TokensCached += counts.CacheReadTokens
		usage.TokensOut += counts.OutputTokens
		usage.Turns++
	}
	return usage, scanner.Err() == nil
}
