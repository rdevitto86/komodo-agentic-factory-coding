package run

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/gate"
	"komodo/internal/pr"
)

// syncRepo builds a root on main whose origin holds one commit the root lacks, and returns that commit.
func syncRepo(t *testing.T) (root, ahead string) {
	t.Helper()
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", "-b", "main", bare)
	root = t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	runGit(t, root, "remote", "add", "origin", bare)
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "file.txt")
	runGit(t, root, "commit", "-m", "seed")
	runGit(t, root, "push", "origin", "main")
	other := t.TempDir()
	runGit(t, "", "clone", bare, other)
	runGit(t, other, "config", "user.email", "a@example.com")
	runGit(t, other, "config", "user.name", "a")
	runGit(t, other, "commit", "--allow-empty", "-m", "ahead")
	runGit(t, other, "push", "origin", "main")
	return root, gitOut(t, other, "rev-parse", "HEAD")
}

// gitOut runs one git command in dir and returns its trimmed output, failing the test on error.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// fakeBuild swaps the build and hook install for counters, restoring both when the test ends.
func fakeBuild(t *testing.T) (builds, installs *int) {
	t.Helper()
	builds, installs = new(int), new(int)
	oldBuild, oldInstall := buildLocal, installHooks
	t.Cleanup(func() { buildLocal, installHooks = oldBuild, oldInstall })
	buildLocal = func(root string, _ io.Writer) (string, error) {
		*builds++
		return filepath.Join(root, "bin", gate.LocalTarget().Name), nil
	}
	installHooks = func(string) ([]string, error) {
		*installs++
		return nil, nil
	}
	return builds, installs
}

// toolkitCheckout makes root look like the toolkit's own checkout with a built binary and a build marker.
func toolkitCheckout(t *testing.T, root, builtFrom string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "cmd", "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "komodo", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", gate.LocalTarget().Name), []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", BuiltFrom), []byte(builtFrom+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSyncFastForwardsACleanDefaultBranchBehindOrigin(t *testing.T) {
	root, ahead := syncRepo(t)
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if head := gitOut(t, root, "rev-parse", "HEAD"); head != ahead {
		t.Fatalf("HEAD = %s, want origin's %s; out = %s", head, ahead, out.String())
	}
	if parents := gitOut(t, root, "rev-list", "--merges", "HEAD"); parents != "" {
		t.Fatalf("sync made a merge commit: %s", parents)
	}
	if !strings.Contains(out.String(), "root: updated") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestSyncDryRunWritesNothing(t *testing.T) {
	root, _ := syncRepo(t)
	runGit(t, root, "fetch", "origin")
	before := gitOut(t, root, "rev-parse", "HEAD")
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, DryRun: true, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if head := gitOut(t, root, "rev-parse", "HEAD"); head != before {
		t.Fatalf("a dry run moved HEAD from %s to %s", before, head)
	}
	if !strings.Contains(out.String(), "root: updated") || !strings.Contains(out.String(), "(dry run)") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestSyncLeavesADirtyTreeAlone(t *testing.T) {
	root, _ := syncRepo(t)
	before := gitOut(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if head := gitOut(t, root, "rev-parse", "HEAD"); head != before {
		t.Fatalf("HEAD moved on a dirty tree: %s to %s", before, head)
	}
	if !strings.Contains(out.String(), "uncommitted changes") {
		t.Fatalf("out = %q; a skipped step must say why", out.String())
	}
}

func TestSyncSkipsABranchOtherThanTheDefault(t *testing.T) {
	root, _ := syncRepo(t)
	runGit(t, root, "checkout", "-b", "feat/side")
	before := gitOut(t, root, "rev-parse", "HEAD")
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if head := gitOut(t, root, "rev-parse", "HEAD"); head != before {
		t.Fatalf("HEAD moved off the default branch: %s to %s", before, head)
	}
	if !strings.Contains(out.String(), "not the default branch") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestSyncBinary(t *testing.T) {
	cases := []struct {
		name     string
		stale    bool
		dryRun   bool
		builds   int
		wantLine string
	}{
		{name: "a stale marker rebuilds", stale: true, builds: 1, wantLine: "binary: rebuilt"},
		{name: "a current marker does not", stale: false, builds: 0, wantLine: "binary: already current"},
		{name: "a dry run reports and builds nothing", stale: true, dryRun: true, builds: 0, wantLine: "binary: rebuilt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, ahead := syncRepo(t)
			builds, installs := fakeBuild(t)
			marker := ahead
			if tc.stale {
				marker = gitOut(t, root, "rev-parse", "HEAD")
			}
			toolkitCheckout(t, root, marker)
			if tc.dryRun {
				runGit(t, root, "fetch", "origin")
			}
			var out bytes.Buffer
			if _, err := Sync(SyncOptions{Root: root, DryRun: tc.dryRun, Stdout: &out}); err != nil {
				t.Fatal(err)
			}
			if *builds != tc.builds || *installs != tc.builds {
				t.Fatalf("builds = %d, installs = %d, want %d; out = %s", *builds, *installs, tc.builds, out.String())
			}
			if !strings.Contains(out.String(), tc.wantLine) {
				t.Fatalf("out = %q, want %q", out.String(), tc.wantLine)
			}
			recorded, err := os.ReadFile(filepath.Join(root, "bin", BuiltFrom))
			if err != nil {
				t.Fatal(err)
			}
			want := ahead
			if tc.dryRun {
				want = marker
			}
			if strings.TrimSpace(string(recorded)) != want {
				t.Fatalf("marker = %q, want %q", recorded, want)
			}
		})
	}
}

// groupFile renders one docs/backlog group file of epic whose one task is ticked when done.
func groupFile(id, epic string, done bool) string {
	box := " "
	if done {
		box = "x"
	}
	return "## [" + id + "] A group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 0.1.0\nepic: " + epic + "\n```\n\n" +
		"- [" + box + "] **TSK-" + strings.TrimPrefix(id, "TG-") + ".1** Do it\n  - files: `a.go`\n"
}

// cleanupRepo builds a root on main, current with its bare origin, holding files under docs/backlog.
func cleanupRepo(t *testing.T, files map[string]string) (root, bare string) {
	t.Helper()
	bare = filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", "-b", "main", bare)
	root = t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	runGit(t, root, "remote", "add", "origin", bare)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/.komodo/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "backlog"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(root, "docs", "backlog", name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "seed")
	runGit(t, root, "push", "origin", "main")
	runGit(t, root, "fetch", "origin")
	return root, bare
}

func TestSyncOpensACleanupPRForAnEpicWhoseFilesOutlivedIt(t *testing.T) {
	cases := []struct {
		name     string
		lastDone bool
		dryRun   bool
		wantPRs  int
		wantLine string
	}{
		{name: "every group shipped", lastDone: true, wantPRs: 1, wantLine: "cleanup: opened chore/cleanup-epic-01"},
		{name: "a group still open", lastDone: false, wantPRs: 0, wantLine: "cleanup: no epic's files outlived it"},
		{name: "a dry run opens nothing", lastDone: true, dryRun: true, wantPRs: 0, wantLine: "(dry run)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, bare := cleanupRepo(t, map[string]string{
				"TG-01.1-a.md": groupFile("TG-01.1", "EPIC-01", true),
				"TG-01.2-b.md": groupFile("TG-01.2", "EPIC-01", tc.lastDone),
				"TG-02.1-c.md": groupFile("TG-02.1", "EPIC-02", false),
			})
			var created [][]string
			client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
				created = append(created, args)
				return "https://example.invalid/pr/9", nil
			}}
			var out bytes.Buffer
			if _, err := Sync(SyncOptions{Root: root, DryRun: tc.dryRun, Stdout: &out, PR: client}); err != nil {
				t.Fatal(err)
			}
			if len(created) != tc.wantPRs {
				t.Fatalf("PRs opened = %v, want %d; out = %s", created, tc.wantPRs, out.String())
			}
			if !strings.Contains(out.String(), tc.wantLine) {
				t.Fatalf("out = %q, want %q", out.String(), tc.wantLine)
			}
			heads := gitOut(t, bare, "branch", "--list", "chore/cleanup-epic-01")
			if tc.wantPRs == 0 {
				if heads != "" {
					t.Fatalf("origin holds %s though no cleanup PR was due", heads)
				}
				return
			}
			args := strings.Join(created[0], " ")
			if !strings.Contains(args, "--base main") || !strings.Contains(args, "--head chore/cleanup-epic-01") {
				t.Fatalf("pr create = %q, want the cleanup branch against main", args)
			}
			left := gitOut(t, bare, "ls-tree", "--name-only", "chore/cleanup-epic-01", "docs/backlog/")
			if left != "docs/backlog/TG-02.1-c.md" {
				t.Fatalf("cleanup branch keeps %q, want only the open epic's file", left)
			}
			if _, err := os.Stat(filepath.Join(root, ".komodo", "wt", "cleanup-epic-01")); err == nil {
				t.Fatal("the cleanup worktree outlived its PR")
			}
		})
	}
}

func TestSyncCleanupSkipsWhatItCannotOrNeedNotOpen(t *testing.T) {
	ended := map[string]string{"TG-01.1-a.md": groupFile("TG-01.1", "EPIC-01", true)}
	cases := []struct {
		name     string
		arrange  func(t *testing.T, root, bare string)
		wantLine string
	}{
		{
			name: "origin already holds the cleanup branch",
			arrange: func(t *testing.T, root, _ string) {
				runGit(t, root, "push", "origin", "main:refs/heads/chore/cleanup-epic-01")
			},
			wantLine: "cleanup: EPIC-01 already has chore/cleanup-epic-01 on origin",
		},
		{
			name: "origin cannot be reached",
			arrange: func(t *testing.T, root, _ string) {
				runGit(t, root, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
			},
			wantLine: "cleanup: skipped, cannot reach origin",
		},
		{
			name: "origin has no default branch",
			arrange: func(t *testing.T, root, _ string) {
				runGit(t, root, "update-ref", "-d", "refs/remotes/origin/main")
				runGit(t, root, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
			},
			wantLine: "cleanup: skipped, origin has no branch",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, bare := cleanupRepo(t, ended)
			tc.arrange(t, root, bare)
			var created int
			client := &pr.Client{Run: func(string, ...string) (string, error) {
				created++
				return "", nil
			}}
			var out bytes.Buffer
			if _, err := Sync(SyncOptions{Root: root, Stdout: &out, PR: client}); err != nil {
				t.Fatal(err)
			}
			if created != 0 {
				t.Fatalf("PRs opened = %d, want none; out = %s", created, out.String())
			}
			if !strings.Contains(out.String(), tc.wantLine) {
				t.Fatalf("out = %q, want %q", out.String(), tc.wantLine)
			}
		})
	}
}

func TestSyncCleanupReturnsAForgeThatRefusesThePR(t *testing.T) {
	root, _ := cleanupRepo(t, map[string]string{"TG-01.1-a.md": groupFile("TG-01.1", "EPIC-01", true)})
	client := &pr.Client{Run: func(string, ...string) (string, error) {
		return "", errors.New("HTTP 422")
	}}
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out, PR: client}); err == nil || !strings.Contains(err.Error(), "HTTP 422") {
		t.Fatalf("Sync = %v; a refused cleanup PR must be returned; out = %s", err, out.String())
	}
}

func TestSyncBinaryDoesNotStampDirtyTree(t *testing.T) {
	root, ahead := syncRepo(t)
	builds, installs := fakeBuild(t)
	toolkitCheckout(t, root, ahead)
	runGit(t, root, "fetch", "origin")
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if *builds != 1 {
		t.Fatalf("builds = %d, want 1; out = %s", *builds, out.String())
	}
	if *installs != 0 {
		t.Fatalf("installs = %d, want 0; out = %s", *installs, out.String())
	}
	recorded, err := os.ReadFile(filepath.Join(root, "bin", BuiltFrom))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(recorded)) != ahead {
		t.Fatalf("marker = %q, want %q (unchanged)", recorded, ahead)
	}
}
