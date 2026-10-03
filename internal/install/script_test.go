package install

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeKomodo logs each call's directory and arguments, so a test sees what the installer ran.
const fakeKomodo = "#!/bin/sh\nprintf '%s|%s\\n' \"$PWD\" \"$*\" >> \"$HOME/komodo.log\"\n"

// fakeGo copies the fake komodo to the path after -o, standing in for a real build.
const fakeGo = "#!/bin/sh\nwhile [ $# -gt 0 ]; do\n  if [ \"$1\" = -o ]; then out=$2; fi\n  shift\ndone\n" +
	"cp \"$FAKE_KOMODO\" \"$out\"\nchmod +x \"$out\"\n"

// scriptFixture is a temp HOME, fake tools, and a copy of install.sh, so nothing touches the real home.
type scriptFixture struct {
	home, bin, checkout, komodo string
}

// newScriptFixture copies install.sh into a temp checkout, a git clone with the Go source marker when source is set.
func newScriptFixture(t *testing.T, source bool) scriptFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	f := scriptFixture{home: t.TempDir(), bin: t.TempDir(), checkout: t.TempDir()}
	f.komodo = filepath.Join(t.TempDir(), "komodo")
	script, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.checkout, "install.sh"), string(script), 0o755)
	if source {
		writeFile(t, filepath.Join(f.checkout, "cmd", "komodo", "main.go"), "package main\n", 0o644)
		gitRepo(t, f.checkout)
	}
	writeFile(t, f.komodo, fakeKomodo, 0o755)
	writeFile(t, filepath.Join(f.bin, "claude"), "#!/bin/sh\n", 0o755)
	writeFile(t, filepath.Join(f.bin, "go"), fakeGo, 0o755)
	return f
}

// run runs install.sh from dir under the fixture's HOME and PATH, with no git repository above dir.
func (f scriptFixture) run(t *testing.T, dir string, env ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("bash", filepath.Join(f.checkout, "install.sh"))
	cmd.Dir = dir
	cmd.Env = append([]string{
		"HOME=" + f.home,
		"PATH=" + f.bin + ":/usr/bin:/bin:/usr/sbin:/sbin",
		"TMPDIR=" + t.TempDir(),
		"FAKE_KOMODO=" + f.komodo,
		"GIT_CEILING_DIRECTORIES=" + filepath.Dir(dir),
	}, env...)
	out, err := cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(out), 0
}

// calls returns each komodo call the fake logged, as directory|arguments.
func (f scriptFixture) calls(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.home, "komodo.log"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// gitRepo makes dir the top of its own empty git repository, written by hand without hooks or config.
func gitRepo(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main\n", 0o644)
	for _, sub := range []string{"objects", filepath.Join("refs", "heads")} {
		if err := os.MkdirAll(filepath.Join(dir, ".git", sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// writeFile writes one fixture file, creating its directory.
func writeFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func TestInstallScriptNamesAMissingPrerequisiteAndHowToGetIt(t *testing.T) {
	f := newScriptFixture(t, true)
	if err := os.Remove(filepath.Join(f.bin, "claude")); err != nil {
		t.Fatal(err)
	}
	out, code := f.run(t, t.TempDir())
	if code != 1 || !strings.Contains(out, "Claude Code is missing") || !strings.Contains(out, "npm install") {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
	if _, err := os.Lstat(filepath.Join(f.home, ".local", "bin", "komodo")); err == nil {
		t.Fatal("a failed prerequisite check still linked komodo")
	}
}

func TestInstallScriptBuildsLinksSetsUpTheRepoAndUpdatesOnASecondRun(t *testing.T) {
	f := newScriptFixture(t, true)
	repo := t.TempDir()
	gitRepo(t, repo)
	realRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{realRepo + "|install --global", realRepo + "|init", realRepo + "|doctor"}
	for run := 1; run <= 2; run++ {
		out, code := f.run(t, repo)
		if code != 0 {
			t.Fatalf("run %d exit %d, output:\n%s", run, code, out)
		}
		target, err := os.Readlink(filepath.Join(f.home, ".local", "bin", "komodo"))
		if err != nil {
			t.Fatalf("run %d did not link komodo: %v", run, err)
		}
		built := filepath.Base(target) == "komodo"
		if filepath.Dir(target) != filepath.Join(f.checkout, "bin") || !built {
			t.Fatalf("run %d linked %s, want the checkout's built binary", run, target)
		}
		got := f.calls(t)
		if len(got) != 3*run || strings.Join(got[3*(run-1):], "\n") != strings.Join(want, "\n") {
			t.Fatalf("run %d called komodo as:\n%s", run, strings.Join(got, "\n"))
		}
	}
}

func TestInstallScriptOutsideARepoInstallsOnlyTheGlobalLayer(t *testing.T) {
	f := newScriptFixture(t, true)
	out, code := f.run(t, t.TempDir())
	if code != 0 {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
	checkout, err := filepath.EvalSymlinks(f.checkout)
	if err != nil {
		t.Fatal(err)
	}
	got := f.calls(t)
	if len(got) != 1 || got[0] != checkout+"|install --global" {
		t.Fatalf("komodo was called as:\n%s", strings.Join(got, "\n"))
	}
}

func TestInstallScriptDownloadsThePinnedReleaseAndRefusesABadChecksum(t *testing.T) {
	f := newScriptFixture(t, false)
	release := t.TempDir()
	digest := sha256.Sum256([]byte(fakeKomodo))
	var sums strings.Builder
	for _, arch := range []string{"amd64", "arm64"} {
		name := "komodo-" + runtime.GOOS + "-" + arch
		writeFile(t, filepath.Join(release, "v9", name), fakeKomodo, 0o644)
		sums.WriteString(hex.EncodeToString(digest[:]) + "  " + name + "\n")
	}
	writeFile(t, filepath.Join(release, "v9", "SHA256SUMS"), sums.String(), 0o644)
	env := []string{"KOMODO_VERSION=v9", "KOMODO_RELEASE_URL=file://" + release}
	repo := t.TempDir()
	gitRepo(t, repo)

	out, code := f.run(t, repo, env...)
	if code != 0 {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
	target, err := os.Readlink(filepath.Join(f.home, ".local", "bin", "komodo"))
	if err != nil || filepath.Dir(target) != filepath.Join(f.home, ".komodo", "bin") {
		t.Fatalf("linked %q (%v), want the downloaded release under HOME", target, err)
	}

	bad := strings.ReplaceAll(sums.String(), hex.EncodeToString(digest[:]), strings.Repeat("0", 64))
	writeFile(t, filepath.Join(release, "v9", "SHA256SUMS"), bad, 0o644)
	out, code = f.run(t, repo, env...)
	if code != 1 || !strings.Contains(out, "checksum mismatch") {
		t.Fatalf("a bad checksum gave exit %d, output:\n%s", code, out)
	}
}
