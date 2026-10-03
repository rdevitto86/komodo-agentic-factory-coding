package mount

import (
	"os"
	"path/filepath"
	"testing"
)

// writeBinary writes a stand-in binary with the given body under dir.
func writeBinary(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "komodo-built")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPublishPutsTheBinaryAtTheFixedPathAndReplacesIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	target, err := HookPath()
	if err != nil {
		t.Fatal(err)
	}
	if got := Publish(writeBinary(t, t.TempDir(), "one")); got != target {
		t.Fatalf("Publish = %s, want %s", got, target)
	}
	if got := Publish(writeBinary(t, t.TempDir(), "two")); got != target {
		t.Fatalf("Publish = %s, want %s", got, target)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "two" {
		t.Fatalf("fixed binary = %q, %v; want the newest build", data, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(target))
	if len(entries) != 1 {
		t.Fatalf("bin holds %d entries, want only the fixed binary", len(entries))
	}
}

func TestPublishMovesAWindowsBinaryAsideBeforeReplacingIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	previous := hookGOOS
	hookGOOS = "windows"
	t.Cleanup(func() { hookGOOS = previous })
	target, _ := HookPath()
	if filepath.Base(target) != "komodo.exe" {
		t.Fatalf("HookPath = %s, want komodo.exe on Windows", target)
	}
	Publish(writeBinary(t, t.TempDir(), "one"))
	Publish(writeBinary(t, t.TempDir(), "two"))
	if data, _ := os.ReadFile(filepath.Join(filepath.Dir(target), "komodo.old.exe")); string(data) != "one" {
		t.Fatalf("aside = %q, want the replaced binary", data)
	}
	if data, _ := os.ReadFile(target); string(data) != "two" {
		t.Fatalf("fixed binary = %q, want the newest build", data)
	}
}

func TestPublishNamesTheBuildItselfWhenItCannotReadIt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	missing := filepath.Join(t.TempDir(), "absent")
	if got := Publish(missing); got != missing {
		t.Fatalf("Publish = %s, want the unreadable path back", got)
	}
}

func TestPruneHookCopiesRemovesOnlyHashedCopiesAndTheAsideBinary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir := filepath.Join(home, ".komodo", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"komodo", "komodo-0123456789ab", "komodo.old.exe", "komodo-darwin-arm64"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := PruneHookCopies(nil)
	if err != nil || len(removed) != 2 {
		t.Fatalf("removed = %v, %v; want the hashed copy and the aside binary", removed, err)
	}
	for _, kept := range []string{"komodo", "komodo-darwin-arm64"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Fatalf("%s was removed: %v", kept, err)
		}
	}
}

func TestPruneHookCopiesKeepsACopyAnInstalledHookStillRuns(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir := filepath.Join(home, ".komodo", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	used, stale := filepath.Join(dir, "komodo-aaaaaaaaaaaa"), filepath.Join(dir, "komodo-bbbbbbbbbbbb")
	for _, path := range []string{used, stale} {
		if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := PruneHookCopies(map[string]bool{used: true})
	if err != nil || len(removed) != 1 || removed[0] != stale {
		t.Fatalf("removed = %v, %v; want only the copy no hook runs", removed, err)
	}
	if _, err := os.Stat(used); err != nil {
		t.Fatalf("the copy a hook runs was removed: %v", err)
	}
}
