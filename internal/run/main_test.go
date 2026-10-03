package run

import (
	"komodo/internal/testhome"
	"os"
	"testing"
)

// TestMain clears the headless run's credential scrub, so a driven ship takes the push path inside a test too.
func TestMain(m *testing.M) {
	for _, key := range []string{"GIT_TERMINAL_PROMPT", "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0"} {
		os.Unsetenv(key)
	}
	cleanup := testhome.Isolate()
	code := m.Run()
	cleanup()
	os.Exit(code)
}
