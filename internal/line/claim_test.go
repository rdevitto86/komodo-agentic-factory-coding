package line

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"komodo/internal/git"
)

func TestWriteClaimWritesUnderTheGitCommonDir(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "a.txt", "a\n", "seed")
	if _, err := WriteClaim(root, "feat/x", "session-a"); err != nil {
		t.Fatal(err)
	}
	path, err := ClaimPath(root, "feat/x")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := git.Run(root, "rev-parse", "--git-common-dir")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	if want := filepath.Join(dir, "komodo-claims", "feat_x.json"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	claim, found, err := ReadClaim(root, "feat/x")
	if err != nil || !found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
	if claim.Session != "session-a" || claim.Branch != "feat/x" {
		t.Fatalf("claim = %+v", claim)
	}
}

func TestWriteClaimRefusesAnotherSessionsLiveClaim(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "a.txt", "a\n", "seed")
	if _, err := WriteClaim(root, "feat/x", "session-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteClaim(root, "feat/x", "session-b"); err == nil {
		t.Fatal("want a refusal writing over another session's live claim")
	}
}

func TestWriteClaimLetsTheSameSessionRenewItsOwnClaim(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "a.txt", "a\n", "seed")
	if _, err := WriteClaim(root, "feat/x", "session-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteClaim(root, "feat/x", "session-a"); err != nil {
		t.Fatalf("the same session renewing its own claim must not be refused: %v", err)
	}
}

func TestWriteClaimReportsAStaleClaimInsteadOfTakingItSilently(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "a.txt", "a\n", "seed")
	path, err := ClaimPath(root, "feat/x")
	if err != nil {
		t.Fatal(err)
	}
	old := Claim{Session: "session-a", Branch: "feat/x", Started: time.Now().UTC().Add(-13 * time.Hour)}
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	stale, err := WriteClaim(root, "feat/x", "session-b")
	if err != nil {
		t.Fatalf("a stale claim must be reported, not refused: %v", err)
	}
	if stale == nil || stale.Session != "session-a" {
		t.Fatalf("stale = %+v, want session-a's old claim reported", stale)
	}
	claim, found, err := ReadClaim(root, "feat/x")
	if err != nil || !found || claim.Session != "session-b" {
		t.Fatalf("claim = %+v, found = %v, err = %v; want session-b to now hold it", claim, found, err)
	}
}

func TestReleaseClaimRemovesTheFile(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "a.txt", "a\n", "seed")
	if _, err := WriteClaim(root, "feat/x", "session-a"); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseClaim(root, "feat/x"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := ReadClaim(root, "feat/x"); err != nil || found {
		t.Fatalf("found = %v, err = %v; want the claim gone", found, err)
	}
}

func TestReleaseClaimToleratesNoClaim(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "a.txt", "a\n", "seed")
	if err := ReleaseClaim(root, "feat/x"); err != nil {
		t.Fatalf("releasing a branch with no claim must not error: %v", err)
	}
}

func TestClaimIsVisibleAcrossWorktreesOfTheSameRepo(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "a.txt", "a\n", "seed")
	worktree := filepath.Join(t.TempDir(), "wt")
	if err := AddWorktree(root, "task/x", "main", worktree); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteClaim(root, "feat/x", "session-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteClaim(worktree, "feat/x", "session-b"); err == nil {
		t.Fatal("a claim written from the root worktree must stop a write from another worktree of the same repo")
	}
}
