package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/mount"
)

// TestRenderNeverLeaksAPersonalConfigCanaryIntoAPlannedFile verifies that a canary planted in the
// machine's own personal Claude config never reaches a file Render plans to write (REQ-3).
func TestRenderNeverLeaksAPersonalConfigCanaryIntoAPlannedFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeLocal := filepath.Join(home, ".claude", "CLAUDE.local.md")
	if err := os.MkdirAll(filepath.Dir(claudeLocal), 0o755); err != nil {
		t.Fatal(err)
	}
	mdCanary := "CANARY_STRING_FOR_TESTING_TSK_05_3_2"
	if err := os.WriteFile(claudeLocal, []byte("# Personal overlay\n\n"+mdCanary+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	settingsLocal := filepath.Join(home, ".claude", "settings.local.json")
	settingsCanary := "CANARY_TEST_STRING_SETTINGS"
	if err := os.WriteFile(settingsLocal, []byte(`{"permissions": {"allow": ["`+settingsCanary+`"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := Render(toolkitRepo(t), "bin/komodo-darwin-arm64")
	if err != nil {
		t.Fatal(err)
	}
	for _, canary := range []string{mdCanary, settingsCanary} {
		for _, change := range plan.Changes {
			if strings.Contains(string(change.Body), canary) {
				t.Fatalf("%s carries the personal config canary %q", change.Path, canary)
			}
		}
	}
}

// TestSessionCLAUDEConfigDirNotInEnvironment verifies that if CLAUDE_CONFIG_DIR
// is set in the parent environment, it is removed from the session's environment.
func TestSessionCLAUDEConfigDirNotInEnvironment(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/fake/personal/config")

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
			t.Fatalf("CLAUDE_CONFIG_DIR should be removed from session env, found: %s", entry)
		}
	}
}

// TestSessionLoadsOnlyLocalSettings verifies that the session argv loads local settings alone, so neither
// personal settings nor the project's shared skills reach a line session.
func TestSessionLoadsOnlyLocalSettings(t *testing.T) {
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

	if !strings.Contains(joined, "--setting-sources local") {
		t.Fatalf("argv must include --setting-sources local to exclude the personal layer and shared skills:\n%s", joined)
	}
}

// TestSessionUsesStrictMcpConfig verifies that the session argv includes
// --strict-mcp-config to restrict MCP servers to project and role definitions.
func TestSessionUsesStrictMcpConfig(t *testing.T) {
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

	if !strings.Contains(joined, "--strict-mcp-config") {
		t.Fatalf("argv must include --strict-mcp-config to restrict MCP configuration:\n%s", joined)
	}
}
