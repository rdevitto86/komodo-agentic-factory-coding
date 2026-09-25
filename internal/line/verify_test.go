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
	if got := BeforeReviewCommand(root, root); got != "make lint" {
		t.Fatalf("before_review = %q", got)
	}
	if got := AfterPublishCommand(root, root); got != "make notify" {
		t.Fatalf("after_publish = %q", got)
	}
}

func TestBeforeReviewAndAfterPublishCommandsAreEmptyByDefault(t *testing.T) {
	root := t.TempDir()
	if got := BeforeReviewCommand(root, root); got != "" {
		t.Fatalf("before_review = %q", got)
	}
	if got := AfterPublishCommand(root, root); got != "" {
		t.Fatalf("after_publish = %q", got)
	}
}

func TestBeforeReviewAndAfterPublishCommandsFallbackAndWorktreeWins(t *testing.T) {
	tests := []struct {
		name                      string
		createWorktreeCommands    bool
		expectedBeforeReview      string
		expectedAfterPublish      string
	}{
		{
			name:                 "fallback from root",
			createWorktreeCommands: false,
			expectedBeforeReview:   "make lint",
			expectedAfterPublish:   "make notify",
		},
		{
			name:                   "worktree wins",
			createWorktreeCommands: true,
			expectedBeforeReview:   "make test",
			expectedAfterPublish:   "make release",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			worktree := t.TempDir()
			rootDir := filepath.Join(root, StateDir)
			if err := os.MkdirAll(rootDir, 0o755); err != nil {
				t.Fatal(err)
			}
			rootBody := `{"before_review":"make lint","after_publish":"make notify"}`
			if err := os.WriteFile(filepath.Join(rootDir, "commands.json"), []byte(rootBody), 0o644); err != nil {
				t.Fatal(err)
			}
			if tt.createWorktreeCommands {
				wtDir := filepath.Join(worktree, StateDir)
				if err := os.MkdirAll(wtDir, 0o755); err != nil {
					t.Fatal(err)
				}
				wtBody := `{"before_review":"make test","after_publish":"make release"}`
				if err := os.WriteFile(filepath.Join(wtDir, "commands.json"), []byte(wtBody), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := BeforeReviewCommand(root, worktree); got != tt.expectedBeforeReview {
				t.Fatalf("before_review = %q, want %q", got, tt.expectedBeforeReview)
			}
			if got := AfterPublishCommand(root, worktree); got != tt.expectedAfterPublish {
				t.Fatalf("after_publish = %q, want %q", got, tt.expectedAfterPublish)
			}
		})
	}
}
