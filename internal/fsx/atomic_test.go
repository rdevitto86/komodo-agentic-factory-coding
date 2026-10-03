package fsx

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteFileReplacesTheFileAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "state.json")
	for _, body := range []string{"first", "second"} {
		if err := WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "second" {
		t.Fatalf("file = %q, %v; want the last write", data, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("dir holds %d entries, want only the file", len(entries))
	}
}

func TestWriteFileSetsTheMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no executable bit")
	}
	path := filepath.Join(t.TempDir(), "hook")
	if err := WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, %v; want 0755", info.Mode().Perm(), err)
	}
}
