//go:build unix

package review

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestAReproducerSkipsWhatIsNeitherFileDirectoryNorSymlink(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := verifyOne(t, Tree{Worktree: root}, Finding{Class: "bug", Evidence: "test ! -e pipe && exit 1"})
	if !got.Blocks {
		t.Fatalf("verdict = %+v, want the pipe left out of the scratch copy", got)
	}
}
