package run

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"komodo/internal/gate"
	"komodo/internal/line"
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

// pinRelease makes root a product repo whose profiles pin version, sends HOME to a temp dir, and swaps the
// download for one serving body and a SHA256SUMS with sum; it returns the release binary's path and the URLs fetched.
func pinRelease(t *testing.T, root, version string, body []byte, sum string) (string, *[]string) {
	t.Helper()
	profile := []byte(`{"release": "` + version + `", "roles": {}}`)
	if err := os.MkdirAll(filepath.Join(root, "komodo", "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "komodo", "AGENTS.md"), []byte("# Agent Rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"full", "economy"} {
		if err := os.WriteFile(filepath.Join(root, "komodo", "profiles", mode+".json"), profile, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KOMODO_RELEASE_URL", "https://example.test/download")
	name := gate.PlatformName()
	base := "https://example.test/download/v" + strings.TrimPrefix(version, "v") + "/"
	served := map[string][]byte{
		base + name:         body,
		base + "SHA256SUMS": []byte(sum + "  " + name + "\n" + strings.Repeat("0", 64) + "  komodo-other\n"),
	}
	fetched := &[]string{}
	old := fetch
	t.Cleanup(func() { fetch = old })
	fetch = func(_ context.Context, url string) ([]byte, error) {
		*fetched = append(*fetched, url)
		data, ok := served[url]
		if !ok {
			return nil, fmt.Errorf("could not download %s: 404", url)
		}
		return data, nil
	}
	return filepath.Join(home, ".komodo", "bin", name), fetched
}

func TestSyncFetchesThePinnedReleaseOutsideTheToolkit(t *testing.T) {
	body := []byte("the released binary")
	digest := sha256.Sum256(body)
	good := hex.EncodeToString(digest[:])
	cases := []struct {
		name      string
		pin       string
		sum       string
		installed string
		wantErr   string
		wantLine  string
		wantBody  bool
		dryRun    bool
		drop      string
		noHome    bool
	}{
		{name: "a release with no manifest installs nothing", pin: "1.0.0-beta.2", sum: good, drop: "SHA256SUMS",
			wantErr: "404"},
		{name: "a release with no binary installs nothing", pin: "1.0.0-beta.2", sum: good,
			drop: gate.PlatformName(), wantErr: "404"},
		{name: "no home installs nothing", pin: "1.0.0-beta.2", sum: good, noHome: true, wantErr: "HOME"},
		{name: "a pinned release installs once its checksum matches", pin: "1.0.0-beta.2", sum: good,
			wantLine: "binary: fetched release 1.0.0-beta.2, checksum verified", wantBody: true},
		{name: "a checksum mismatch installs nothing", pin: "1.0.0-beta.2", sum: strings.Repeat("f", 64),
			wantErr: "checksum mismatch"},
		{name: "the pinned release already installed fetches nothing", pin: "1.0.0-beta.2", sum: good,
			installed: "#!/bin/sh\necho 'komodo 1.0.0-beta.2 (abc)'\n", wantLine: "binary: already release 1.0.0-beta.2"},
		{name: "an older release installed is replaced", pin: "v1.0.0-beta.2", sum: good,
			installed: "#!/bin/sh\necho 'komodo 1.0.0-alpha.8 (abc)'\n", wantLine: "binary: fetched release", wantBody: true},
		{name: "no pin fetches nothing", wantLine: "binary: skipped, the profile pins no release"},
		{name: "a manifest without this platform installs nothing", pin: "1.0.0-beta.2", wantErr: "lists no"},
		{name: "a dry run fetches nothing", pin: "1.0.0-beta.2", sum: good, dryRun: true,
			wantLine: "binary: fetched release 1.0.0-beta.2 (dry run)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if (tc.installed != "" || tc.noHome) && runtime.GOOS == "windows" {
				t.Skip("the installed stand-in is a shell script, and the home comes from more than HOME")
			}
			root, _ := syncRepo(t)
			path, fetched := pinRelease(t, root, tc.pin, body, tc.sum)
			if tc.drop != "" {
				served := fetch
				fetch = func(ctx context.Context, url string) ([]byte, error) {
					if strings.HasSuffix(url, "/"+tc.drop) {
						return nil, fmt.Errorf("could not download %s: 404", url)
					}
					return served(ctx, url)
				}
			}
			if tc.noHome {
				t.Setenv("HOME", "")
			}
			if tc.installed != "" {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tc.installed), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			var out bytes.Buffer
			got, err := Sync(SyncOptions{Root: root, DryRun: tc.dryRun, Stdout: &out})
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
					t.Fatalf("a failed checksum left %s behind: %v", path, statErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.wantLine) {
				t.Fatalf("out = %q, want %q", out.String(), tc.wantLine)
			}
			if !tc.wantBody {
				if got != "" || len(*fetched) != 0 {
					t.Fatalf("path = %q, fetched = %v; nothing is due", got, *fetched)
				}
				return
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != path || !bytes.Equal(data, body) {
				t.Fatalf("path = %q, want %q; installed = %q", got, path, data)
			}
		})
	}
}

func TestHTTPGetReturnsTheBodyOrTheStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/asset" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, "asset body")
	}))
	body, err := httpGet(context.Background(), server.URL+"/asset")
	if err != nil || string(body) != "asset body" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
	if _, err := httpGet(context.Background(), server.URL+"/missing"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want the 404 named", err)
	}
	if _, err := httpGet(context.Background(), "://no-scheme"); err == nil {
		t.Fatal("a malformed URL must fail")
	}
	server.Close()
	if _, err := httpGet(context.Background(), server.URL+"/asset"); err == nil {
		t.Fatal("a server that is gone must fail")
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

// syncWorktreeRepo builds a root remoted at origin with a .komodo/wt worktree on its own branch,
// merged into main and pushed when merged is set, with an uncommitted edit when dirty is set.
func syncWorktreeRepo(t *testing.T, merged, dirty bool) (root, worktree string) {
	t.Helper()
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", "-b", "main", bare)
	root = t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	runGit(t, root, "remote", "add", "origin", bare)
	runGit(t, root, "commit", "--allow-empty", "-m", "seed")
	runGit(t, root, "push", "origin", "main")
	branch := "feat/side"
	worktree = filepath.Join(root, ".komodo", "wt", "side")
	runGit(t, root, "branch", branch)
	runGit(t, root, "worktree", "add", worktree, branch)
	runGit(t, worktree, "commit", "--allow-empty", "-m", "work")
	if merged {
		runGit(t, root, "merge", "--no-ff", "--no-edit", branch)
		runGit(t, root, "push", "origin", "main", branch)
	}
	if dirty {
		if err := os.WriteFile(filepath.Join(worktree, "dirty.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, worktree
}

func TestSyncRemovesACleanMergedWorktree(t *testing.T) {
	root, worktree := syncWorktreeRepo(t, true, false)
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree); err == nil {
		t.Fatalf("the clean merged worktree survived sync; out = %s", out.String())
	}
	if !strings.Contains(out.String(), "removed worktree") {
		t.Fatalf("out = %q, want sync to say what it removed", out.String())
	}
}

func TestSyncNamesADirtyWorktreeWithoutRemovingIt(t *testing.T) {
	root, worktree := syncWorktreeRepo(t, true, true)
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree); err != nil {
		t.Fatalf("a dirty worktree was removed: %v; out = %s", err, out.String())
	}
	if !strings.Contains(out.String(), "worktree: ") || !strings.Contains(out.String(), "is dirty, not removed") {
		t.Fatalf("out = %q, want the dirty worktree named", out.String())
	}
}

func TestSyncNamesAnUnmergedWorktreeWithoutRemovingIt(t *testing.T) {
	root, worktree := syncWorktreeRepo(t, false, false)
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree); err != nil {
		t.Fatalf("an unmerged worktree was removed: %v; out = %s", err, out.String())
	}
	if !strings.Contains(out.String(), "has not merged, not removed") {
		t.Fatalf("out = %q, want the unmerged worktree named", out.String())
	}
}

func TestSyncRemovesACleanMergedDetachedWorktreeAndItsTip(t *testing.T) {
	root, worktree := syncWorktreeRepo(t, false, false)
	runGit(t, root, "worktree", "remove", "--force", worktree)
	if err := line.AddDetached(root, "feat/side", "main", worktree); err != nil {
		t.Fatal(err)
	}
	runGit(t, worktree, "commit", "--allow-empty", "-m", "work")
	if err := line.Advance(root, "feat/side", worktree, gitOut(t, root, "rev-parse", line.TipRef("feat/side"))); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "push", "origin", line.TipRef("feat/side")+":refs/heads/feat/side", line.TipRef("feat/side")+":refs/heads/main")
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree); err == nil {
		t.Fatalf("the merged detached worktree survived sync; out = %s", out.String())
	}
	if gitOut(t, root, "for-each-ref", "refs/komodo/") != "" {
		t.Fatalf("the tip ref survived sync; out = %s", out.String())
	}
}

func TestSyncNamesAnUnmergedDetachedWorktreeByItsTrackedBranch(t *testing.T) {
	root, worktree := syncWorktreeRepo(t, false, false)
	runGit(t, root, "worktree", "remove", "--force", worktree)
	if err := line.AddDetached(root, "feat/side", "main", worktree); err != nil {
		t.Fatal(err)
	}
	runGit(t, worktree, "commit", "--allow-empty", "-m", "work")
	if err := line.Advance(root, "feat/side", worktree, gitOut(t, root, "rev-parse", line.TipRef("feat/side"))); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "on feat/side has not merged, not removed") {
		t.Fatalf("out = %q, want the detached worktree named by its tracked branch", out.String())
	}
}

func TestSyncDetachesACleanAttachedWorktree(t *testing.T) {
	root, worktree := syncWorktreeRepo(t, false, false)
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if got := gitOut(t, worktree, "config", "--worktree", "--get", "komodo.branch"); got != "feat/side" {
		t.Fatalf("komodo.branch = %q; out = %s", got, out.String())
	}
	if gitOut(t, root, "rev-parse", line.TipRef("feat/side")) != gitOut(t, worktree, "rev-parse", "HEAD") {
		t.Fatalf("the tip ref does not hold the worktree's commit; out = %s", out.String())
	}
	if exec.Command("git", "-C", worktree, "symbolic-ref", "-q", "HEAD").Run() == nil {
		t.Fatalf("the worktree still holds a branch; out = %s", out.String())
	}
	if gitOut(t, root, "branch", "--list", "feat/side") == "" {
		t.Fatal("the person's branch was deleted")
	}
}

func TestSyncDetachNamesADirtyWorktreeAndLeavesItAttached(t *testing.T) {
	root, worktree := syncWorktreeRepo(t, false, true)
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if exec.Command("git", "-C", worktree, "symbolic-ref", "-q", "HEAD").Run() != nil {
		t.Fatalf("a dirty worktree was detached; out = %s", out.String())
	}
	if !strings.Contains(out.String(), "still holds feat/side, not detached") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestSyncDryRunRemovesNoWorktree(t *testing.T) {
	root, worktree := syncWorktreeRepo(t, true, false)
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, DryRun: true, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree); err != nil {
		t.Fatalf("a dry run removed a worktree: %v; out = %s", err, out.String())
	}
	if !strings.Contains(out.String(), "worktree: skipped in a dry run") {
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
			if gitOut(t, root, "branch", "--list", "chore/cleanup-epic-01") != "" || gitOut(t, root, "for-each-ref", "refs/komodo/") != "" {
				t.Fatal("the cleanup left a local branch or tip ref behind")
			}
		})
	}
}

// TestSyncCleanupKeepsAnEpicOpenWhenAFileFailsToParse proves a group file whose checkbox line
// misses the task grammar never counts as ended, so its epic's cleanup PR is never opened.
func TestSyncCleanupKeepsAnEpicOpenWhenAFileFailsToParse(t *testing.T) {
	malformed := "## [TG-01.2] A group [P: H] [READY]\n\n```yaml\ntype: feat\nversion: 0.1.0\nepic: EPIC-01\n```\n\n" +
		"- [ ] not a task line, missing the bold id\n"
	root, bare := cleanupRepo(t, map[string]string{
		"TG-01.1-a.md": groupFile("TG-01.1", "EPIC-01", true),
		"TG-01.2-b.md": malformed,
	})
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
		t.Fatalf("PRs opened = %d, want none; a file that fails to parse must keep its epic open; out = %s", created, out.String())
	}
	if !strings.Contains(out.String(), "cleanup: no epic's files outlived it") {
		t.Fatalf("out = %q", out.String())
	}
	if heads := gitOut(t, bare, "branch", "--list", "chore/cleanup-epic-01"); heads != "" {
		t.Fatalf("origin holds %s though EPIC-01 is still open", heads)
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

// TestSyncCleanupReturnsAForgeThatRefusesThePR proves a refused PR still cleans up the worktree, its
// tip ref and the branch it already pushed, so a retry opens the PR instead of failing forever.
func TestSyncCleanupReturnsAForgeThatRefusesThePR(t *testing.T) {
	root, bare := cleanupRepo(t, map[string]string{"TG-01.1-a.md": groupFile("TG-01.1", "EPIC-01", true)})
	client := &pr.Client{Run: func(string, ...string) (string, error) {
		return "", errors.New("HTTP 422")
	}}
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out, PR: client}); err == nil || !strings.Contains(err.Error(), "HTTP 422") {
		t.Fatalf("Sync = %v; a refused cleanup PR must be returned; out = %s", err, out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".komodo", "wt", "cleanup-epic-01")); err == nil {
		t.Fatal("a cleanup worktree survived a refused PR")
	}
	if gitOut(t, root, "for-each-ref", "refs/komodo/") != "" {
		t.Fatal("a cleanup's tip ref survived a refused PR")
	}
	if heads := gitOut(t, bare, "branch", "--list", "chore/cleanup-epic-01"); heads != "" {
		t.Fatal("origin kept the pushed cleanup branch though its PR was refused")
	}

	var created [][]string
	client.Run = func(_ string, args ...string) (string, error) {
		created = append(created, args)
		return "https://example.invalid/pr/9", nil
	}
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out, PR: client}); err != nil {
		t.Fatalf("Sync after a retry = %v; out = %s", err, out.String())
	}
	if len(created) == 0 {
		t.Fatal("a retried cleanup never opened its pull request")
	}
}

// TestSyncCleanupRemovesTheWorktreeAndTipWhenThePushFails proves a push a server hook refuses still
// cleans up the worktree and tip ref, so a retry re-cuts instead of erroring on a registered path.
func TestSyncCleanupRemovesTheWorktreeAndTipWhenThePushFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the server hook is a shell script")
	}
	root, bare := cleanupRepo(t, map[string]string{"TG-01.1-a.md": groupFile("TG-01.1", "EPIC-01", true)})
	hook := filepath.Join(bare, "hooks", "pre-receive")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out}); err == nil {
		t.Fatalf("Sync = nil; a refused push must be returned; out = %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".komodo", "wt", "cleanup-epic-01")); err == nil {
		t.Fatal("a cleanup worktree survived a refused push")
	}
	if gitOut(t, root, "for-each-ref", "refs/komodo/") != "" {
		t.Fatal("a cleanup's tip ref survived a refused push")
	}

	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	client := &pr.Client{Run: func(_ string, args ...string) (string, error) {
		return "https://example.invalid/pr/9", nil
	}}
	if _, err := Sync(SyncOptions{Root: root, Stdout: &out, PR: client}); err != nil {
		t.Fatalf("Sync after the push works = %v; out = %s", err, out.String())
	}
}

// TestSyncBinaryReturnsItsOwnRebuildEvenWhenAPostMergeHookWouldStampFirst proves a real post-merge
// hook sees KOMODO_SYNC during the merge and skips, so syncBinary's own rebuild is the one that runs.
func TestSyncBinaryReturnsItsOwnRebuildEvenWhenAPostMergeHookWouldStampFirst(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the installed hook is a shell script")
	}
	root, ahead := syncRepo(t)
	builds, installs := fakeBuild(t)
	stale := gitOut(t, root, "rev-parse", "HEAD")
	toolkitCheckout(t, root, stale)
	hook := "#!/bin/sh\nif [ -z \"$" + gate.SyncEnv + "\" ]; then git rev-parse HEAD > bin/" + BuiltFrom + "; fi\n"
	if err := os.WriteFile(filepath.Join(root, ".git", "hooks", "post-merge"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	got, err := Sync(SyncOptions{Root: root, Stdout: &out})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "bin", gate.LocalTarget().Name)
	if got != want {
		t.Fatalf("Sync returned %q, want %q; a hook that stamped first must never hide the rebuilt path", got, want)
	}
	if *builds != 1 || *installs != 1 {
		t.Fatalf("builds = %d, installs = %d, want 1 each; out = %s", *builds, *installs, out.String())
	}
	if !strings.Contains(out.String(), "binary: rebuilt") {
		t.Fatalf("out = %q", out.String())
	}
	recorded, err := os.ReadFile(filepath.Join(root, "bin", BuiltFrom))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(recorded)) != ahead {
		t.Fatalf("marker = %q, want %q", recorded, ahead)
	}
}

func TestSyncBinaryDropsTheStampOnADirtyTree(t *testing.T) {
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
	// A dirty tree's build is no commit's, so the stamp goes and the next clean sync rebuilds.
	if _, err := os.Stat(filepath.Join(root, "bin", BuiltFrom)); !os.IsNotExist(err) {
		t.Fatalf("marker still present (%v); a dirty build must drop it", err)
	}
}
