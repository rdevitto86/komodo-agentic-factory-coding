package claude

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"komodo/internal/mount"
)

func TestSessionArgvForBuilder(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "test brief",
		Tools:  []string{"read", "edit", "write", "shell", "search"},
		Schema: []byte(`{"type":"object"}`),
	}
	argv, _, prompt := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	joined := strings.Join(argv, " ")

	if prompt != "test brief" {
		t.Errorf("prompt = %q, want %q", prompt, "test brief")
	}

	checks := []string{
		"-p",
		"--setting-sources local",
		"--plugin-dir /repo/.claude/plugins/builder",
		"--settings /repo/.claude/settings.json",
		"--tools Read, Edit, Write, Bash, Grep, Glob",
		"--permission-mode dontAsk",
		"--model sonnet",
		"--effort extended",
		"--strict-mcp-config",
		"--output-format stream-json",
		"--verbose",
		"--json-schema",
	}
	for _, want := range checks {
		if !strings.Contains(joined, want) {
			t.Errorf("argv missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "--resume") {
		t.Fatal("argv should not contain --resume when not resuming")
	}
	if strings.Contains(joined, "--max-budget-usd") {
		t.Fatal("argv should not contain --max-budget-usd when maxBudgetUSD is 0")
	}
}

func TestSessionArgvForLens(t *testing.T) {
	req := mount.StartRequest{
		Role:   "lens",
		Brief:  "test brief",
		Tools:  []string{"read", "search"},
		Schema: []byte(`{"type":"object"}`),
	}
	argv, _, _ := Session("/repo", "/worktree", req, "", "", "haiku", "standard", 10, 0)
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "--tools Read, Grep, Glob") {
		t.Errorf("lens tools argv missing correct tools:\n%s", joined)
	}
	if !strings.Contains(joined, "--model haiku") {
		t.Errorf("lens model not haiku:\n%s", joined)
	}
}

func TestSessionStartSendsTheBriefAsThePrompt(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "fix the parser bug",
		Tools:  []string{"read"},
		Schema: []byte(`{}`),
	}
	argv, _, prompt := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	joined := strings.Join(argv, " ")

	if prompt != "fix the parser bug" {
		t.Errorf("prompt = %q, want the brief", prompt)
	}
	if strings.Contains(joined, "fix the parser bug") {
		t.Errorf("argv should not carry the prompt text:\n%s", joined)
	}
}

func TestSessionPromptStartingWithDashStaysOffArgv(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "- fix X",
		Tools:  []string{"read"},
		Schema: []byte(`{}`),
	}
	argv, _, prompt := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	if prompt != "- fix X" {
		t.Errorf("prompt = %q, want the brief unchanged", prompt)
	}
	for _, arg := range argv {
		if arg != "-p" && strings.HasPrefix(arg, "-") && arg == "- fix X" {
			t.Fatalf("prompt leaked into argv where it could be parsed as a flag: %v", argv)
		}
	}
}

func TestSessionResume(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "test brief",
		Tools:  []string{"read"},
		Schema: []byte(`{}`),
	}
	argv, _, prompt := Session("/repo", "/worktree", req, "session-123", "fix the failing test", "sonnet", "extended", 10, 0)
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "--resume session-123") {
		t.Errorf("argv missing --resume flag:\n%s", joined)
	}
	if prompt != "fix the failing test" {
		t.Errorf("prompt = %q, want the resume input", prompt)
	}
	if strings.Contains(joined, "test brief") || strings.Contains(joined, "fix the failing test") {
		t.Errorf("argv should not carry either prompt text:\n%s", joined)
	}
}

func TestSessionMaxBudgetUSD(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "test brief",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	argv, _, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 5.25)
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "--max-budget-usd 5.25") {
		t.Errorf("argv missing --max-budget-usd:\n%s", joined)
	}
}

func TestSessionEnvRemovesCLAUDEConfigDir(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/home/user/.claude")
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	for _, entry := range env {
		if strings.HasPrefix(entry, "CLAUDE_CONFIG_DIR=") {
			t.Fatalf("env should not contain CLAUDE_CONFIG_DIR: %s", env)
		}
	}
}

func TestSessionEnvSetsGoCache(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	for _, entry := range env {
		if strings.HasPrefix(entry, "GOCACHE=") {
			if !strings.HasSuffix(entry, "GOCACHE=/worktree/.gocache") {
				t.Errorf("GOCACHE not in worktree: %s", entry)
			}
			return
		}
	}
	t.Fatal("GOCACHE not set in env")
}

func TestSessionEnvSetsGOTMPDIR(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	for _, entry := range env {
		if strings.HasPrefix(entry, "GOTMPDIR=") {
			if !strings.HasSuffix(entry, "GOTMPDIR=/worktree/.gotmpdir") {
				t.Errorf("GOTMPDIR not in worktree: %s", entry)
			}
			return
		}
	}
	t.Fatal("GOTMPDIR not set in env")
}

func TestSessionEnvSetsGopath(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	for _, entry := range env {
		if strings.HasPrefix(entry, "GOPATH=") {
			if !strings.HasSuffix(entry, "GOPATH=/worktree/.gopath") {
				t.Errorf("GOPATH not in worktree: %s", entry)
			}
			return
		}
	}
	t.Fatal("GOPATH not set in env")
}

func TestSessionEnvSetsGomodcache(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	for _, entry := range env {
		if strings.HasPrefix(entry, "GOMODCACHE=") {
			if !strings.HasSuffix(entry, "GOMODCACHE=/worktree/.gopath/pkg/mod") {
				t.Errorf("GOMODCACHE not in worktree: %s", entry)
			}
			return
		}
	}
	t.Fatal("GOMODCACHE not set in env")
}

func TestSessionEnvSetsGoproxy(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	for _, entry := range env {
		if strings.HasPrefix(entry, "GOPROXY=") {
			if !strings.HasSuffix(entry, "GOPROXY=off") {
				t.Errorf("GOPROXY not off: %s", entry)
			}
			return
		}
	}
	t.Fatal("GOPROXY not set in env")
}

func TestSessionEnvSetsGoflags(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	for _, entry := range env {
		if strings.HasPrefix(entry, "GOFLAGS=") {
			if !strings.HasSuffix(entry, "GOFLAGS=-modcacherw") {
				t.Errorf("GOFLAGS not -modcacherw: %s", entry)
			}
			return
		}
	}
	t.Fatal("GOFLAGS not set in env")
}

func TestSessionEnvSetsClaudeCodeStopHookBlockCap(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	for _, entry := range env {
		if strings.HasPrefix(entry, "CLAUDE_CODE_STOP_HOOK_BLOCK_CAP=") {
			if !strings.HasSuffix(entry, "CLAUDE_CODE_STOP_HOOK_BLOCK_CAP=3") {
				t.Errorf("CLAUDE_CODE_STOP_HOOK_BLOCK_CAP not 3: %s", entry)
			}
			return
		}
	}
	t.Fatal("CLAUDE_CODE_STOP_HOOK_BLOCK_CAP not set in env")
}

func TestSessionEnvSetsMaxTurnsForBuilder(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 40, 0)

	for _, entry := range env {
		if strings.HasPrefix(entry, "CLAUDE_CODE_MAX_TURNS=") {
			if !strings.HasSuffix(entry, "CLAUDE_CODE_MAX_TURNS=40") {
				t.Errorf("CLAUDE_CODE_MAX_TURNS not 40: %s", entry)
			}
			return
		}
	}
	t.Fatal("CLAUDE_CODE_MAX_TURNS not set in env")
}

func TestSessionEnvSetsMaxTurnsForLens(t *testing.T) {
	req := mount.StartRequest{
		Role:   "lens",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _ := Session("/repo", "/worktree", req, "", "", "haiku", "standard", 8, 0)

	for _, entry := range env {
		if strings.HasPrefix(entry, "CLAUDE_CODE_MAX_TURNS=") {
			if !strings.HasSuffix(entry, "CLAUDE_CODE_MAX_TURNS=8") {
				t.Errorf("CLAUDE_CODE_MAX_TURNS not 8: %s", entry)
			}
			return
		}
	}
	t.Fatal("CLAUDE_CODE_MAX_TURNS not set in env")
}

func TestSessionEnvSetsDisableAutoupdater(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	for _, entry := range env {
		if strings.HasPrefix(entry, "DISABLE_AUTOUPDATER=") {
			if !strings.HasSuffix(entry, "DISABLE_AUTOUPDATER=1") {
				t.Errorf("DISABLE_AUTOUPDATER not 1: %s", entry)
			}
			return
		}
	}
	t.Fatal("DISABLE_AUTOUPDATER not set in env")
}

func TestSessionSchemaInArgv(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"result":{"type":"string"}}}`)
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: schema,
	}
	argv, _, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	found := false
	for i, arg := range argv {
		if arg == "--json-schema" && i+1 < len(argv) {
			if argv[i+1] == string(schema) {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("argv missing schema or has wrong value: %v", argv)
	}
}

func TestSessionNoToolsGivesEmptyToolsList(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	argv, _, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	joined := strings.Join(argv, " ")

	if strings.Contains(joined, "--tools") {
		t.Fatal("argv should not contain --tools when no tools provided")
	}
}

func TestSessionPreservesOtherEnvVars(t *testing.T) {
	t.Setenv("MY_VAR", "test_value")
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	found := false
	for _, entry := range env {
		if entry == "MY_VAR=test_value" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("session did not preserve other env vars")
	}
}

func TestSessionArgvOrder(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{"read"},
		Schema: []byte(`{}`),
	}
	argv, _, prompt := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	if len(argv) < 1 || argv[0] != "-p" {
		t.Fatalf("argv doesn't start with -p: %v", argv)
	}
	if prompt != "/" {
		t.Fatalf("prompt = %q, want the default /", prompt)
	}

	settingsIdx := -1
	for i, arg := range argv {
		if arg == "--settings" {
			settingsIdx = i
			break
		}
	}
	if settingsIdx == -1 || settingsIdx+1 >= len(argv) {
		t.Fatal("--settings not found or no path after it")
	}
}

func TestSessionPluginDirPath(t *testing.T) {
	req := mount.StartRequest{
		Role:   "architect",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	argv, _, _ := Session("/repo/root", "/worktree", req, "", "", "opus", "extended", 10, 0)
	joined := strings.Join(argv, " ")

	expected := "--plugin-dir /repo/root/.claude/plugins/architect"
	if !strings.Contains(joined, expected) {
		t.Errorf("argv missing correct plugin dir: %s\nfull: %s", expected, joined)
	}
}

func TestSessionSettingsPath(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	argv, _, _ := Session("/my/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	joined := strings.Join(argv, " ")

	expected := "--settings /my/repo/.claude/settings.json"
	if !strings.Contains(joined, expected) {
		t.Errorf("argv missing correct settings path: %s\nfull: %s", expected, joined)
	}
}

func TestSessionMultipleTools(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{"read", "shell", "search"},
		Schema: []byte(`{}`),
	}
	argv, _, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	var toolsStr string
	for i, arg := range argv {
		if arg == "--tools" && i+1 < len(argv) {
			toolsStr = argv[i+1]
			break
		}
	}

	if toolsStr == "" {
		t.Fatal("--tools not found in argv")
	}

	expectedTools := []string{"Read", "Bash", "Grep", "Glob"}
	for _, tool := range expectedTools {
		if !strings.Contains(toolsStr, tool) {
			t.Errorf("tools string missing %q: %s", tool, toolsStr)
		}
	}
}

func TestSessionEffortValue(t *testing.T) {
	cases := []string{"standard", "extended", "maximum"}
	for _, effort := range cases {
		req := mount.StartRequest{
			Role:   "builder",
			Tools:  []string{},
			Schema: []byte(`{}`),
		}
		argv, _, _ := Session("/repo", "/worktree", req, "", "", "sonnet", effort, 10, 0)
		joined := strings.Join(argv, " ")

		expected := "--effort " + effort
		if !strings.Contains(joined, expected) {
			t.Errorf("argv missing --effort %s:\n%s", effort, joined)
		}
	}
}

func TestSessionModelValue(t *testing.T) {
	models := []string{"haiku", "sonnet", "opus"}
	for _, model := range models {
		req := mount.StartRequest{
			Role:   "builder",
			Tools:  []string{},
			Schema: []byte(`{}`),
		}
		argv, _, _ := Session("/repo", "/worktree", req, "", "", model, "extended", 10, 0)
		joined := strings.Join(argv, " ")

		expected := "--model " + model
		if !strings.Contains(joined, expected) {
			t.Errorf("argv missing --model %s:\n%s", model, joined)
		}
	}
}

func TestSessionEnvWithWorktreeSlashes(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _ := Session("/repo", "/work/tree/path", req, "", "", "sonnet", "extended", 10, 0)

	for _, entry := range env {
		if strings.HasPrefix(entry, "GOCACHE=") {
			if !strings.Contains(entry, "/work/tree/path/.gocache") {
				t.Errorf("GOCACHE has incorrect worktree path: %s", entry)
			}
		}
		if strings.HasPrefix(entry, "GOPATH=") {
			if !strings.Contains(entry, "/work/tree/path/.gopath") {
				t.Errorf("GOPATH has incorrect worktree path: %s", entry)
			}
		}
	}
}

func TestSessionSchemaValidJSON(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"result": map[string]string{"type": "string"},
		},
	}
	schemaBytes, _ := json.Marshal(schema)

	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: schemaBytes,
	}
	argv, _, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	var foundSchema bool
	for i, arg := range argv {
		if arg == "--json-schema" && i+1 < len(argv) {
			foundSchema = true
			if !bytes.Equal([]byte(argv[i+1]), schemaBytes) {
				t.Errorf("schema in argv does not match: got %q, want %q", argv[i+1], string(schemaBytes))
			}
		}
	}
	if !foundSchema {
		t.Fatal("--json-schema not found in argv")
	}
}

func TestSessionMaxBudgetPrecision(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	argv, _, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 10.99)
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "--max-budget-usd 10.99") {
		t.Errorf("argv budget precision not preserved:\n%s", joined)
	}
}

func TestSessionEmptyResume(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	argv, _, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	joined := strings.Join(argv, " ")

	if strings.Contains(joined, "--resume") {
		t.Fatal("--resume should not appear when handle is empty")
	}
}

func TestSessionBuilderVerbsMapToHostTools(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{"read", "edit", "write", "shell", "search"},
		Schema: []byte(`{}`),
	}
	argv, _, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

	var toolsStr string
	for i, arg := range argv {
		if arg == "--tools" && i+1 < len(argv) {
			toolsStr = argv[i+1]
			break
		}
	}

	hostTools := []string{"Read", "Edit", "Write", "Bash", "Grep", "Glob"}
	for _, tool := range hostTools {
		if !strings.Contains(toolsStr, tool) {
			t.Errorf("tools missing host name %q: %s", tool, toolsStr)
		}
	}
}
