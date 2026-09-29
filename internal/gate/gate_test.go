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

func TestInstallWritesEveryHook(t *testing.T) {
	dir := t.TempDir()
	written, err := Install(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 5 {
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
	if !strings.Contains(hookScript, "git rev-parse --show-toplevel") || !strings.Contains(hookScript, `cmd="go run ./cmd/komodo"`) {
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
	if !strings.Contains(hookScript, "pre-push)") || !strings.Contains(hookScript, "gate --fuzz") {
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

// gitCommand runs one git command in dir and fails the test on error, returning trimmed stdout.
func gitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestRebuildStampsTheNewHeadWhenAGoFileChanged proves REQ-5: after a pull that changes a Go file,
// bin/.built-from equals the new HEAD.
func TestRebuildStampsTheNewHeadWhenAGoFileChanged(t *testing.T) {
	root := t.TempDir()
	gitCommand(t, root, "init", "-q")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "seed")
	from := gitCommand(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "main.go")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "add a go file")
	to := gitCommand(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.22\n\ntoolchain go1.27.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeDir := t.TempDir()
	fakeGo(t, fakeDir, "#!/bin/sh\nif [ \"$1\" = build ]; then shift 2; echo built > \"$1\"; exit 0; fi\nexit 1\n")
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))

	if err := Rebuild(root, from, to, io.Discard); err != nil {
		t.Fatal(err)
	}
	marker, err := os.ReadFile(filepath.Join(root, "bin", BuiltFrom))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(marker)) != to {
		t.Fatalf("marker = %q, want %q", marker, to)
	}
}

// TestRebuildResolvesHEADToItsFullCommitSHA proves --to HEAD is never stamped literally, so doctor
// never reads bin/.built-from as a name git cannot compare to a real commit.
func TestRebuildResolvesHEADToItsFullCommitSHA(t *testing.T) {
	root := t.TempDir()
	gitCommand(t, root, "init", "-q")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "seed")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "main.go")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "add a go file")
	want := gitCommand(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.22\n\ntoolchain go1.27.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeDir := t.TempDir()
	fakeGo(t, fakeDir, "#!/bin/sh\nif [ \"$1\" = build ]; then shift 2; echo built > \"$1\"; exit 0; fi\nexit 1\n")
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))

	if err := Rebuild(root, "HEAD~1", "HEAD", io.Discard); err != nil {
		t.Fatal(err)
	}
	marker, err := os.ReadFile(filepath.Join(root, "bin", BuiltFrom))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(marker)); got != want || got == "HEAD" {
		t.Fatalf("marker = %q, want the resolved commit %q", got, want)
	}
}

// TestRebuildInstallsHooksBeforeStamping proves a pull that changes a Go file rewrites the git hooks,
// not just the marker, so a hook script change is never hidden behind a stale-looking stamp.
func TestRebuildInstallsHooksBeforeStamping(t *testing.T) {
	root := t.TempDir()
	gitCommand(t, root, "init", "-q")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "seed")
	from := gitCommand(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "main.go")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "add a go file")
	to := gitCommand(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.22\n\ntoolchain go1.27.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeDir := t.TempDir()
	fakeGo(t, fakeDir, "#!/bin/sh\nif [ \"$1\" = build ]; then shift 2; echo built > \"$1\"; exit 0; fi\nexit 1\n")
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))

	if err := Rebuild(root, from, to, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "hooks", "pre-commit")); err != nil {
		t.Fatalf("want the pre-commit hook installed, err = %v", err)
	}
}

// TestRebuildDoesNotStampADirtyTree proves an uncommitted tracked edit is built but never stamped,
// so a later clean build from the same commit is not skipped as already current.
func TestRebuildDoesNotStampADirtyTree(t *testing.T) {
	root := t.TempDir()
	gitCommand(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "main.go")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "seed")
	from := gitCommand(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "main.go")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "edit")
	to := gitCommand(t, root, "rev-parse", "HEAD")
	// A tracked edit left uncommitted after the commit that produced to.
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() { println() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.22\n\ntoolchain go1.27.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeDir := t.TempDir()
	fakeGo(t, fakeDir, "#!/bin/sh\nif [ \"$1\" = build ]; then shift 2; echo built > \"$1\"; exit 0; fi\nexit 1\n")
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))

	if err := Rebuild(root, from, to, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", BuiltFrom)); !os.IsNotExist(err) {
		t.Fatalf("want no marker written for a dirty tree, err = %v", err)
	}
}

// TestRebuildDoesNothingWithoutABuildInputChange proves an unrelated file never triggers a build.
func TestRebuildDoesNothingWithoutABuildInputChange(t *testing.T) {
	root := t.TempDir()
	gitCommand(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "README.md")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "seed")
	from := gitCommand(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "README.md")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "edit")
	to := gitCommand(t, root, "rev-parse", "HEAD")

	if err := Rebuild(root, from, to, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", BuiltFrom)); !os.IsNotExist(err) {
		t.Fatalf("want no marker written, err = %v", err)
	}
}

// TestRebuildSkipsAZeroFromCommit proves the null object id a fresh checkout passes never rebuilds.
func TestRebuildSkipsAZeroFromCommit(t *testing.T) {
	root := t.TempDir()
	gitCommand(t, root, "init", "-q")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "seed")
	to := gitCommand(t, root, "rev-parse", "HEAD")
	if err := Rebuild(root, zeroOID, to, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", BuiltFrom)); !os.IsNotExist(err) {
		t.Fatalf("want no marker written, err = %v", err)
	}
}

// TestHookScriptRebuildsOnPostMergeAndPostRewrite proves each hook passes its old and new commit to --rebuild.
func TestHookScriptRebuildsOnPostMergeAndPostRewrite(t *testing.T) {
	main := t.TempDir()
	gitCommand(t, main, "init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(main, "cmd", "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(main, "cmd", "komodo", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, main, "add", "cmd/komodo/main.go")
	gitCommand(t, main, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "seed")
	head := gitCommand(t, main, "rev-parse", "HEAD")

	fakes := t.TempDir()
	fakeGo(t, fakes, "#!/bin/sh\necho ran \"$@\"\n")

	script := filepath.Join(fakes, "post-merge")
	if err := os.WriteFile(script, []byte(hookScript), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", script)
	cmd.Dir = main
	cmd.Env = []string{"PATH=" + fakes + ":" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("post-merge: %v: %s", err, out)
	}
	if want := "ran run ./cmd/komodo gate --rebuild --from  --to " + head; strings.TrimSpace(string(out)) != want {
		t.Fatalf("post-merge: out = %q, want %q", out, want)
	}

	script = filepath.Join(fakes, "post-rewrite")
	if err := os.WriteFile(script, []byte(hookScript), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("sh", script, "amend")
	cmd.Dir = main
	cmd.Env = []string{"PATH=" + fakes + ":" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
	cmd.Stdin = strings.NewReader("old1 new1 extra\nold2 new2\n")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("post-rewrite: %v: %s", err, out)
	}
	if want := "ran run ./cmd/komodo gate --rebuild --from old1 --to new2"; strings.TrimSpace(string(out)) != want {
		t.Fatalf("post-rewrite: out = %q, want %q", out, want)
	}
}

// TestPostCheckoutOnlyRebuildsInTheMainWorkingTree proves a worktree the line cut never rebuilds,
// and a branch checkout in the main working tree does.
func TestPostCheckoutOnlyRebuildsInTheMainWorkingTree(t *testing.T) {
	main := t.TempDir()
	gitCommand(t, main, "init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(main, "cmd", "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(main, "cmd", "komodo", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, main, "add", "cmd/komodo/main.go")
	gitCommand(t, main, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "seed")
	worktree := filepath.Join(t.TempDir(), "wt")
	gitCommand(t, main, "worktree", "add", "-q", "-b", "feat/x", worktree)

	fakes := t.TempDir()
	fakeGo(t, fakes, "#!/bin/sh\necho ran \"$@\"\n")
	script := filepath.Join(fakes, "post-checkout")
	if err := os.WriteFile(script, []byte(hookScript), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(dir string) string {
		cmd := exec.Command("sh", script, "old", "new", "1")
		cmd.Dir = dir
		cmd.Env = []string{"PATH=" + fakes + ":" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("post-checkout in %s: %v: %s", dir, err, out)
		}
		return string(out)
	}
	if out := run(worktree); strings.Contains(out, "ran") {
		t.Fatalf("a worktree the line cut must not rebuild: out = %q", out)
	}
	if out := run(main); !strings.Contains(out, "ran run ./cmd/komodo gate --rebuild --from old --to new") {
		t.Fatalf("the main working tree must rebuild: out = %q", out)
	}
}

// TestPostMergeOnlyRebuildsInTheMainWorkingTree proves a merge inside a worktree the line cut never
// rebuilds, so a Go-touching group merge cannot rewrite the shared hooks from unreviewed source.
func TestPostMergeOnlyRebuildsInTheMainWorkingTree(t *testing.T) {
	main := t.TempDir()
	gitCommand(t, main, "init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(main, "cmd", "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(main, "cmd", "komodo", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, main, "add", "cmd/komodo/main.go")
	gitCommand(t, main, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "seed")
	worktree := filepath.Join(t.TempDir(), "wt")
	gitCommand(t, main, "worktree", "add", "-q", "-b", "feat/x", worktree)

	fakes := t.TempDir()
	fakeGo(t, fakes, "#!/bin/sh\necho ran \"$@\"\n")
	script := filepath.Join(fakes, "post-merge")
	if err := os.WriteFile(script, []byte(hookScript), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(dir string) string {
		cmd := exec.Command("sh", script)
		cmd.Dir = dir
		cmd.Env = []string{"PATH=" + fakes + ":" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("post-merge in %s: %v: %s", dir, err, out)
		}
		return string(out)
	}
	if out := run(worktree); strings.Contains(out, "ran") {
		t.Fatalf("a worktree the line cut must not rebuild: out = %q", out)
	}
	if out := run(main); !strings.Contains(out, "ran run ./cmd/komodo gate --rebuild") {
		t.Fatalf("the main working tree must rebuild: out = %q", out)
	}
}

// TestPostRewriteOnlyRebuildsInTheMainWorkingTree proves a rebase inside a worktree the line cut
// never rebuilds, so ship's catch-up rebase cannot rewrite the shared hooks from unreviewed source.
func TestPostRewriteOnlyRebuildsInTheMainWorkingTree(t *testing.T) {
	main := t.TempDir()
	gitCommand(t, main, "init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(main, "cmd", "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(main, "cmd", "komodo", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, main, "add", "cmd/komodo/main.go")
	gitCommand(t, main, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "seed")
	worktree := filepath.Join(t.TempDir(), "wt")
	gitCommand(t, main, "worktree", "add", "-q", "-b", "feat/x", worktree)

	fakes := t.TempDir()
	fakeGo(t, fakes, "#!/bin/sh\necho ran \"$@\"\n")
	script := filepath.Join(fakes, "post-rewrite")
	if err := os.WriteFile(script, []byte(hookScript), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(dir string) string {
		cmd := exec.Command("sh", script, "rebase")
		cmd.Dir = dir
		cmd.Env = []string{"PATH=" + fakes + ":" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
		cmd.Stdin = strings.NewReader("old1 new1 extra\nold2 new2\n")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("post-rewrite in %s: %v: %s", dir, err, out)
		}
		return string(out)
	}
	if out := run(worktree); strings.Contains(out, "ran") {
		t.Fatalf("a worktree the line cut must not rebuild: out = %q", out)
	}
	if out := run(main); !strings.Contains(out, "ran run ./cmd/komodo gate --rebuild --from old1 --to new2") {
		t.Fatalf("the main working tree must rebuild: out = %q", out)
	}
}

func TestDiffAddedLinesReadsOnlyAddedAndChangedLines(t *testing.T) {
	diff := "diff --git a/a.go b/a.go\n" +
		"--- a/a.go\n" +
		"+++ b/a.go\n" +
		"@@ -1,2 +1,3 @@\n" +
		" package a\n" +
		"-func Old() {}\n" +
		"+func Old() { return }\n" +
		"+func New() {}\n"

	added := diffAddedLines(diff)
	if got := added["a.go"]; len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("added = %v, want [2 3]", got)
	}
}

func TestDiffAddedLinesSkipsADeletedFile(t *testing.T) {
	diff := "diff --git a/a.go b/a.go\n" +
		"--- a/a.go\n" +
		"+++ /dev/null\n" +
		"@@ -1,1 +0,0 @@\n" +
		"-func Old() {}\n"

	if added := diffAddedLines(diff); len(added) != 0 {
		t.Fatalf("added = %v, want none for a deleted file", added)
	}
}

func TestStagedDiffLinesReadsTheIndex(t *testing.T) {
	root := t.TempDir()
	gitCommand(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "a.go")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "seed")

	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n\nfunc New() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "a.go")

	added, err := StagedDiffLines(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := added["a.go"]; len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("added = %v, want [2 3]", got)
	}
}

func TestCommentsCheckOnlyFlagsAStagedLine(t *testing.T) {
	root := t.TempDir()
	gitCommand(t, root, "init", "-q")
	body := "package a\n\nfunc Old() int {\n\tx := 1\n\treturn x\n}\n"
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "a.go")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "seed, undocumented on purpose")

	added := "package a\n\nfunc Old() int {\n\tx := 1\n\treturn x\n}\n\nfunc New() int {\n\ty := 1\n\treturn y\n}\n"
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte(added), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "a.go")

	var out strings.Builder
	check := CommentsCheck(root, "nonobvious")
	err := check.Run(&out)
	if err == nil {
		t.Fatal("want the new undocumented function to fail the check")
	}
	if !strings.Contains(out.String(), "New") {
		t.Fatalf("out = %q, want it to name New", out.String())
	}
	if strings.Contains(out.String(), "Old") {
		t.Fatalf("out = %q, want the pre-existing Old left alone", out.String())
	}
}

// pushRepo builds a root with a seed commit, then a second commit changing rel, returning both commits.
func pushRepo(t *testing.T, rel, body string) (root, from, to string) {
	t.Helper()
	root = t.TempDir()
	gitCommand(t, root, "init", "-q")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "seed")
	from = gitCommand(t, root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", rel)
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "change "+rel)
	to = gitCommand(t, root, "rev-parse", "HEAD")
	return root, from, to
}

func TestPushedFilesListsWhatChangedBetweenTwoCommits(t *testing.T) {
	root, from, to := pushRepo(t, "docs/notes.md", "notes\n")
	paths, err := PushedFiles(root, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "docs/notes.md" {
		t.Fatalf("paths = %v, want [docs/notes.md]", paths)
	}
}

func TestPushedFilesDiffsAgainstTheEmptyTreeForANewBranch(t *testing.T) {
	root := t.TempDir()
	gitCommand(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "a.go")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "seed")
	to := gitCommand(t, root, "rev-parse", "HEAD")

	paths, err := PushedFiles(root, zeroOID, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "a.go" {
		t.Fatalf("paths = %v, want [a.go]", paths)
	}
}

func TestPushedFuzzTargetsNamesOnlyATouchedPackage(t *testing.T) {
	touched := PushedFuzzTargets([]string{"internal/backlog/parse.go", "docs/notes.md"})
	if len(touched) != 1 || touched[0].Name != "FuzzParse" {
		t.Fatalf("touched = %v, want only FuzzParse", touched)
	}
}

func TestPushedFuzzTargetsIsEmptyWithoutATouchedPackage(t *testing.T) {
	if touched := PushedFuzzTargets([]string{"docs/notes.md"}); len(touched) != 0 {
		t.Fatalf("touched = %v, want none", touched)
	}
}

func TestPushChecksDropsBuildForAMarkdownOnlyPush(t *testing.T) {
	root, from, to := pushRepo(t, "docs/notes.md", "notes\n")
	build := []Check{{Name: "go test"}}
	checks, err := PushChecks(root, from, to, "", build)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 0 {
		t.Fatalf("checks = %v, want none for a markdown-only push", checks)
	}
}

func TestPushChecksKeepsBuildWhenAGoFileChanged(t *testing.T) {
	root, from, to := pushRepo(t, "a.go", "package a\n")
	build := []Check{{Name: "go test"}}
	checks, err := PushChecks(root, from, to, "", build)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 || checks[0].Name != "go test" {
		t.Fatalf("checks = %v, want [go test]", checks)
	}
}

func TestPushChecksOnlyFuzzesATouchedPackage(t *testing.T) {
	root, from, to := pushRepo(t, "internal/backlog/parse.go", "package backlog\n")
	checks, err := PushChecks(root, from, to, "1s", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 || checks[0].Name != "fuzz FuzzParse" {
		t.Fatalf("checks = %v, want only fuzz FuzzParse", checks)
	}
}

func TestPushChecksFuzzesNothingWithoutATouchedFuzzPackage(t *testing.T) {
	root, from, to := pushRepo(t, "docs/notes.md", "notes\n")
	checks, err := PushChecks(root, from, to, "1s", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 0 {
		t.Fatalf("checks = %v, want none", checks)
	}
}

func TestPushChecksRunsEverythingWithoutATo(t *testing.T) {
	root, from, _ := pushRepo(t, "docs/notes.md", "notes\n")
	build := []Check{{Name: "go test"}}
	checks, err := PushChecks(root, from, "", "1s", build)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1+len(FuzzTargets) || checks[0].Name != "go test" {
		t.Fatalf("checks = %v, want go test plus every fuzz target", checks)
	}
}

// toolkitCheckoutFor builds a git repo whose cmd/komodo/main.go marks it as the toolkit's own checkout,
// so the hook script gates it with "go run ./cmd/komodo" instead of a built binary.
func toolkitCheckoutFor(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitCommand(t, root, "init", "-q", "-b", "main")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "seed")
	if err := os.MkdirAll(filepath.Join(root, "cmd", "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "komodo", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestPrePushHookScopesTheGateToTheRefsGitPasses proves the hook reads git's pre-push stdin protocol
// and passes the pushed range, so the gate can scope its build and fuzz checks to it.
func TestPrePushHookScopesTheGateToTheRefsGitPasses(t *testing.T) {
	root := toolkitCheckoutFor(t)
	fakes := t.TempDir()
	fakeGo(t, fakes, "#!/bin/sh\necho go \"$@\"\n")
	script := filepath.Join(fakes, "pre-push")
	if err := os.WriteFile(script, []byte(hookScript), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", script)
	cmd.Dir = root
	cmd.Env = []string{"PATH=" + fakes + ":" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
	cmd.Stdin = strings.NewReader("refs/heads/main abc123 refs/heads/main def456\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pre-push: %v: %s", err, out)
	}
	if !strings.Contains(string(out), "gate --fuzz 10s --from def456 --to abc123") {
		t.Fatalf("out = %q, want the pushed range passed through", out)
	}
}

func TestPrePushHookFallsBackWithoutStdin(t *testing.T) {
	root := toolkitCheckoutFor(t)
	fakes := t.TempDir()
	fakeGo(t, fakes, "#!/bin/sh\necho go \"$@\"\n")
	script := filepath.Join(fakes, "pre-push")
	if err := os.WriteFile(script, []byte(hookScript), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", script)
	cmd.Dir = root
	cmd.Env = []string{"PATH=" + fakes + ":" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pre-push: %v: %s", err, out)
	}
	if !strings.Contains(string(out), "gate --fuzz 10s") || strings.Contains(string(out), "--from") {
		t.Fatalf("out = %q, want the plain fuzz lane without a range", out)
	}
}
