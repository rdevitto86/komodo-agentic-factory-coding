package claude

import (
	"os"
	"path/filepath"
	"strings"

	"komodo/internal/harness"
)

// subprocessScrubEnv is never set: it leaves the forge token in the shell tool and overrides dontAsk.
const subprocessScrubEnv = "CLAUDE_CODE_SUBPROCESS_ENV_SCRUB"

// scrubEnv returns base with every forge credential removed, by harness.Scrub's one pattern match,
// and this host's own subprocess-scrub override dropped too.
func scrubEnv(base []string) []string {
	filtered := make([]string, 0, len(base))
	for _, entry := range base {
		if key, _, found := strings.Cut(entry, "="); found && key == subprocessScrubEnv {
			continue
		}
		filtered = append(filtered, entry)
	}
	return harness.Scrub(filtered)
}

// credentialPaths are the files and directories holding a forge token that a session's sandbox denies reads of:
// git's credential store and gh's config, wherever this environment puts them.
func credentialPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	paths := []string{
		filepath.Join(home, ".git-credentials"),
		filepath.Join(config, "git", "credentials"),
		filepath.Join(config, "gh"),
	}
	if gh := os.Getenv("GH_CONFIG_DIR"); gh != "" && gh != paths[2] {
		paths = append(paths, gh)
	}
	return paths
}
