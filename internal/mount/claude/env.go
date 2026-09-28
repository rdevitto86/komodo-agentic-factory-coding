package claude

import (
	"os"
	"path/filepath"
	"strings"
)

// subprocessScrubEnv is never set: it leaves the forge token in the shell tool and overrides dontAsk.
const subprocessScrubEnv = "CLAUDE_CODE_SUBPROCESS_ENV_SCRUB"

// forgeEnv are the environment variables that would hand a session a push credential.
var forgeEnv = []string{
	"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GIT_ASKPASS", "SSH_AUTH_SOCK",
	"GIT_CONFIG_PARAMETERS", subprocessScrubEnv,
}

// forgeWords and secretWords are the name parts that together mark a push credential.
var (
	forgeWords  = map[string]bool{"GIT": true, "GITHUB": true, "GH": true, "GITLAB": true, "GL": true, "BITBUCKET": true}
	secretWords = map[string]bool{"TOKEN": true, "PAT": true, "SECRET": true, "PASSWORD": true, "KEY": true}
)

// scrubEnv returns base with every forge credential removed, git unable to prompt or authenticate,
// and gh pointed at an empty config directory.
func scrubEnv(base []string) []string {
	out := make([]string, 0, len(base)+6)
	for _, entry := range base {
		key, _, found := strings.Cut(entry, "=")
		if !found || forgeKey(key) || scrubOverride(key) {
			continue
		}
		out = append(out, entry)
	}
	return append(out,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=credential.helper",
		"GIT_CONFIG_VALUE_0=",
		"GIT_SSH_COMMAND=ssh -F "+os.DevNull+" -o BatchMode=yes -o IdentitiesOnly=yes -o IdentityFile="+os.DevNull,
		"GH_CONFIG_DIR="+filepath.Join(os.TempDir(), "komodo-gh-noauth"),
	)
}

// forgeKey reports whether a key names a push credential, exactly or by its shape, such as GITLAB_TOKEN;
// the host's own login, which names no forge, survives.
func forgeKey(key string) bool {
	for _, name := range forgeEnv {
		if key == name {
			return true
		}
	}
	forge, secret := false, false
	for _, part := range strings.Split(key, "_") {
		forge = forge || forgeWords[part]
		secret = secret || secretWords[part]
	}
	return forge && secret
}

// scrubOverride reports whether scrubEnv sets this key itself, so an inherited value never survives.
func scrubOverride(key string) bool {
	switch key {
	case "GIT_TERMINAL_PROMPT", "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0",
		"GIT_CONFIG_VALUE_0", "GIT_SSH_COMMAND", "GH_CONFIG_DIR":
		return true
	}
	return false
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
