package review

import (
	"os"
	"testing"

	"komodo/internal/testhome"
)

// TestMain runs every test with a temp home and temp directory, so none touches the real ones.
func TestMain(m *testing.M) {
	cleanup := testhome.Isolate()
	code := m.Run()
	cleanup()
	os.Exit(code)
}
