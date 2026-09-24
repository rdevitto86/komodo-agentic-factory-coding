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
	writeFakeBinary(t, dir, "git", "#!/bin/sh\necho /fake/root/.git\n")
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

func TestHookFromAWorktreeUsesTheMainCheckoutBinary(t *testing.T) {
	main := t.TempDir()
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git(main, "init", "-q", "-b", "main")
	git(main, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "seed")
	worktree := filepath.Join(t.TempDir(), "wt")
	git(main, "worktree", "add", "-q", "-b", "feat/x", worktree)
	fakes := t.TempDir()
	writeFakeBinary(t, fakes, "uname", "#!/bin/sh\ncase \"$1\" in\n-s) echo 'Darwin' ;;\n-m) echo 'arm64' ;;\nesac\n")
	binary := filepath.Join(main, "bin", "komodo-darwin-arm64")
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFakeBinary(t, filepath.Dir(binary), "komodo-darwin-arm64", "#!/bin/sh\necho ran \"$1\"\n")
	script := filepath.Join(fakes, "hook")
	if err := os.WriteFile(script, []byte(hookScript), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", script)
	cmd.Dir = worktree
	cmd.Env = []string{"PATH=" + fakes + ":" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "ran gate") {
		t.Fatalf("err = %v, out = %q; a worktree's hook must run the main checkout's binary", err, out)
	}
}

// TestHookInTheToolkitGatesFromTheCheckoutItCommits proves a checkout holding cmd/komodo runs its
// own source, not the main checkout's binary, and a push still fuzzes.
func TestHookInTheToolkitGatesFromTheCheckoutItCommits(t *testing.T) {
	if !strings.Contains(hookScript, "git rev-parse --show-toplevel") || !strings.Contains(hookScript, "exec go run ./cmd/komodo gate") {
		t.Fatal("the hook script never gates from the committing checkout")
	}
	main := t.TempDir()
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git(main, "init", "-q", "-b", "main")
	git(main, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "seed")
	worktree := filepath.Join(t.TempDir(), "wt")
	git(main, "worktree", "add", "-q", "-b", "feat/x", worktree)
	source := filepath.Join(worktree, "cmd", "komodo", "main.go")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakes := t.TempDir()
	writeFakeBinary(t, fakes, "go", "#!/bin/sh\necho go \"$@\" in \"$(pwd -P)\"\n")
	sub := filepath.Join(worktree, "cmd")
	want, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}
	for hook, args := range map[string]string{"pre-commit": "run ./cmd/komodo gate in", "pre-push": "run ./cmd/komodo gate --fuzz 10s in"} {
		script := filepath.Join(fakes, hook)
		if err := os.WriteFile(script, []byte(hookScript), 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("sh", script)
		cmd.Dir = sub
		cmd.Env = []string{"PATH=" + fakes + ":" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
		out, err := cmd.CombinedOutput()
		if err != nil || strings.TrimSpace(string(out)) != "go "+args+" "+want {
			t.Fatalf("%s: err = %v, out = %q; want go %s %s", hook, err, out, args, want)
		}
	}
}

func TestCommandDropsTheGitEnvironmentAHookSets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("GIT_INDEX_FILE", ".git/index")
	t.Setenv("GIT_DIR", ".git")
	var out bytes.Buffer
	check := Command("env", t.TempDir(), "sh", "-c", "echo ${GIT_INDEX_FILE:-unset} ${GIT_DIR:-unset}")
	if err := check.Run(&out); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "unset unset" {
		t.Fatalf("a hook's git variables reached the child: %q", got)
	}
}

// TestFuzzChecksCoverEveryTarget proves the gate builds one named check per fuzz target.
func TestFuzzChecksCoverEveryTarget(t *testing.T) {
	checks := FuzzChecks(t.TempDir(), "1s")
	if len(checks) != len(FuzzTargets) {
		t.Fatalf("checks = %d, targets = %d", len(checks), len(FuzzTargets))
	}
	for index, check := range checks {
		if check.Name != "fuzz "+FuzzTargets[index].Name {
			t.Fatalf("check %d is named %q", index, check.Name)
		}
	}
}

// TestTestArgsAlwaysRunsEveryPackage proves the race flag never narrows what the gate tests.
func TestTestArgsAlwaysRunsEveryPackage(t *testing.T) {
	args := TestArgs()
	if args[0] != "go" || args[1] != "test" || args[len(args)-1] != "./..." {
		t.Fatalf("args = %q", args)
	}
}

// TestPrePushHookFuzzes proves only the pre-push hook adds the fuzz lane.
func TestPrePushHookFuzzes(t *testing.T) {
	if !strings.Contains(hookScript, `"pre-push" ]; then`) || !strings.Contains(hookScript, "gate --fuzz") {
		t.Fatal("the hook script never fuzzes on push")
	}
}

// TestBuildFlagsStampTheChangelogVersionAndCommit proves a build names what it was built from.
func TestBuildFlagsStampTheChangelogVersionAndCommit(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("# Changelog\n\n## 1.2.3-beta.1 — unreleased\n\n## 1.2.2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flags := strings.Join(buildFlags(root), " ")
	if !strings.Contains(flags, "-X main.version=1.2.3-beta.1") || !strings.Contains(flags, "-X main.commit=unknown") {
		t.Fatalf("flags = %s", flags)
	}
	if !strings.Contains(flags, "-trimpath") || !strings.Contains(flags, "-buildvcs=false") {
		t.Fatalf("flags lost reproducibility: %s", flags)
	}
}

// TestBuildFlagsReadsTheCommitFromGit proves a repo's HEAD names the build, not "unknown".
func TestBuildFlagsReadsTheCommitFromGit(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git("init", "-q")
	git("-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "seed")
	flags := strings.Join(buildFlags(root), " ")
	if strings.Contains(flags, "-X main.commit=unknown") {
		t.Fatalf("flags = %s, want a real commit", flags)
	}
}

// fakeGo writes an executable script named "go" that fakes env, build, and rev-parse for the tests below.
func fakeGo(t *testing.T, dir, body string) {
	t.Helper()
	writeFakeBinary(t, dir, "go", body)
}

// TestCommandPinsTheGoToolchainWhenGoModNamesOne proves a successful pin reaches the child's environment.
func TestCommandPinsTheGoToolchainWhenGoModNamesOne(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	root := t.TempDir()
	mod := "module x\n\ngo 1.22\n\ntoolchain go1.27.1\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	check := Command("env", root, "sh", "-c", "echo ${GOTOOLCHAIN:-unset}")
	if err := check.Run(&out); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "go1.27.1" {
		t.Fatalf("GOTOOLCHAIN = %q", got)
	}
}

// TestBuildWritesTheOutputABuildProduces proves a successful go build lands at the target path.
func TestBuildWritesTheOutputABuildProduces(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.22\n\ntoolchain go1.27.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeDir := t.TempDir()
	fakeGo(t, fakeDir, "#!/bin/sh\nif [ \"$1\" = build ]; then shift 2; echo built > \"$1\"; exit 0; fi\nexit 1\n")
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))
	dir := t.TempDir()
	path, err := Build(root, dir, Target{Name: "komodo-fake", GOOS: "linux", Arch: "amd64"})
	if err != nil {
		t.Fatal(err)
	}
	if body, readErr := os.ReadFile(path); readErr != nil || strings.TrimSpace(string(body)) != "built" {
		t.Fatalf("path = %q, body = %q, err = %v", path, body, readErr)
	}
}

// TestBuildFailsWithoutAPinnedToolchain proves Build refuses to guess a toolchain.
func TestBuildFailsWithoutAPinnedToolchain(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(root, t.TempDir(), LocalTarget()); err == nil {
		t.Fatal("want an error when go.mod names no toolchain")
	}
}

// TestBuildWrapsAFailedGoBuild proves a build failure names the target and carries the compiler's stderr.
func TestBuildWrapsAFailedGoBuild(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.22\n\ntoolchain go1.27.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeDir := t.TempDir()
	fakeGo(t, fakeDir, "#!/bin/sh\necho boom >&2\nexit 1\n")
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))
	_, err := Build(root, t.TempDir(), Target{Name: "komodo-fake", GOOS: "linux", Arch: "amd64"})
	if err == nil || !strings.Contains(err.Error(), "komodo-fake") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}

// TestBuildLocalWritesUnderRootBin proves a local build lands under root/bin and reports its name.
func TestBuildLocalWritesUnderRootBin(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.22\n\ntoolchain go1.27.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeDir := t.TempDir()
	fakeGo(t, fakeDir, "#!/bin/sh\nif [ \"$1\" = build ]; then shift 2; echo built > \"$1\"; exit 0; fi\nexit 1\n")
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))
	var out bytes.Buffer
	path, err := BuildLocal(root, &out)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "bin", LocalTarget().Name); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if !strings.Contains(out.String(), "built "+LocalTarget().Name) {
		t.Fatalf("out = %q", out.String())
	}
}

// TestBuildLocalFailsWhenBinIsAFile proves a blocked bin/ directory surfaces its error.
func TestBuildLocalFailsWhenBinIsAFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bin"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildLocal(root, io.Discard); err == nil {
		t.Fatal("want an error when bin/ cannot be created")
	}
}

// TestInstallFailsWhenHooksIsAFile proves a blocked hooks/ directory surfaces its error.
func TestInstallFailsWhenHooksIsAFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hooks"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(dir); err == nil {
		t.Fatal("want an error when hooks/ cannot be created")
	}
}

// TestInstallFailsWhenAHookPathIsADirectory proves a blocked hook file surfaces its error.
func TestInstallFailsWhenAHookPathIsADirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "hooks", "pre-commit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(dir); err == nil {
		t.Fatal("want an error when a hook path is already a directory")
	}
}

// TestTestArgsDropsTheRaceFlagWithoutCgo proves the plain branch runs when cgo is unavailable.
func TestTestArgsDropsTheRaceFlagWithoutCgo(t *testing.T) {
	fakeDir := t.TempDir()
	fakeGo(t, fakeDir, "#!/bin/sh\necho 0\n")
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))
	args := TestArgs()
	if strings.Join(args, " ") != "go test ./..." {
		t.Fatalf("args = %q", args)
	}
}
