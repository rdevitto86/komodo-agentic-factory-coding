package gate

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"komodo/internal/backlog"
	"komodo/internal/comments"
	"komodo/internal/guard"
	"komodo/internal/lease"
	"komodo/internal/mount"
	"komodo/internal/proc"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
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
	if len(written) != 7 {
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
		if !strings.Contains(string(body), "git-hook") || !strings.Contains(string(body), ".komodo/bin/komodo") {
			t.Fatalf("%s does not exec the installed binary's git-hook", path)
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
	want := "komodo"
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if platform := fmt.Sprintf("komodo-%s-%s", runtime.GOOS, runtime.GOARCH); !strings.HasPrefix(PlatformName(), platform) {
		t.Fatalf("platform name = %s, want %s", PlatformName(), platform)
	}
	if target.Name != want {
		t.Fatalf("name = %q, want %q", target.Name, want)
	}
}

// pathWithoutKomodo is this PATH minus any directory holding a komodo, so a hook never finds the host's line.
func pathWithoutKomodo() string {
	var keep []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if _, err := os.Stat(filepath.Join(dir, "komodo")); err != nil {
			keep = append(keep, dir)
		}
	}
	return strings.Join(keep, string(os.PathListSeparator))
}

// writeFakeBinary drops an executable shell script named name on a fake PATH entry.
func writeFakeBinary(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
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
	targets := FuzzTargets()
	checks := FuzzChecksFor(t.TempDir(), "1s", targets)
	if len(checks) != len(targets) {
		t.Fatalf("checks = %d, targets = %d", len(checks), len(targets))
	}
	for index, check := range checks {
		if check.Name != "fuzz "+targets[index].Name {
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

// TestCommandKillsAHungCommandAndItsChildren proves a command past CommandTimeout is killed,
// process group included, so a hung check never blocks the gate.
func TestCommandKillsAHungCommandAndItsChildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	saved := CommandTimeout
	CommandTimeout = 200 * time.Millisecond
	t.Cleanup(func() { CommandTimeout = saved })
	var out bytes.Buffer
	check := Command("hang", t.TempDir(), "sh", "-c", "sleep 30 & sleep 30")
	started := time.Now()
	if err := check.Run(&out); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want it to name the timeout", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("the kill took %s; the group was not killed", time.Since(started))
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

// TestBuildKillsAHungCompilerAndItsChildren proves a build past CommandTimeout is killed,
// process group included, so a hung compiler never blocks the gate.
func TestBuildKillsAHungCompilerAndItsChildren(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.22\n\ntoolchain go1.27.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeDir := t.TempDir()
	fakeGo(t, fakeDir, "#!/bin/sh\nif [ \"$1\" = build ]; then sleep 30 & sleep 30; fi\nexit 1\n")
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))
	saved := CommandTimeout
	CommandTimeout = 200 * time.Millisecond
	t.Cleanup(func() { CommandTimeout = saved })
	started := time.Now()
	if _, err := Build(root, t.TempDir(), Target{Name: "komodo-fake", GOOS: "linux", Arch: "amd64"}); err == nil ||
		!strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want it to name the timeout", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("the kill took %s; the group was not killed", time.Since(started))
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

// TestBuildLocalIfGoRepoSkipsWithoutGoMod proves install never fails building a non-Go repo's binary.
func TestBuildLocalIfGoRepoSkipsWithoutGoMod(t *testing.T) {
	root := t.TempDir()
	path, err := BuildLocalIfGoRepo(root, io.Discard)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if path != "" {
		t.Fatalf("path = %q, want empty", path)
	}
}

// TestBuildLocalIfGoRepoBuildsWithAGoMod proves install still builds the binary in the toolkit's own checkout.
func TestBuildLocalIfGoRepoBuildsWithAGoMod(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module komodo\n\ngo 1.22\n\ntoolchain go1.27.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cmd", "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "komodo", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeDir := t.TempDir()
	fakeGo(t, fakeDir, "#!/bin/sh\nif [ \"$1\" = build ]; then shift 2; echo built > \"$1\"; exit 0; fi\nexit 1\n")
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))
	path, err := BuildLocalIfGoRepo(root, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "bin", LocalTarget().Name); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

// TestBuildLocalIfGoRepoSkipsATargetRepoWithItsOwnCmdKomodo proves install never builds a target repo's own CLI as the line.
func TestBuildLocalIfGoRepoSkipsATargetRepoWithItsOwnCmdKomodo(t *testing.T) {
	root := checkoutWithModule(t, "github.com/example/runner")
	path, err := BuildLocalIfGoRepo(root, io.Discard)
	if err != nil || path != "" {
		t.Fatalf("path = %q, err = %v; want no build", path, err)
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

// TestRebuildSkipsWhileSyncEnvIsSet proves a merge komodo sync drives rebuilds the binary once,
// through syncBinary, never again through the post-merge hook it triggers.
func TestRebuildSkipsWhileSyncEnvIsSet(t *testing.T) {
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
	t.Setenv(SyncEnv, "1")

	if err := Rebuild(root, from, to, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", BuiltFrom)); !os.IsNotExist(err) {
		t.Fatalf("want no marker written while sync drives the merge, err = %v", err)
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
	if err := comments.RecordSweep(root); err != nil {
		t.Fatal(err)
	}

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

// seedOldUndocumented commits a.go holding one undocumented exported function, Old.
func seedOldUndocumented(t *testing.T, root string) {
	t.Helper()
	gitCommand(t, root, "init", "-q")
	body := "package a\n\nfunc Old() int {\n\tx := 1\n\treturn x\n}\n"
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "a.go")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "seed, Old left undocumented")
}

// stageDocumentedNew stages b.go holding one documented exported function, New.
func stageDocumentedNew(t *testing.T, root string) {
	t.Helper()
	body := "package a\n\n// New says hi.\nfunc New() int {\n\treturn 1\n}\n"
	if err := os.WriteFile(filepath.Join(root, "b.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "b.go")
}

// TestCommentsCheckSweepsAFreshRepo proves a repo with no recorded sweep gets a full one.
func TestCommentsCheckSweepsAFreshRepo(t *testing.T) {
	root := t.TempDir()
	seedOldUndocumented(t, root)
	stageDocumentedNew(t, root)

	var out strings.Builder
	if err := CommentsCheck(root, "nonobvious").Run(&out); err == nil {
		t.Fatal("want a fresh repo's full sweep to catch the pre-existing Old")
	}
	if !strings.Contains(out.String(), "Old") {
		t.Fatalf("out = %q, want it to name the pre-existing Old", out.String())
	}
}

// TestCommentsCheckSecondRunWithTheSameFingerprintIsDiffOnly proves a recorded sweep narrows later runs.
func TestCommentsCheckSecondRunWithTheSameFingerprintIsDiffOnly(t *testing.T) {
	root := t.TempDir()
	gitCommand(t, root, "init", "-q")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "seed")

	var first strings.Builder
	if err := CommentsCheck(root, "nonobvious").Run(&first); err != nil {
		t.Fatalf("want the first sweep on an empty repo to pass: %s", first.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".komodo", "comments-sweep.json")); err != nil {
		t.Fatalf("a passing sweep did not record its fingerprint: %v", err)
	}

	old := filepath.Join(root, "old.go")
	if err := os.WriteFile(old, []byte("package a\n\nfunc Old() int {\n\tx := 1\n\treturn x\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "old.go")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "seed Old, left undocumented")
	stageDocumentedNew(t, root)

	var second strings.Builder
	if err := CommentsCheck(root, "nonobvious").Run(&second); err != nil {
		t.Fatalf("want a diff-only run to leave the untouched Old alone: %s", second.String())
	}
}

// TestCommentsCheckSweepsAgainWhenTheFingerprintChanges proves a stale recorded fingerprint triggers a sweep.
func TestCommentsCheckSweepsAgainWhenTheFingerprintChanges(t *testing.T) {
	root := t.TempDir()
	seedOldUndocumented(t, root)
	if err := comments.RecordSweep(root); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(root, ".komodo", "comments-sweep.json")
	if err := os.WriteFile(stale, []byte(`{"fingerprint":"stale"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stageDocumentedNew(t, root)

	var out strings.Builder
	if err := CommentsCheck(root, "nonobvious").Run(&out); err == nil {
		t.Fatal("want a stale fingerprint to trigger a full sweep that catches the pre-existing Old")
	}
	if !strings.Contains(out.String(), "Old") {
		t.Fatalf("out = %q, want it to name the pre-existing Old", out.String())
	}
}

// TestCommentsCheckAFailingSweepDoesNotRecord proves a sweep that finds problems leaves no state behind.
func TestCommentsCheckAFailingSweepDoesNotRecord(t *testing.T) {
	root := t.TempDir()
	seedOldUndocumented(t, root)

	var out strings.Builder
	if err := CommentsCheck(root, "nonobvious").Run(&out); err == nil {
		t.Fatal("want the first sweep to fail on the undocumented Old")
	}
	if _, err := os.Stat(filepath.Join(root, ".komodo", "comments-sweep.json")); err == nil {
		t.Fatal("a failing sweep recorded its fingerprint")
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
	if len(checks) != 1+len(FuzzTargets()) || checks[0].Name != "go test" {
		t.Fatalf("checks = %v, want go test plus every fuzz target", checks)
	}
}

// toolkitCheckoutFor builds a git repo whose cmd/komodo/main.go and komodo module mark it as the toolkit's own checkout,
// so the hook script gates it with "go run ./cmd/komodo" instead of a built binary.
func toolkitCheckoutFor(t *testing.T) string {
	t.Helper()
	return checkoutWithModule(t, "komodo")
}

// checkoutWithModule builds a git repo holding cmd/komodo/main.go under the named go.mod module.
func checkoutWithModule(t *testing.T, module string) string {
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
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+module+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// gateBinaryOnce builds this checkout's own komodo binary once, shared by every hook-installing test.
var (
	gateBinaryOnce sync.Once
	gateBinaryPath string
	gateBinaryErr  error
)

// gateBinary returns the path to a real, freshly built komodo binary for this module.
func gateBinary(t *testing.T) string {
	t.Helper()
	gateBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "komodo-gate-test-bin")
		if err != nil {
			gateBinaryErr = err
			return
		}
		path := filepath.Join(dir, LocalTarget().Name)
		cmd := exec.Command("go", "build", "-o", path, "komodo/cmd/komodo")
		if out, buildErr := cmd.CombinedOutput(); buildErr != nil {
			gateBinaryErr = fmt.Errorf("build komodo: %v: %s", buildErr, out)
			return
		}
		gateBinaryPath = path
	})
	if gateBinaryErr != nil {
		t.Fatal(gateBinaryErr)
	}
	return gateBinaryPath
}

// installedRepo inits a repo on branch, installs the real hooks, and gives it the real binary and a
// trivial build check, so a commit that clears --check-branch and --commit-msg clears the rest too.
func installedRepo(t *testing.T, branch string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", "-q", "-b", branch)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	// Each repo gets its own home, so the binary its hooks exec is this test's build and no other's.
	t.Setenv("HOME", t.TempDir())
	if _, err := Install(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	built, err := os.ReadFile(gateBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	hook, err := mount.HookPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook, built, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".komodo", "commands.json"), []byte(`{"compile": "true"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("* text=auto eol=lf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// commitIn runs git commit in root with a throwaway identity, returning its combined output.
func commitIn(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	full := append([]string{"-c", "user.email=a@example.com", "-c", "user.name=a", "commit"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestPreCommitHookRefusesACriticalRef proves a commit on main is refused before the gate itself runs.
func TestPreCommitHookRefusesACriticalRef(t *testing.T) {
	root := installedRepo(t, "main")
	out, err := commitIn(t, root, "--allow-empty", "-m", "feat: x")
	if err == nil {
		t.Fatal("want a refusal, got none")
	}
	if !strings.Contains(out, "create a branch first") {
		t.Fatalf("out = %q", out)
	}
}

// TestPreCommitHookRefusesANonConventionalBranch proves a branch such as wip is refused.
func TestPreCommitHookRefusesANonConventionalBranch(t *testing.T) {
	root := installedRepo(t, "wip")
	out, err := commitIn(t, root, "--allow-empty", "-m", "feat: x")
	if err == nil {
		t.Fatal("want a refusal, got none")
	}
	if !strings.Contains(out, "<type>/<kebab-name>") {
		t.Fatalf("out = %q", out)
	}
}

// TestPreCommitHookAllowsAConventionalBranch proves a branch such as fix/x commits cleanly.
func TestPreCommitHookAllowsAConventionalBranch(t *testing.T) {
	root := installedRepo(t, "fix/x")
	out, err := commitIn(t, root, "--allow-empty", "-m", "fix: x")
	if err != nil {
		t.Fatalf("err = %v, out = %q", err, out)
	}
}

// TestPreCommitHookAllowsTheEpicBranch proves an epic branch, feat plus a version, stays allowed.
func TestPreCommitHookAllowsTheEpicBranch(t *testing.T) {
	root := installedRepo(t, "feat/1.0.0-beta.3")
	out, err := commitIn(t, root, "--allow-empty", "-m", "feat: x")
	if err != nil {
		t.Fatalf("err = %v, out = %q", err, out)
	}
}

// TestCommitMsgHookRefusesATrailer proves a co-authored-by trailer is refused, whatever wrote it.
func TestCommitMsgHookRefusesATrailer(t *testing.T) {
	root := installedRepo(t, "fix/x")
	out, err := commitIn(t, root, "--allow-empty", "-m", "feat: x\n\nCo-authored-by: A <a@b.c>")
	if err == nil {
		t.Fatal("want a refusal, got none")
	}
	if !strings.Contains(out, "trailer") {
		t.Fatalf("out = %q", out)
	}
}

// TestPreCommitHookAllowsTheLinesOwnBranch proves a branch the line itself cuts, a parsed group's own
// Branch, commits cleanly through the real installed hooks, not a hand-typed shell copy of its shape.
func TestPreCommitHookAllowsTheLinesOwnBranch(t *testing.T) {
	text := "### [TG-10.1] One backlog grammar\n```yaml\ntype: fix\n```\n"
	parsed := backlog.Parse(text)
	if len(parsed.Groups) != 1 {
		t.Fatalf("groups = %d", len(parsed.Groups))
	}
	branch := parsed.Groups[0].Branch()
	root := installedRepo(t, branch)
	out, err := commitIn(t, root, "--allow-empty", "-m", "fix: x")
	if err != nil {
		t.Fatalf("branch = %q, err = %v, out = %q", branch, err, out)
	}
}

// TestCommitMsgHookEnforcesARepoPolicysTrailerPattern proves a repo's own .komodo/policy.json widens
// what the commit-msg hook refuses, since it reads the loaded policy, not a fixed shell pattern list.
func TestCommitMsgHookEnforcesARepoPolicysTrailerPattern(t *testing.T) {
	root := installedRepo(t, "fix/x")
	policy := `{"critical_refs": ["main"], "trailer_patterns": ["(?im)^signed-off-by\\s*[:=]"]}`
	if err := os.WriteFile(filepath.Join(root, ".komodo", "policy.json"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := commitIn(t, root, "--allow-empty", "-m", "fix: x\n\nSigned-off-by: A <a@b.c>")
	if err == nil {
		t.Fatal("want a refusal, got none")
	}
	if !strings.Contains(out, "trailer") {
		t.Fatalf("out = %q", out)
	}
}

// TestPreCommitHookSkipsADetachedHead proves a rebase's detached HEAD never trips the branch check.
func TestPreCommitHookSkipsADetachedHead(t *testing.T) {
	root := installedRepo(t, "fix/x")
	if out, err := commitIn(t, root, "--allow-empty", "-m", "fix: seed"); err != nil {
		t.Fatalf("seed commit: err = %v, out = %q", err, out)
	}
	sha, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	checkout := exec.Command("git", "checkout", "-q", strings.TrimSpace(string(sha)))
	checkout.Dir = root
	if out, err := checkout.CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v: %s", err, out)
	}
	out, err := commitIn(t, root, "--allow-empty", "-m", "fix: on a detached head")
	if err != nil {
		t.Fatalf("err = %v, out = %q", err, out)
	}
}

// TestPushProblemRefusesALeasedBranchExceptToItsOwnRun proves a person's push to a live lease is refused
// with the group, pid and lapse time, the lease's own run passes, and a lapsed lease frees the branch.
func TestPushProblemRefusesALeasedBranchExceptToItsOwnRun(t *testing.T) {
	root, _, _ := pushRepo(t, "a.txt", "a\n")
	holder, ok := proc.Of(os.Getpid())
	if !ok {
		t.Skip("this platform cannot name a process by its start time")
	}
	now := time.Now()
	if err := lease.Take(root, "TG-1", "feat/held", holder, now); err != nil {
		t.Fatal(err)
	}
	policy := guard.DefaultPolicy()
	t.Setenv(lease.RunEnv, "")
	problem := PushProblem(root, "refs/heads/feat/held", policy, now)
	if !strings.Contains(problem, "TG-1") || !strings.Contains(problem, strconv.Itoa(os.Getpid())) || !strings.Contains(problem, "until") {
		t.Fatalf("problem = %q, want the group, pid and lapse time", problem)
	}
	if problem := PushProblem(root, "refs/heads/feat/free", policy, now); problem != "" {
		t.Fatalf("an unleased branch refused: %q", problem)
	}
	if problem := PushProblem(root, "refs/heads/feat/held", policy, now.Add(lease.TTL+time.Minute)); problem != "" {
		t.Fatalf("a lapsed lease refused: %q", problem)
	}
	t.Setenv(lease.RunEnv, strconv.Itoa(os.Getpid()))
	if problem := PushProblem(root, "refs/heads/feat/held", policy, now); problem != "" {
		t.Fatalf("the lease's own run refused: %q", problem)
	}
}

func TestStampPublishesACleanBuildAndDropsADirtyOnesStamp(t *testing.T) {
	root := t.TempDir()
	gitCommand(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "main.go")
	gitCommand(t, root, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "seed")
	head := gitCommand(t, root, "rev-parse", "HEAD")
	var published []string
	previous := publish
	publish = func(path string) string { published = append(published, path); return path }
	t.Cleanup(func() { publish = previous })
	build := func(root string, _ io.Writer) (string, error) {
		path := filepath.Join(root, "bin", "komodo")
		return path, os.MkdirAll(filepath.Dir(path), 0o755)
	}
	install := func(string) ([]string, error) { return nil, nil }
	if _, stamped, err := Stamp(root, t.TempDir(), head, build, install, io.Discard); err != nil || !stamped {
		t.Fatalf("stamped = %v, %v; want a clean tree stamped", stamped, err)
	}
	if len(published) != 1 {
		t.Fatalf("published = %v, want the clean build once", published)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\n// edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stamped, err := Stamp(root, t.TempDir(), head, build, install, io.Discard); err != nil || stamped {
		t.Fatalf("stamped = %v, %v; want a dirty tree left unstamped", stamped, err)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", BuiltFrom)); !os.IsNotExist(err) {
		t.Fatalf("stamp still present (%v); a dirty build is no commit's", err)
	}
	if len(published) != 1 {
		t.Fatalf("published = %v; a dirty build must never reach the hooks", published)
	}
}
