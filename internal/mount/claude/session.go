package claude

import (
	"os"
	"path/filepath"
	"strconv"

	"komodo/internal/mount"
)

// Session builds the argv and environment for starting or resuming a Claude Code session headless.
func Session(
	root, worktree string,
	req mount.StartRequest,
	resumed mount.Handle,
	resumeInput string,
	model, effort string,
	maxTurns int,
	maxBudgetUSD float64,
) (argv []string, env []string) {
	prompt := req.Brief
	if resumed != "" {
		prompt = resumeInput
	}
	if prompt == "" {
		prompt = "/"
	}

	argv = []string{"-p", prompt}
	argv = append(argv, "--setting-sources", "project,local")

	pluginDir := filepath.Join(root, Dir, "plugins", req.Role)
	argv = append(argv, "--plugin-dir", pluginDir)

	settingsPath := filepath.Join(root, Dir, "settings.json")
	argv = append(argv, "--settings", settingsPath)

	if len(req.Tools) > 0 {
		argv = append(argv, "--tools", toolNames(req.Tools))
	}

	argv = append(argv, "--permission-mode", "dontAsk")
	argv = append(argv, "--model", model)
	argv = append(argv, "--effort", effort)
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

	env = os.Environ()
	env = removeEnv(env, "CLAUDE_CONFIG_DIR")
	env = setEnv(env, "CLAUDE_CODE_STOP_HOOK_BLOCK_CAP", "3")
	env = setEnv(env, "CLAUDE_CODE_MAX_TURNS", strconv.Itoa(maxTurns))
	env = setEnv(env, "DISABLE_AUTOUPDATER", "1")
	env = setEnv(env, "GOCACHE", filepath.Join(worktree, ".gocache"))
	env = setEnv(env, "GOTMPDIR", filepath.Join(worktree, ".gotmpdir"))
	env = setEnv(env, "GOPATH", filepath.Join(worktree, ".gopath"))
	env = setEnv(env, "GOMODCACHE", filepath.Join(worktree, ".gopath", "pkg", "mod"))
	env = setEnv(env, "GOPROXY", "off")
	env = setEnv(env, "GOFLAGS", "-modcacherw")

	return argv, env
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
		if len(entry) <= len(prefix) || entry[:len(prefix)] != prefix {
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
