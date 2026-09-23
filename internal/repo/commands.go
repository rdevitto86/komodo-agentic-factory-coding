package repo

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// CommandsFile is where a repo overrides the shell command each station runs.
const CommandsFile = ".komodo/commands.json"

// Commands are the shell commands a repo may override at each station.
type Commands struct {
	Verify       string `json:"verify"`
	Compile      string `json:"compile"`
	BeforeReview string `json:"before_review"`
	AfterPublish string `json:"after_publish"`
}

// LoadCommands reads a repo's commands file, which may be absent.
func LoadCommands(root string) Commands {
	var commands Commands
	data, err := os.ReadFile(filepath.Join(root, CommandsFile))
	if err != nil {
		return commands
	}
	_ = json.Unmarshal(data, &commands)
	return commands
}
