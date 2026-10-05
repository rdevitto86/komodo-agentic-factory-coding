package claude

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"komodo/internal/guard"
	"komodo/internal/mount"
)

// withLineSettings writes a minimal, valid line settings file under root, so a sandboxed platform's
// withSandbox merge has something real to read instead of refusing the session.
func withLineSettings(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, Dir, LineSettings)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"permissions":{"deny":["Edit(~/.claude/**)"]},"hooks":{"PreToolUse":[{"matcher":"*","hooks":[` +
		`{"type":"command","command":"komodo guard"}]}]}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// sandboxedSessionRoot is a fresh repo root already carrying a rendered line settings file.
func sandboxedSessionRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	withLineSettings(t, root)
	return root
}

// argAfter returns the argument following flag, or empty when flag is absent.
func argAfter(argv []string, flag string) string {
	for i, arg := range argv {
		if arg == flag && i+1 < len(argv) {
			return argv[i+1]
		}
	}
	return ""
}

func TestSessionArgvForBuilder(t *testing.T) {
	root := sandboxedSessionRoot(t)
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "test brief",
		Tools:  []string{"read", "edit", "write", "shell", "search"},
		Schema: []byte(`{"type":"object"}`),
	}
	argv, _, prompt, err := Session(root, "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")

	if prompt != "test brief" {
		t.Errorf("prompt = %q, want %q", prompt, "test brief")
	}

	checks := []string{
		"-p",
		"--setting-sources local",
		"--plugin-dir " + filepath.Join(root, Dir, "plugins", "builder"),
		"--tools Read, Edit, Write, Bash, Grep, Glob",
		"--allowedTools Read, Edit, Write, Bash(ls:*), ",
		"Bash(git diff:*)",
		"--disallowedTools Edit(//worktree/docs/prd.md), Edit(//worktree/eval/**), Edit(//worktree/komodo/policy.json), Bash(git add:*)",
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

	settingsValue := argAfter(argv, "--settings")
	var settings map[string]json.RawMessage
	if err := json.Unmarshal([]byte(settingsValue), &settings); err != nil {
		t.Fatalf("--settings value is not JSON: %s", settingsValue)
	}
	for _, key := range []string{"hooks", "permissions"} {
		if _, ok := settings[key]; !ok {
			t.Errorf("--settings value is missing %q: %s", key, settingsValue)
		}
	}
}

func TestSessionArgvForLens(t *testing.T) {
	req := mount.StartRequest{
		Role:   "lens",
		Brief:  "test brief",
		Tools:  []string{"read", "search"},
		Schema: []byte(`{"type":"object"}`),
	}
	argv, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "haiku", "standard", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "--tools Read, Grep, Glob") {
		t.Errorf("lens tools argv missing correct tools:\n%s", joined)
	}
	if !strings.Contains(joined, "--model haiku") {
		t.Errorf("lens model not haiku:\n%s", joined)
	}
}

func TestSessionOmitsAnUnsetEffort(t *testing.T) {
	req := mount.StartRequest{Role: "reviewer", Brief: "review", Tools: []string{"read"}, Schema: []byte(`{"type":"object"}`)}
	argv, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "opus", "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range argv {
		if arg == "--effort" {
			t.Fatalf("argv = %v; an unset effort must leave the flag out", argv)
		}
	}
}

func TestSessionStartSendsTheBriefAsThePrompt(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "fix the parser bug",
		Tools:  []string{"read"},
		Schema: []byte(`{}`),
	}
	argv, _, prompt, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
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
	argv, _, prompt, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

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
	argv, _, prompt, err := Session(
		sandboxedSessionRoot(t), "/worktree", req, "session-123", "fix the failing test", "sonnet", "extended", 10, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
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
	argv, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 5.25)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "--max-budget-usd 5.25") {
		t.Errorf("argv missing --max-budget-usd:\n%s", joined)
	}
}

func TestSessionEnvRemovesCLAUDEConfigDir(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/home/user/.claude")
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range env {
		if strings.HasPrefix(entry, "CLAUDE_CONFIG_DIR=") {
			t.Fatalf("env should not contain CLAUDE_CONFIG_DIR: %s", env)
		}
	}
}

func TestSetEnvReplacesAnEmptyValue(t *testing.T) {
	env := setEnv([]string{"GOCACHE=", "HOME=/h"}, "GOCACHE", "/cache")
	if strings.Join(env, " ") != "HOME=/h GOCACHE=/cache" {
		t.Fatalf("env = %q, want one GOCACHE entry", env)
	}
}

func TestSessionEnvSetsGoCache(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range env {
		if strings.HasPrefix(entry, "GOCACHE=") {
			if !strings.HasSuffix(entry, "GOCACHE=/worktree/.komodo/go/cache") {
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
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range env {
		if strings.HasPrefix(entry, "GOTMPDIR=") {
			if entry != "GOTMPDIR="+SessionTmp("/worktree") {
				t.Errorf("GOTMPDIR = %s, want the session's temp root outside the worktree", entry)
			}
			return
		}
	}
	t.Fatal("GOTMPDIR not set in env")
}

func TestSessionEnvSetsGopath(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range env {
		if strings.HasPrefix(entry, "GOPATH=") {
			if !strings.HasSuffix(entry, "GOPATH=/worktree/.komodo/go/path") {
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
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range env {
		if strings.HasPrefix(entry, "GOMODCACHE=") {
			if !strings.HasSuffix(entry, "GOMODCACHE=/worktree/.komodo/go/path/pkg/mod") {
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
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

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
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

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

func TestSessionEnvMakesGitInitWriteNoHooks(t *testing.T) {
	req := mount.StartRequest{Role: "builder", Brief: "b", Schema: []byte(`{}`)}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	var template []string
	for _, entry := range env {
		if strings.HasPrefix(entry, "GIT_TEMPLATE_DIR=") {
			template = append(template, entry)
		}
	}
	if len(template) != 1 || template[0] != "GIT_TEMPLATE_DIR=" {
		t.Fatalf("GIT_TEMPLATE_DIR entries = %v, want one empty value", template)
	}

	// The repo's tests run a non-bare git init under GOTMPDIR, which the sandbox allows only outside the worktree.
	tmp := ""
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, "GOTMPDIR="); ok {
			tmp = value
		}
	}
	if rel, err := filepath.Rel("/worktree", tmp); tmp == "" || err != nil || !strings.HasPrefix(rel, "..") {
		t.Fatalf("GOTMPDIR = %q, want a dir outside the worktree", tmp)
	}
	repo := filepath.Join(t.TempDir(), "w")
	cmd := exec.Command("git", "init", "-q", "-b", "main", repo)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "config")); err != nil {
		t.Fatalf("git init made no repo: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "hooks")); !os.IsNotExist(err) {
		t.Fatalf("git init wrote a hooks dir (stat err %v); the session's env must suppress templates", err)
	}
}

func TestSessionEnvSetsClaudeCodeStopHookBlockCap(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

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
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 40, 0)
	if err != nil {
		t.Fatal(err)
	}

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
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "haiku", "standard", 8, 0)
	if err != nil {
		t.Fatal(err)
	}

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
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

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
		Brief:  "b",
		Tools:  []string{},
		Schema: schema,
	}
	argv, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

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
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	argv, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")

	if strings.Contains(joined, "--tools") {
		t.Fatal("argv should not contain --tools when no tools provided")
	}
}

func TestSessionPreservesOtherEnvVars(t *testing.T) {
	t.Setenv("MY_VAR", "test_value")
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

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
		Brief:  "b",
		Tools:  []string{"read"},
		Schema: []byte(`{}`),
	}
	argv, _, prompt, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(argv) < 1 || argv[0] != "-p" {
		t.Fatalf("argv doesn't start with -p: %v", argv)
	}
	if prompt != "b" {
		t.Fatalf("prompt = %q, want the brief", prompt)
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

func TestSessionRefusesAnEmptyBrief(t *testing.T) {
	req := mount.StartRequest{Role: "builder", Tools: []string{"read"}, Schema: []byte(`{}`)}
	_, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err == nil {
		t.Fatal("an empty brief must refuse to start a session rather than send \"/\"")
	}
}

func TestSessionRefusesAnEmptyResumeInput(t *testing.T) {
	req := mount.StartRequest{Role: "builder", Brief: "test brief", Tools: []string{"read"}, Schema: []byte(`{}`)}
	_, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "session-123", "", "sonnet", "extended", 10, 0)
	if err == nil {
		t.Fatal("an empty resume input must refuse to start a session rather than send \"/\"")
	}
}

func TestSessionPluginDirPath(t *testing.T) {
	req := mount.StartRequest{
		Role:   "architect",
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	root := sandboxedSessionRoot(t)
	argv, _, _, err := Session(root, "/worktree", req, "", "", "opus", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")

	expected := "--plugin-dir " + filepath.Join(root, Dir, "plugins", "architect")
	if !strings.Contains(joined, expected) {
		t.Errorf("argv missing correct plugin dir: %s\nfull: %s", expected, joined)
	}
}

// fixedSandboxMerge is withSandbox's merged value for the fixture at /worktree, with HOME fixed for a stable credential
// path, read when the test runs so the temp root follows its TMPDIR.
func fixedSandboxMerge() string {
	return `{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"komodo guard"}]}]},` +
		`"permissions":{"deny":["Edit(~/.claude/**)"]},"sandbox":{"allowUnsandboxedCommands":false,"enabled":true,` +
		`"failIfUnavailable":true,"filesystem":{"allowWrite":["` + SessionTmp("/worktree") + `"],` +
		`"denyRead":["/home/komodo-fixed-test/.git-credentials",` +
		`"/home/komodo-fixed-test/.config/git/credentials","/home/komodo-fixed-test/.config/gh"]},` +
		`"network":{"allowLocalBinding":true,"allowedDomains":[]}}}`
}

func TestSessionSettingsPath(t *testing.T) {
	t.Setenv("HOME", "/home/komodo-fixed-test")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("GH_CONFIG_DIR", "")
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	root := sandboxedSessionRoot(t)
	argv, _, _, err := Session(root, "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")

	settingsPath := filepath.Join(root, Dir, LineSettings)
	expected := "--settings " + settingsPath
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		expected = "--settings " + fixedSandboxMerge()
	}
	if !strings.Contains(joined, expected) {
		t.Errorf("argv missing correct settings path: %s\nfull: %s", expected, joined)
	}
}

func TestTheSandboxMergesIntoTheRolesRenderedSettings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	sandbox := lineSandbox(mount.Overlay{}, "linux", "")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	rendered := `{"hooks":{"PreToolUse":[{"matcher":"Bash"}]},"permissions":{"deny":["Edit(~/.claude/**)"]}}`
	if err := os.WriteFile(filepath.Join(root, Dir, LineSettings), []byte(rendered), 0o644); err != nil {
		t.Fatal(err)
	}
	merged, err := withSandbox(filepath.Join(root, Dir, LineSettings), sandbox)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal([]byte(merged), &settings); err != nil {
		t.Fatalf("settings %q: %v", merged, err)
	}
	for _, key := range []string{"hooks", "permissions", "sandbox"} {
		if _, found := settings[key]; !found {
			t.Fatalf("settings = %s, missing %s", merged, key)
		}
	}
	if !parseSandbox(t, merged).FailIfUnavailable {
		t.Fatalf("settings = %s; the merged sandbox must still fail if unavailable", merged)
	}
}

func TestWithSandboxRefusesAMissingSettingsFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, Dir, LineSettings)
	sandbox := lineSandbox(mount.Overlay{}, "linux", "")
	if _, err := withSandbox(path, sandbox); err == nil {
		t.Fatal("a missing settings file must refuse, not drop its hooks and denies")
	}
}

func TestWithSandboxRefusesMalformedSettings(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, Dir, LineSettings)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	sandbox := lineSandbox(mount.Overlay{}, "linux", "")
	if _, err := withSandbox(path, sandbox); err == nil {
		t.Fatal("malformed settings must refuse, not drop its hooks and denies")
	}
}

func TestSessionRefusesToLaunchWhenTheLineSettingsFileIsMissing(t *testing.T) {
	if lineSandbox(mount.LoadOverlay(), runtime.GOOS, "") == "" {
		t.Skip("this platform has no sandbox to merge")
	}
	req := mount.StartRequest{Role: "builder", Brief: "b", Tools: []string{"read"}, Schema: []byte(`{}`)}
	_, _, _, err := Session(t.TempDir(), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err == nil {
		t.Fatal("a missing line settings file must refuse to launch, not drop the role's hooks and denies")
	}
}

func TestSessionMultipleTools(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "b",
		Tools:  []string{"read", "shell", "search"},
		Schema: []byte(`{}`),
	}
	argv, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

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
			Brief:  "b",
			Tools:  []string{},
			Schema: []byte(`{}`),
		}
		argv, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", effort, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
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
			Brief:  "b",
			Tools:  []string{},
			Schema: []byte(`{}`),
		}
		argv, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", model, "extended", 10, 0)
		if err != nil {
			t.Fatal(err)
		}
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
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/work/tree/path", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range env {
		if strings.HasPrefix(entry, "GOCACHE=") {
			if !strings.Contains(entry, "/work/tree/path/.komodo/go/cache") {
				t.Errorf("GOCACHE has incorrect worktree path: %s", entry)
			}
		}
		if strings.HasPrefix(entry, "GOPATH=") {
			if !strings.Contains(entry, "/work/tree/path/.komodo/go/path") {
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
		Brief:  "b",
		Tools:  []string{},
		Schema: schemaBytes,
	}
	argv, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

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
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	argv, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 10.99)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "--max-budget-usd 10.99") {
		t.Errorf("argv budget precision not preserved:\n%s", joined)
	}
}

func TestSessionEmptyResume(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "b",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	argv, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")

	if strings.Contains(joined, "--resume") {
		t.Fatal("--resume should not appear when handle is empty")
	}
}

func TestSessionBuilderVerbsMapToHostTools(t *testing.T) {
	req := mount.StartRequest{
		Role:   "builder",
		Brief:  "b",
		Tools:  []string{"read", "edit", "write", "shell", "search"},
		Schema: []byte(`{}`),
	}
	argv, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	if err != nil {
		t.Fatal(err)
	}

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

func TestARoleSessionCarriesTheLineSandbox(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := sandboxedSessionRoot(t)
	argv, _, _, err := Session(root, "/worktree", mount.StartRequest{Role: "builder", Brief: "b"}, "", "", "sonnet", "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := lineSandbox(mount.LoadOverlay(), runtime.GOOS, SessionTmp("/worktree"))
	if want != "" {
		merged, err := withSandbox(filepath.Join(root, Dir, LineSettings), want)
		if err != nil {
			t.Fatal(err)
		}
		carried := false
		for i, arg := range argv[:len(argv)-1] {
			carried = carried || (arg == "--settings" && argv[i+1] == merged)
		}
		if !carried {
			t.Fatalf("argv = %v; a role session must carry the line sandbox as inline settings", argv)
		}
	}
	if strings.Count(strings.Join(argv, " "), "--settings") != 1 {
		t.Fatalf("argv = %v; exactly one --settings is passed, since the host keeps only the last", argv)
	}
	if want != "" && !strings.Contains(want, ".git-credentials") {
		t.Fatalf("sandbox = %s; the credential store must be unreadable", want)
	}
}

func TestSessionTempRootSitsOutsideTheWorktree(t *testing.T) {
	worktree := t.TempDir()
	_, env, _, err := Session(
		sandboxedSessionRoot(t), worktree, mount.StartRequest{Role: "builder", Brief: "b"}, "", "", "m", "", 10, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	var tmp string
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, "CLAUDE_CODE_TMPDIR="); ok {
			tmp = value
		}
	}
	if tmp == "" || tmp != SessionTmp(worktree) {
		t.Fatalf("CLAUDE_CODE_TMPDIR = %q, want the worktree's own temp root", tmp)
	}
	if rel, err := filepath.Rel(worktree, tmp); err == nil && !strings.HasPrefix(rel, "..") {
		t.Fatalf("temp root %q sits inside the worktree, where a walk up for .git finds the real repo", tmp)
	}
	if SessionTmp(worktree) == SessionTmp(worktree+"-other") {
		t.Fatal("two worktrees share one temp root")
	}
}

func TestSessionEnvNamesTheLineRoleForTheGuard(t *testing.T) {
	req := mount.StartRequest{Role: "reviewer", Brief: "b", Tools: []string{"read"}, Schema: []byte(`{}`)}
	_, env, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "opus", "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range env {
		found = found || entry == guard.RoleEnv+"=reviewer"
	}
	if !found {
		t.Fatalf("env = %v; the guard must see the session's line role", env)
	}
}

func TestSessionDeniesTheLinePathsEvenWithNoTools(t *testing.T) {
	req := mount.StartRequest{Role: "builder", Brief: "b", Schema: []byte(`{}`)}
	argv, _, _, err := Session(sandboxedSessionRoot(t), "/worktree", req, "", "", "sonnet", "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"Edit(//worktree/docs/prd.md)", "Edit(//worktree/eval/**)", "Edit(//worktree/komodo/policy.json)"} {
		if !strings.Contains(joined, "--disallowedTools ") || !strings.Contains(joined, want) {
			t.Errorf("argv missing deny %q:\n%s", want, joined)
		}
	}
}
