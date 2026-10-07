package fsx

import (
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
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

func TestWriteFileFailsWhenTheDirCannotBeMade(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows permission bits differ")
	}
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocker, "sub", "state.json")
	if err := WriteFile(path, []byte("x"), 0o644); err == nil {
		t.Fatal("want an error when a path segment is a file, not a directory")
	}
}

func TestWriteFileFailsWhenTheDirIsNotWritable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows permission bits differ")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o700)
	path := filepath.Join(dir, "state.json")
	if err := WriteFile(path, []byte("x"), 0o644); err == nil {
		t.Fatal("want an error creating a temp file in a read-only directory")
	}
}

func TestWriteFileFailsWhenThePathIsADirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("x"), 0o644); err == nil {
		t.Fatal("want an error renaming a file over an existing directory")
	}
}

func TestWriteFileFailsWhenTheFileSizeLimitIsExceeded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("RLIMIT_FSIZE has no Windows equivalent")
	}
	signal.Ignore(syscall.SIGXFSZ)
	defer signal.Reset(syscall.SIGXFSZ)
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
		t.Fatal(err)
	}
	defer syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit)
	tight := syscall.Rlimit{Cur: 1, Max: limit.Max}
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &tight); err != nil {
		t.Skipf("cannot tighten RLIMIT_FSIZE: %v", err)
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := WriteFile(path, []byte("more than one byte"), 0o644); err == nil {
		t.Fatal("want an error writing past the file size limit")
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
