package toolkit

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestAStrayKomodoDirectoryNeverReplacesTheEmbeddedToolkit(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "komodo", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.ReadFile(FS(root), filepath.Join("roles", "builder.md")); err != nil {
		t.Fatalf("a komodo/ directory without the markers hid the embedded roles: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "komodo", "roles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "komodo", "roles", "builder.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := fs.ReadFile(FS(root), filepath.Join("roles", "builder.md"))
	if err != nil || string(data) != "x" {
		t.Fatalf("a komodo/ directory with the markers was not served from disk: %q %v", data, err)
	}
}
