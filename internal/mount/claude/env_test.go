package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/mount"
)

// lookup reads one key back out of an environment list.
func lookup(env []string, key string) (string, bool) {
	for _, entry := range env {
		if name, value, found := strings.Cut(entry, "="); found && name == key {
			return value, true
		}
	}
	return "", false
}

func TestAScrubbedSessionEnvironmentHoldsNoForgeToken(t *testing.T) {
	const token = "ghp_forgetoken"
	base := []string{
		"GH_TOKEN=" + token, "GITHUB_TOKEN=" + token, "GH_ENTERPRISE_TOKEN=" + token, "GITHUB_PAT=" + token,
		"GITLAB_TOKEN=" + token, "BITBUCKET_PASSWORD=" + token, "HOMEBREW_GITHUB_API_TOKEN=" + token,
		"GIT_ASKPASS=/bin/askpass", "SSH_AUTH_SOCK=/tmp/agent.sock", "GIT_CONFIG_PARAMETERS='credential.helper'=store",
		"PATH=/usr/bin",
	}
	scrubbed := scrubEnv(base)
	for _, entry := range scrubbed {
		if strings.Contains(entry, token) {
			t.Fatalf("%s survived the scrub", entry)
		}
	}
	for _, key := range []string{"GIT_ASKPASS", "SSH_AUTH_SOCK", "GIT_CONFIG_PARAMETERS"} {
		if _, found := lookup(scrubbed, key); found {
			t.Fatalf("%s survived the scrub", key)
		}
	}
	if path, _ := lookup(scrubbed, "PATH"); path != "/usr/bin" {
		t.Fatalf("PATH = %q, want the inherited value", path)
	}
}

func TestALaunchedSessionStartsFromAScrubbedEnvironment(t *testing.T) {
	const token = "ghp_forgetoken"
	t.Setenv("GH_TOKEN", token)
	t.Setenv("GITLAB_TOKEN", token)
	t.Setenv(subprocessScrubEnv, "1")
	_, env, _ := Session("/repo", "/worktree", mount.StartRequest{Role: "builder"}, "", "", "sonnet", "", 10, 0)
	for _, entry := range env {
		if strings.Contains(entry, token) {
			t.Fatalf("%s reached the session", entry)
		}
	}
	if _, found := lookup(env, subprocessScrubEnv); found {
		t.Fatalf("%s reached the session", subprocessScrubEnv)
	}
	if prompt, _ := lookup(env, "GIT_TERMINAL_PROMPT"); prompt != "0" {
		t.Fatalf("GIT_TERMINAL_PROMPT = %q; the session's git may prompt for a credential", prompt)
	}
}

func TestAScrubbedSessionNeverSetsTheHostsOwnSubprocessScrub(t *testing.T) {
	scrubbed := scrubEnv([]string{subprocessScrubEnv + "=1", "PATH=/usr/bin"})
	if _, found := lookup(scrubbed, subprocessScrubEnv); found {
		t.Fatalf("%s survived; it keeps the forge token and overrides dontAsk", subprocessScrubEnv)
	}
}

func TestAScrubbedSessionKeepsTheHostsOwnLogin(t *testing.T) {
	base := []string{"CLAUDE_CODE_OAUTH_TOKEN=login", "ANTHROPIC_API_KEY=key", "PATH=/usr/bin"}
	scrubbed := scrubEnv(base)
	for _, key := range []string{"CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_API_KEY"} {
		if _, found := lookup(scrubbed, key); !found {
			t.Fatalf("%s was scrubbed; only a forge's push credential may be", key)
		}
	}
}

func TestAScrubbedSessionLeavesGitAndGhUnableToAuthenticate(t *testing.T) {
	scrubbed := scrubEnv([]string{
		"GIT_TERMINAL_PROMPT=1", "GIT_SSH_COMMAND=ssh -i /home/me/.ssh/id_ed25519", "GH_CONFIG_DIR=/home/me/.config/gh",
	})
	want := map[string]string{
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_CONFIG_COUNT":    "1",
		"GIT_CONFIG_KEY_0":    "credential.helper",
		"GIT_CONFIG_VALUE_0":  "",
	}
	for key, value := range want {
		if got, found := lookup(scrubbed, key); !found || got != value {
			t.Fatalf("%s = %q, %v; want %q", key, got, found, value)
		}
	}
	ssh, _ := lookup(scrubbed, "GIT_SSH_COMMAND")
	if strings.Contains(ssh, "id_ed25519") || !strings.Contains(ssh, "-F "+os.DevNull) {
		t.Fatalf("GIT_SSH_COMMAND = %q", ssh)
	}
	if gh, _ := lookup(scrubbed, "GH_CONFIG_DIR"); gh == "/home/me/.config/gh" {
		t.Fatal("GH_CONFIG_DIR still points at gh's real config")
	}
	count := 0
	for _, entry := range scrubbed {
		if strings.HasPrefix(entry, "GIT_TERMINAL_PROMPT=") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("GIT_TERMINAL_PROMPT appears %d times", count)
	}
}

func TestTheCredentialPathsCoverGitsStoreAndGhsConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("GH_CONFIG_DIR", "")
	paths := credentialPaths()
	for _, want := range []string{
		filepath.Join(home, ".git-credentials"),
		filepath.Join(home, ".config", "git", "credentials"),
		filepath.Join(home, ".config", "gh"),
	} {
		if !containsPath(paths, want) {
			t.Fatalf("paths = %v, missing %s", paths, want)
		}
	}
}

func TestTheCredentialPathsFollowTheEnvironmentsOwnConfigDirectories(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config := t.TempDir()
	gh := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("GH_CONFIG_DIR", gh)
	paths := credentialPaths()
	for _, want := range []string{filepath.Join(config, "git", "credentials"), filepath.Join(config, "gh"), gh} {
		if !containsPath(paths, want) {
			t.Fatalf("paths = %v, missing %s", paths, want)
		}
	}
}

// containsPath reports whether the list holds the path.
func containsPath(paths []string, path string) bool {
	for _, item := range paths {
		if item == path {
			return true
		}
	}
	return false
}
