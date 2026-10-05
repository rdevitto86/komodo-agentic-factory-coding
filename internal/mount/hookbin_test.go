package mount

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"komodo/internal/changelog"
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

// versionsFromBodies makes each stand-in binary report its own body as its version, counting the reads.
func versionsFromBodies(t *testing.T) *int {
	t.Helper()
	calls := 0
	previous := binaryVersion
	binaryVersion = func(path string) string {
		calls++
		data, _ := os.ReadFile(path)
		return string(data)
	}
	t.Cleanup(func() { binaryVersion = previous })
	return &calls
}

// captureRefusals collects what Publish prints when it keeps the installed binary.
func captureRefusals(t *testing.T) *strings.Builder {
	t.Helper()
	var out strings.Builder
	previous := refusalOut
	refusalOut = &out
	t.Cleanup(func() { refusalOut = previous })
	return &out
}

func TestPublishOrdersTheInstalledBinaryAndTheCandidateByVersion(t *testing.T) {
	cases := []struct {
		name, installed, candidate, want string
		refused                          bool
	}{
		{"older candidate", "1.0.0-beta.6", "1.0.0-beta.5", "1.0.0-beta.6", true},
		{"unstamped candidate", "1.0.0-beta.5", "dev", "1.0.0-beta.5", true},
		{"newer candidate", "1.0.0-beta.5", "1.0.0-beta.6", "1.0.0-beta.6", false},
		{"stable over prerelease", "1.0.0-rc.1", "1.0.0", "1.0.0", false},
		{"unstamped installed", "dev", "1.0.0-beta.5", "1.0.0-beta.5", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			versionsFromBodies(t)
			out := captureRefusals(t)
			target, _ := HookPath()
			Publish(writeBinary(t, t.TempDir(), tc.installed))
			candidate := writeBinary(t, t.TempDir(), tc.candidate)
			if got := Publish(candidate); got != target {
				t.Fatalf("Publish = %s, want %s", got, target)
			}
			if data, _ := os.ReadFile(target); string(data) != tc.want {
				t.Fatalf("installed = %q, want %q", data, tc.want)
			}
			lines := strings.Split(strings.TrimSpace(out.String()), "\n")
			if !tc.refused {
				if out.Len() != 0 {
					t.Fatalf("printed %q; a replacement prints nothing", out.String())
				}
				return
			}
			if len(lines) != 1 || !strings.Contains(lines[0], tc.installed) || !strings.Contains(lines[0], candidate) {
				t.Fatalf("refusal = %q, want one line naming %s and the candidate", out.String(), tc.installed)
			}
			if changelog.Valid(tc.candidate) && !strings.Contains(lines[0], tc.candidate) {
				t.Fatalf("refusal = %q, want the candidate's version %s named", lines[0], tc.candidate)
			}
		})
	}
}

func TestBinaryBuildReadsTheVersionCommandsOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	dir := t.TempDir()
	stamped := filepath.Join(dir, "stamped")
	if err := os.WriteFile(stamped, []byte("#!/bin/sh\necho 'komodo 1.0.0-beta.6 (0123456789ab)'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if version, commit := BinaryBuild(stamped); version != "1.0.0-beta.6" || commit != "0123456789ab" {
		t.Fatalf("BinaryBuild = %q, %q; want 1.0.0-beta.6 and 0123456789ab", version, commit)
	}
	if version, commit := BinaryBuild(writeBinary(t, dir, "not a program")); version != "" || commit != "" {
		t.Fatalf("BinaryBuild of a file that cannot run = %q, %q; want none", version, commit)
	}
}

func TestBinaryVersionReadsAStampAndNeverRunsATestBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-ins are shell scripts")
	}
	dir := t.TempDir()
	script := []byte("#!/bin/sh\necho 'komodo 1.0.0-beta.6 (0123456789ab)'\n")
	stamped, test := filepath.Join(dir, "komodo"), filepath.Join(dir, "mount.test")
	for _, path := range []string{stamped, test} {
		if err := os.WriteFile(path, script, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := binaryVersion(stamped); got != "1.0.0-beta.6" {
		t.Fatalf("binaryVersion = %q, want 1.0.0-beta.6", got)
	}
	if got := binaryVersion(test); got != "" {
		t.Fatalf("binaryVersion of a test binary = %q, want it never run", got)
	}
}

func TestPublishSkipsAByteEqualBinaryWithoutReadingVersions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	Publish(writeBinary(t, t.TempDir(), "1.0.0-beta.5"))
	calls := versionsFromBodies(t)
	if got, _ := HookPath(); Publish(writeBinary(t, t.TempDir(), "1.0.0-beta.5")) != got {
		t.Fatalf("Publish of an identical binary did not name the fixed path")
	}
	if *calls != 0 {
		t.Fatalf("read %d versions; a byte-equal binary skips before any", *calls)
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

// toolkitCheckout is a checkout with its own minimal .git, a cmd/komodo, and a bin/komodo reporting version.
func toolkitCheckout(t *testing.T, version string) (root, built string) {
	t.Helper()
	// MainCheckout reports git's resolved path, so the fixture resolves macOS's /var symlink too.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Join(".git", "objects"), filepath.Join(".git", "refs"), filepath.Join("cmd", "komodo"), "bin"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	built = filepath.Join(root, "bin", "komodo"+exe)
	for path, body := range map[string]string{
		filepath.Join(root, ".git", "HEAD"):             "ref: refs/heads/main\n",
		filepath.Join(root, "cmd", "komodo", "main.go"): "package main\n",
		built: version,
	} {
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root, built
}

func TestLatestBinaryPicksTheHighestVersion(t *testing.T) {
	cases := []struct {
		name, checkout, release string
		wantRelease             bool
	}{
		{"an older checkout build loses to a newer release", "1.0.0-beta.5", "1.0.0-beta.6", true},
		{"a newer checkout build beats an older release", "1.0.0-rc.1", "1.0.0-beta.6", false},
		{"an unstamped checkout build loses to a release", "dev", "1.0.0-beta.5", true},
		{"a tie keeps the checkout build", "1.0.0", "1.0.0", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			versionsFromBodies(t)
			root, built := toolkitCheckout(t, tc.checkout)
			exe := ""
			if runtime.GOOS == "windows" {
				exe = ".exe"
			}
			release := filepath.Join(home, ".komodo", "bin", "komodo-"+runtime.GOOS+"-"+runtime.GOARCH+exe)
			if err := os.MkdirAll(filepath.Dir(release), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(release, []byte(tc.release), 0o755); err != nil {
				t.Fatal(err)
			}
			want := built
			if tc.wantRelease {
				want = release
			}
			if got := LatestBinary(root); got != want {
				t.Fatalf("LatestBinary = %s, want %s", got, want)
			}
		})
	}
}
