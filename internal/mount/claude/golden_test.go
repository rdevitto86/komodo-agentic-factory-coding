package claude

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// update rewrites the golden files from the current render instead of comparing against them.
var update = flag.Bool("update", false, "rewrite testdata/golden from the current render")

// golden compares body, with machine paths replaced by placeholders, against testdata/golden/name.
func golden(t *testing.T, name, body string, paths map[string]string) {
	t.Helper()
	for path, placeholder := range paths {
		body = strings.ReplaceAll(body, path, placeholder)
	}
	file := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("%v; run go test ./internal/mount/claude/ -run Golden -update to write it", err)
	}
	if string(want) != body {
		t.Fatalf("%s changed; review the diff and rerun with -update if it is intended:\ngot:\n%s\nwant:\n%s", name, body, want)
	}
}

// TestGoldenRenders pins every settings file the mount renders byte for byte, so a render change shows in
// the diff of the PR that makes it rather than on someone's machine.
func TestGoldenRenders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := orchestratorRepo(t)
	binary := filepath.Join(t.TempDir(), "komodo-built")
	if err := os.WriteFile(binary, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{root: "$ROOT", home: "$HOME"}
	plan, err := Render(root, binary)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range plan.Changes {
		switch filepath.Base(change.Path) {
		case "settings.json", LineSettings:
			golden(t, filepath.Base(change.Path), string(change.Body), paths)
		}
	}
	global, err := RenderGlobal(root, home, binary)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range global.Changes {
		if filepath.Base(change.Path) == "settings.json" {
			golden(t, "global-settings.json", string(change.Body), paths)
		}
	}
}
