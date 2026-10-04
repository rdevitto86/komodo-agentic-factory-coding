package guard

import (
	"os"
	"testing"

	"komodo/internal/testhome"
)

// TestMain isolates home and temp, and registers the fake host first, so no test depends on run order.
func TestMain(m *testing.M) {
	cleanup := testhome.Isolate()
	registerFakeHost()
	code := m.Run()
	cleanup()
	os.Exit(code)
}
