package harness

import (
	"komodo/internal/testhome"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain clears the headless run's credential scrub and lock pid, so tests inside a run act as outside it,
// and stops git's walk up at the temp root, so a fixture never finds the worktree holding it.
func TestMain(m *testing.M) {
	scrubbed := []string{"GIT_TERMINAL_PROMPT", "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0", LockEnv}
	for _, key := range scrubbed {
		_ = os.Unsetenv(key)
	}
	if err := os.Setenv("GIT_CEILING_DIRECTORIES", strings.Join(tempRoots(), string(os.PathListSeparator))); err != nil {
		panic(err)
	}
	cleanup := testhome.Isolate()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// tempRoots are the directories t.TempDir creates under, as given and with symlinks resolved: os.TempDir,
// never GOTMPDIR, which t.TempDir ignores and a harness session points inside its own worktree.
func tempRoots() []string {
	base := os.TempDir()
	roots := []string{filepath.Clean(base)}
	if resolved, err := filepath.EvalSymlinks(base); err == nil && resolved != roots[0] {
		roots = append(roots, resolved)
	}
	return roots
}

func TestTempRootsFollowTempDirNotGOTMPDIR(t *testing.T) {
	t.Setenv("GOTMPDIR", filepath.Join(t.TempDir(), "elsewhere"))
	if roots := tempRoots(); roots[0] != filepath.Clean(os.TempDir()) {
		t.Fatalf("roots = %v; the git ceiling must sit where t.TempDir creates, %s", roots, os.TempDir())
	}
}
