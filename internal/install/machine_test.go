package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOnWindowsDriveFlagsOnlyAWSLRepoUnderMnt(t *testing.T) {
	wsl := "Linux version 5.15.90.1-microsoft-standard-WSL2"
	cases := []struct {
		root, proc string
		want       bool
	}{
		{"/mnt/c/src/repo", wsl, true},
		{"/home/a/src/repo", wsl, false},
		{"/mnt/c/src/repo", "Linux version 6.1.0-generic", false},
	}
	for _, c := range cases {
		if got := OnWindowsDrive(c.root, c.proc); got != c.want {
			t.Errorf("OnWindowsDrive(%s, %s) = %v, want %v", c.root, c.proc, got, c.want)
		}
	}
}

func TestLinkOnPathLinksKomodoAndHintsWhenTheDirIsNotOnPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows puts the binary's own directory on PATH")
	}
	binary := filepath.Join(t.TempDir(), "komodo")
	if err := os.WriteFile(binary, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "bin")
	hint, err := LinkOnPath(binary, dir, "/usr/bin")
	if err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(filepath.Join(dir, "komodo")); err != nil || target != binary {
		t.Fatalf("link = %q, %v; want %s", target, err, binary)
	}
	if !strings.Contains(hint, dir) {
		t.Fatalf("hint = %q, want it to name %s", hint, dir)
	}
	// A second run replaces the link and, with dir on PATH, says nothing.
	if hint, err := LinkOnPath(binary, dir, "/usr/bin"+string(os.PathListSeparator)+dir); err != nil || hint != "" {
		t.Fatalf("hint = %q, %v; want none once dir is on PATH", hint, err)
	}
}
