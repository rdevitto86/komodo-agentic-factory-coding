package line

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
	"komodo/internal/backlog/backlogtest"
)

// reseed drops root's backlog tree and seeds it again from text's legacy grammar.
func reseed(t *testing.T, root, text string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(root, backlog.GroupFilesDir)); err != nil {
		t.Fatal(err)
	}
	backlogtest.SeedText(t, root, text)
}

// relGroupPath is a group's the group index file relative to its repo root, slash separated.
func relGroupPath(groupID string) string {
	return filepath.ToSlash(filepath.Join(backlog.GroupDirPath("", groupID), backlog.GroupFileName))
}

// relTaskPath is a task's file relative to its repo root, slash separated.
func relTaskPath(taskID string) string {
	return filepath.ToSlash(filepath.Join(backlog.GroupDirPath("", backlog.GroupIDOfTask(taskID)), backlog.TaskFileName(taskID)))
}

// groupPath is a group's the group index file under root.
func groupPath(root, groupID string) string {
	return filepath.Join(root, filepath.FromSlash(relGroupPath(groupID)))
}

// taskPath is a task's file under root.
func taskPath(root, taskID string) string {
	return filepath.Join(root, filepath.FromSlash(relTaskPath(taskID)))
}

// readGroupText reads a group's index and task files under root as one text, as the tree assembles them.
func readGroupText(t *testing.T, root, groupID string) string {
	t.Helper()
	dir, found, err := backlog.Locate(root, groupID)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("%s is not under %s", groupID, root)
	}
	var parts []string
	for _, path := range dir.Paths() {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, string(data))
	}
	return strings.Join(parts, "\n")
}

// copyBacklogTree copies every file under src's docs/backlog into the same place under dst.
func copyBacklogTree(t *testing.T, src, dst string) {
	t.Helper()
	base := filepath.Join(src, backlog.GroupFilesDir)
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// commitAll stages every change under root and commits it under message.
func commitAll(t *testing.T, root, message string) {
	t.Helper()
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", message)
}
