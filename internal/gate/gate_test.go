package gate

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunStopsAtTheFirstFailure(t *testing.T) {
	ran := []string{}
	checks := []Check{
		{Name: "one", Run: func(_ io.Writer) error { ran = append(ran, "one"); return nil }},
		{Name: "two", Run: func(_ io.Writer) error { ran = append(ran, "two"); return errors.New("boom") }},
		{Name: "three", Run: func(_ io.Writer) error { ran = append(ran, "three"); return nil }},
	}
	var out bytes.Buffer
	err := Run(checks, &out)
	if err == nil || !strings.Contains(err.Error(), "two: boom") {
		t.Fatalf("err = %v", err)
	}
	if strings.Join(ran, ",") != "one,two" {
		t.Fatalf("ran = %v", ran)
	}
}

func TestRunReportsEveryCheckPassing(t *testing.T) {
	var out bytes.Buffer
	checks := []Check{{Name: "only", Run: func(_ io.Writer) error { return nil }}}
	if err := Run(checks, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1 check(s) passed") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestSumIsStable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, []byte("komodo"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := Sum(path)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := Sum(path)
	if first != second || len(first) != 64 {
		t.Fatalf("sum = %q %q", first, second)
	}
}

func TestInstallWritesBothHooks(t *testing.T) {
	dir := t.TempDir()
	written, err := Install(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 2 {
		t.Fatalf("written = %v", written)
	}
	for _, path := range written {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("%s is not executable", path)
		}
		body, _ := os.ReadFile(path)
		if !strings.Contains(string(body), "gate") || !strings.Contains(string(body), "uname") {
			t.Fatalf("%s does not run the gate by platform", path)
		}
	}
}

func TestToolchainReadsThePinnedVersion(t *testing.T) {
	dir := t.TempDir()
	mod := "module x\n\ngo 1.22\n\ntoolchain go1.27.1\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Toolchain(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "go1.27.1" {
		t.Fatalf("toolchain = %q", got)
	}
}

func TestToolchainFailsWithoutAPin(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Toolchain(dir); err == nil {
		t.Fatal("want an error when go.mod names no toolchain")
	}
}

func TestLocalTargetMatchesRuntime(t *testing.T) {
	target := LocalTarget()
	if target.GOOS != runtime.GOOS || target.Arch != runtime.GOARCH {
		t.Fatalf("target = %+v", target)
	}
	want := fmt.Sprintf("komodo-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if target.Name != want {
		t.Fatalf("name = %q, want %q", target.Name, want)
	}
}

// writeFakeBinary drops an executable shell script named name on a fake PATH entry.
func writeFakeBinary(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// runHook runs the hook script under sh, with fakeDir first on PATH.
func runHook(fakeDir, script string) (string, error) {
	cmd := exec.Command("sh", script)
	cmd.Env = []string{"PATH=" + fakeDir + ":" + os.Getenv("PATH")}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return out.String(), err
}

// TestHookScriptPicksBinaryPerPlatform proves the case statement never falls back to the Windows exe.
func TestHookScriptPicksBinaryPerPlatform(t *testing.T) {
	dir := t.TempDir()
	writeFakeBinary(t, dir, "git", "#!/bin/sh\necho /fake/root\n")
	script := filepath.Join(dir, "hook")
	if err := os.WriteFile(script, []byte(hookScript), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ unameS, unameM, wantBin string }{
		{"Darwin", "arm64", "komodo-darwin-arm64"},
		{"Darwin", "x86_64", "komodo-darwin-amd64"},
		{"Linux", "x86_64", "komodo-linux-amd64"},
		{"Linux", "aarch64", "komodo-linux-arm64"},
		{"MINGW64_NT-10.0", "x86_64", "komodo-windows-amd64.exe"},
	}
	for _, c := range cases {
		writeFakeBinary(t, dir, "uname", fmt.Sprintf(
			"#!/bin/sh\ncase \"$1\" in\n-s) echo '%s' ;;\n-m) echo '%s' ;;\nesac\n", c.unameS, c.unameM))
		out, err := runHook(dir, script)
		if err == nil {
			t.Fatalf("%s-%s: want failure, no binary is built in the fake root", c.unameS, c.unameM)
		}
		if want := "no binary at /fake/root/bin/" + c.wantBin; !strings.Contains(out, want) {
			t.Errorf("%s-%s: out = %q, want contains %q", c.unameS, c.unameM, out, want)
		}
	}
	writeFakeBinary(t, dir, "uname", "#!/bin/sh\ncase \"$1\" in\n-s) echo 'FreeBSD' ;;\n-m) echo 'amd64' ;;\nesac\n")
	out, err := runHook(dir, script)
	if err == nil {
		t.Fatal("want failure for an unrecognised platform")
	}
	if !strings.Contains(out, "no binary for this platform") || strings.Contains(out, "windows") {
		t.Fatalf("an unknown platform must not fall back to the Windows binary: out = %q", out)
	}
}
