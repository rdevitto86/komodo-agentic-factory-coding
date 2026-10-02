package install

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// fakeKomodo logs each call's arguments, so a test sees what the installer handed off to.
const fakeKomodo = "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$HOME/komodo.log\"\n"

// writeFile writes body to path with mode, creating its directory.
func writeFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

// branching is any shell or PowerShell control flow beyond the one checksum test each installer keeps.
var branching = regexp.MustCompile(`(?m)^\s*(if|case|for|while|until|function|elif)\b|\(\)\s*\{`)

func TestTheInstallersOnlyDownloadVerifyAndHandOff(t *testing.T) {
	for _, name := range []string{"install.sh", "install.ps1"} {
		data, err := os.ReadFile(filepath.Join("..", "..", name))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) > 20 {
			t.Errorf("%s has %d lines; an installer only downloads, verifies and runs komodo install", name, len(lines))
		}
		var rest []string
		for _, line := range lines {
			if !strings.Contains(line, "checksum mismatch") {
				rest = append(rest, line)
			}
		}
		if found := branching.FindString(strings.Join(rest, "\n")); found != "" {
			t.Errorf("%s branches at %q; its logic belongs in komodo install", name, strings.TrimSpace(found))
		}
		if !strings.Contains(string(data), "install") || !strings.Contains(strings.ToLower(string(data)), "sha256") {
			t.Errorf("%s must verify the SHA-256 and run komodo install", name)
		}
	}
}

// release writes a fake komodo release for this machine under dir/v9, with sums for its SHA256SUMS.
func release(t *testing.T, dir, sum string) string {
	t.Helper()
	arch := runtime.GOARCH
	name := "komodo-" + runtime.GOOS + "-" + arch
	writeFile(t, filepath.Join(dir, "v9", name), fakeKomodo, 0o644)
	writeFile(t, filepath.Join(dir, "v9", "SHA256SUMS"), sum+"  "+name+"\n", 0o644)
	return name
}

// runInstaller runs install.sh under home with the release at dir, returning its output and exit code.
func runInstaller(t *testing.T, home, dir string) (string, int) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", script)
	cmd.Dir = t.TempDir()
	cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "KOMODO_VERSION=v9", "KOMODO_RELEASE_URL=file://" + dir}
	out, err := cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(out), 0
}

func TestInstallScriptDownloadsVerifiesAndRunsKomodoInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh runs under sh")
	}
	home, dir := t.TempDir(), t.TempDir()
	digest := sha256.Sum256([]byte(fakeKomodo))
	name := release(t, dir, hex.EncodeToString(digest[:]))
	if out, code := runInstaller(t, home, dir); code != 0 {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
	if data, err := os.ReadFile(filepath.Join(home, ".komodo", "bin", name)); err != nil || string(data) != fakeKomodo {
		t.Fatalf("installed %q, %v; want the verified release", data, err)
	}
	if log, err := os.ReadFile(filepath.Join(home, "komodo.log")); err != nil || strings.TrimSpace(string(log)) != "install" {
		t.Fatalf("komodo was called as %q, %v; want one komodo install", log, err)
	}
}

func TestInstallScriptRefusesABadChecksumAndInstallsNothing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh runs under sh")
	}
	home, dir := t.TempDir(), t.TempDir()
	name := release(t, dir, strings.Repeat("0", 64))
	out, code := runInstaller(t, home, dir)
	if code != 1 || !strings.Contains(out, "checksum mismatch") {
		t.Fatalf("a bad checksum gave exit %d, output:\n%s", code, out)
	}
	for _, leftover := range []string{name, name + ".new"} {
		if _, err := os.Stat(filepath.Join(home, ".komodo", "bin", leftover)); !os.IsNotExist(err) {
			t.Fatalf("%s is present (%v); a bad checksum must install nothing", leftover, err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, "komodo.log")); !os.IsNotExist(err) {
		t.Fatal("komodo ran after a bad checksum")
	}
}
