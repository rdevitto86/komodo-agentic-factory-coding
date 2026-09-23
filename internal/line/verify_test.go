package line

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBeforeReviewAndAfterPublishCommandsReadTheCommandsFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, StateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"before_review":"make lint","after_publish":"make notify"}`
	if err := os.WriteFile(filepath.Join(dir, "commands.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := BeforeReviewCommand(root); got != "make lint" {
		t.Fatalf("before_review = %q", got)
	}
	if got := AfterPublishCommand(root); got != "make notify" {
		t.Fatalf("after_publish = %q", got)
	}
}

func TestBeforeReviewAndAfterPublishCommandsAreEmptyByDefault(t *testing.T) {
	root := t.TempDir()
	if got := BeforeReviewCommand(root); got != "" {
		t.Fatalf("before_review = %q", got)
	}
	if got := AfterPublishCommand(root); got != "" {
		t.Fatalf("after_publish = %q", got)
	}
}
