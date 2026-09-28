package claude

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"komodo/internal/guard"
	"komodo/internal/mount"
)

// Session builds the argv, environment and stdin prompt for starting or resuming a Claude Code
// session headless. The prompt goes on stdin, never argv, past its size and leading-dash limits.
func Session(
	root, worktree string,
	req mount.StartRequest,
	resumed mount.Handle,
	resumeInput string,
	model, effort string,
	maxTurns int,
	maxBudgetUSD float64,
) (argv []string, env []string, prompt string) {
	prompt = req.Brief
	if resumed != "" {
		prompt = resumeInput
	}
	if prompt == "" {
		prompt = "/"
	}

	argv = []string{"-p"}
	argv = append(argv, "--setting-sources", "local")

	pluginDir := filepath.Join(root, Dir, "plugins", req.Role)
	argv = append(argv, "--plugin-dir", pluginDir)

	settings := filepath.Join(root, Dir, "settings.json")
	if sandbox := lineSandbox(mount.LoadOverlay(), runtime.GOOS); sandbox != "" {
		settings = withSandbox(settings, sandbox)
	}
	argv = append(argv, "--settings", settings)

	// With dontAsk, only allowed calls run and a denied one never does, so these rules bound the role.
	allow, deny := rolePermissions(root, worktree, req)
	if verbs := roleTools(root, req); len(verbs) > 0 {
		argv = append(argv, "--tools", toolNames(verbs), "--allowedTools", strings.Join(allow, ", "))
	}
	argv = append(argv, "--disallowedTools", strings.Join(deny, ", "))

	argv = append(argv, "--permission-mode", "dontAsk")
	argv = append(argv, "--model", model)
	// An unset tier effort leaves the host's default, rather than passing an empty value it warns on.
	if effort != "" {
		argv = append(argv, "--effort", effort)
	}
	argv = append(argv, "--strict-mcp-config")
	argv = append(argv, "--output-format", "stream-json")
	argv = append(argv, "--verbose")
	argv = append(argv, "--json-schema", string(req.Schema))

	if resumed != "" {
		argv = append(argv, "--resume", string(resumed))
	}

	if maxBudgetUSD > 0 {
		argv = append(argv, "--max-budget-usd", strconv.FormatFloat(maxBudgetUSD, 'f', 2, 64))
	}

	// The session starts from a scrubbed environment, so no forge credential reaches it.
	env = scrubEnv(os.Environ())
	env = removeEnv(env, "CLAUDE_CONFIG_DIR")
	env = setEnv(env, "CLAUDE_CODE_STOP_HOOK_BLOCK_CAP", "3")
	env = setEnv(env, "CLAUDE_CODE_MAX_TURNS", strconv.Itoa(maxTurns))
	env = setEnv(env, "DISABLE_AUTOUPDATER", "1")
	// The guard hook inherits this, and refuses a line role what it leaves the orchestrator.
	env = setEnv(env, guard.RoleEnv, req.Role)
	// Go's caches live under the worktree's .komodo, which the sandbox allows and a ship never stages.
	goDir := filepath.Join(worktree, ".komodo", "go")
	env = setEnv(env, "GOCACHE", filepath.Join(goDir, "cache"))
	env = setEnv(env, "GOTMPDIR", filepath.Join(goDir, "tmp"))
	env = setEnv(env, "GOPATH", filepath.Join(goDir, "path"))
	env = setEnv(env, "GOMODCACHE", filepath.Join(goDir, "path", "pkg", "mod"))
	env = setEnv(env, "GOPROXY", "off")
	env = setEnv(env, "GOFLAGS", "-modcacherw")
	env = setEnv(env, "CLAUDE_CODE_TMPDIR", SessionTmp(worktree))

	return argv, env, prompt
}

// withSandbox returns the settings file at path merged with the inline sandbox settings as one inline object,
// since the host keeps only the last --settings it is given; an unreadable file leaves the sandbox alone.
func withSandbox(path, sandbox string) string {
	merged := map[string]json.RawMessage{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &merged)
	}
	var overrides map[string]json.RawMessage
	if json.Unmarshal([]byte(sandbox), &overrides) != nil {
		return sandbox
	}
	for key, value := range overrides {
		merged[key] = value
	}
	data, err := json.Marshal(merged)
	if err != nil {
		return sandbox
	}
	return string(data)
}

// SessionTmp is the private temp root a worktree's sessions get, outside any repo, so a test walking up
// from a temp dir for .git or a backlog never finds the real worktree.
func SessionTmp(worktree string) string {
	sum := sha256.Sum256([]byte(worktree))
	base := os.TempDir()
	if rel, err := filepath.Rel(worktree, base); err == nil && !strings.HasPrefix(rel, "..") {
		base = "/tmp"
	}
	return filepath.Join(base, "komodo-"+hex.EncodeToString(sum[:6]))
}

// toolNames maps Komodo verbs to this host's tool names for the --tools flag.
func toolNames(verbs []string) string {
	var names []string
	for _, verb := range verbs {
		if toolList, ok := tools[verb]; ok {
			names = append(names, toolList...)
		}
	}
	if len(names) == 0 {
		return ""
	}
	out := names[0]
	for _, name := range names[1:] {
		out += ", " + name
	}
	return out
}

// removeEnv removes all entries where the key matches name.
func removeEnv(env []string, name string) []string {
	var out []string
	prefix := name + "="
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			out = append(out, entry)
		}
	}
	return out
}

// setEnv sets or overwrites the env var named name to value, removing any prior entry.
func setEnv(env []string, name, value string) []string {
	env = removeEnv(env, name)
	return append(env, name+"="+value)
}
