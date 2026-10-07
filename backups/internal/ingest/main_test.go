package ingest

import (
	"os"
	"testing"

	"komodo/internal/testhome"
)

// TestMain runs every test with a temp home directory, so none writes the real ~/.komodo or ~/.claude.
func TestMain(m *testing.M) {
	cleanup := testhome.Isolate()
	code := m.Run()
	cleanup()
	os.Exit(code)
}
