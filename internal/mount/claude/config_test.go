package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/mount"
)

// TestSessionCanaryInPersonalConfigDoesNotLeakToArgv verifies that personal config
// canary strings never appear in session argv per decision 0025.
func TestSessionCanaryInPersonalConfigDoesNotLeakToArgv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create a fake personal CLAUDE.md with a canary string.
	claudeLocal := filepath.Join(home, ".claude", "CLAUDE.local.md")
	if err := os.MkdirAll(filepath.Dir(claudeLocal), 0o755); err != nil {
		t.Fatal(err)
	}
	canary := "CANARY_STRING_FOR_TESTING_TSK_05_3_2"
	content := "# Personal overlay\n\n" + canary + "\n"
	if err := os.WriteFile(claudeLocal, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{"read"},
		Schema: []byte(`{}`),
	}
	argv, _, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	joined := strings.Join(argv, " ")

	if strings.Contains(joined, canary) {
		t.Fatalf("argv should not contain personal config canary: %s", joined)
	}
}

// TestSessionCanaryInPersonalSettingsDoesNotLeakToArgv verifies that a canary line
// in a fake personal settings file never appears in the session's argv.
func TestSessionCanaryInPersonalSettingsDoesNotLeakToArgv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create a fake personal settings.json with a canary string.
	settingsLocal := filepath.Join(home, ".claude", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(settingsLocal), 0o755); err != nil {
		t.Fatal(err)
	}
	canary := "CANARY_TEST_STRING_SETTINGS"
	content := `{"permissions": {"allow": ["` + canary + `"]}}`
	if err := os.WriteFile(settingsLocal, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{"read"},
		Schema: []byte(`{}`),
	}
	argv, _, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	joined := strings.Join(argv, " ")

	if strings.Contains(joined, canary) {
		t.Fatalf("argv should not contain personal settings canary: %s", joined)
	}
}

// TestSessionCLAUDEConfigDirNotInEnvironment verifies that if CLAUDE_CONFIG_DIR
// is set in the parent environment, it is removed from the session's environment.
func TestSessionCLAUDEConfigDirNotInEnvironment(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/fake/personal/config")

	req := mount.StartRequest{
		Role:   "builder",
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	_, env, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)

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
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	argv, _, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)
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
		Tools:  []string{},
		Schema: []byte(`{}`),
	}
	argv, _, _ := Session("/repo", "/worktree", req, "", "", "sonnet", "extended", 10, 0)
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "--strict-mcp-config") {
		t.Fatalf("argv must include --strict-mcp-config to restrict MCP configuration:\n%s", joined)
	}
}
