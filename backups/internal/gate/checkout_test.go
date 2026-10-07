package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checkoutRepo is a repo with one committed file, a.txt holding "committed".
func checkoutRepo(t *testing.T) (root, head string) {
	t.Helper()
	root = t.TempDir()
	gitCommand(t, root, "init", "-q", "-b", "feat/a")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("committed"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "a.txt")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "seed")
	return root, gitCommand(t, root, "rev-parse", "HEAD")
}

func TestCleanCheckoutIsTheRootWhenItIsCleanAtTheCommit(t *testing.T) {
	root, head := checkoutRepo(t)
	dir, cleanup, err := CleanCheckout(root, head)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if dir != root {
		t.Fatalf("dir = %s, want the root itself", dir)
	}
}

// TestCleanCheckoutProvesTheCommitNotADirtyTree proves an uncommitted edit never reaches the
// checked-out tree, and cleanup leaves no worktree behind.
func TestCleanCheckoutProvesTheCommitNotADirtyTree(t *testing.T) {
	root, head := checkoutRepo(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("uncommitted"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir, cleanup, err := CleanCheckout(root, head)
	if err != nil {
		t.Fatal(err)
	}
	if dir == root {
		t.Fatal("a dirty tree was gated in place")
	}
	if data, err := os.ReadFile(filepath.Join(dir, "a.txt")); err != nil || string(data) != "committed" {
		t.Fatalf("a.txt = %q, %v; want the committed content", data, err)
	}
	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("stat = %v; cleanup must remove the checkout", err)
	}
	if list := gitCommand(t, root, "worktree", "list"); strings.Count(list, "\n") != 0 {
		t.Fatalf("worktree list = %q; cleanup must unregister the checkout", list)
	}
}

func TestCleanCheckoutChecksOutAPushedCommitOtherThanHead(t *testing.T) {
	root, first := checkoutRepo(t)
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "second")
	dir, cleanup, err := CleanCheckout(root, first)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if dir == root || gitCommand(t, dir, "rev-parse", "HEAD") != first {
		t.Fatalf("dir = %s; want a checkout at %s", dir, first)
	}
}
