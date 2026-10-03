// Package gate is the local precheck: mechanical, model-free, before every commit and push.
package gate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"komodo/internal/backlog"
	"komodo/internal/changelog"
	"komodo/internal/comments"
	"komodo/internal/fsx"
	"komodo/internal/git"
	"komodo/internal/guard"
	"komodo/internal/lease"
	"komodo/internal/mount"
	"komodo/internal/proc"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Target is one komodo binary: its file name and the platform it is built for.
type Target struct {
	Name string
	GOOS string
	Arch string
}

// Check is one step of the gate, named for the line the runner prints.
type Check struct {
	Name string
	Run  func(io.Writer) error
}

// CommandTimeout bounds how long one gate command or build may run before it is killed; a test lowers it.
var CommandTimeout = proc.DefaultTimeout

// waitDelay bounds how long a killed command's output pipes may stay open after it exits.
const waitDelay = 5 * time.Second

// Run executes every check in order and stops at the first failure.
func Run(checks []Check, out io.Writer) error {
	for _, check := range checks {
		fmt.Fprintf(out, "gate: %s\n", check.Name)
		if err := check.Run(out); err != nil {
			return fmt.Errorf("%s: %w", check.Name, err)
		}
	}
	fmt.Fprintf(out, "gate: %d check(s) passed\n", len(checks))
	return nil
}

// Command builds a check that runs one command in the repo root, pinned to the repo's toolchain,
// killed with its whole process group once CommandTimeout passes.
func Command(name, root string, args ...string) Check {
	return CommandEnv(name, root, nil, args...)
}

// CommandEnv is Command with env appended to the child's environment, such as GOOS for a cross-platform vet.
func CommandEnv(name, root string, env []string, args ...string) Check {
	return Check{Name: name, Run: func(out io.Writer) error {
		ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Dir = root
		cmd.Stdout, cmd.Stderr = out, out
		cmd.Env = append(childEnv(), env...)
		if toolchain, err := Toolchain(root); err == nil {
			cmd.Env = append(cmd.Env, "GOTOOLCHAIN="+toolchain)
		}
		proc.Group(cmd)
		cmd.Cancel = func() error {
			proc.KillGroup(cmd)
			return nil
		}
		cmd.WaitDelay = waitDelay
		err := cmd.Run()
		proc.KillGroup(cmd)
		if errors.Is(err, exec.ErrWaitDelay) {
			err = nil
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("%s: timed out after %s", name, CommandTimeout)
		}
		return err
	}}
}

// hookGitVars are the variables git sets for a hook; a child inheriting them aims every git call at this repo.
var hookGitVars = []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_PREFIX", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_COMMON_DIR"}

// childEnv is this process's environment without the git variables a hook sets.
func childEnv() []string {
	var env []string
	for _, pair := range os.Environ() {
		name, _, _ := strings.Cut(pair, "=")
		dropped := false
		for _, gitVar := range hookGitVars {
			if name == gitVar {
				dropped = true
				break
			}
		}
		if !dropped {
			env = append(env, pair)
		}
	}
	return env
}

// Toolchain reads the toolchain version go.mod pins, for example "go1.27.1".
func Toolchain(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "toolchain "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", fmt.Errorf("go.mod names no toolchain")
}

// LocalTarget is the binary this host builds for itself: bin/komodo, which Windows names komodo.exe.
func LocalTarget() Target {
	name := "komodo"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return Target{Name: name, GOOS: runtime.GOOS, Arch: runtime.GOARCH}
}

// PlatformName is this platform's release asset name, such as komodo-darwin-arm64.
func PlatformName() string {
	name := fmt.Sprintf("komodo-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// buildFlags are the flags that make a rebuild of one commit byte-identical, stamping its version and commit.
func buildFlags(root string) []string {
	version, commit := "dev", "unknown"
	if text, err := changelog.Read(root); err == nil {
		if latest := changelog.Latest(text); latest != "" {
			version = latest
		}
	}
	if out, err := git.Run(root, "rev-parse", "--short=12", "HEAD"); err == nil {
		commit = out
	}
	ldflags := fmt.Sprintf("-s -w -X main.version=%s -X main.commit=%s", version, commit)
	return []string{"-trimpath", "-buildvcs=false", "-ldflags", ldflags}
}

// Build compiles one target into dir, pinned to the repo's toolchain, and returns the path it wrote;
// a hung compiler is killed, process group included, once CommandTimeout passes.
func Build(root, dir string, target Target) (string, error) {
	toolchain, err := Toolchain(root)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, target.Name)
	// The compiler writes beside the binary and a rename puts it in place, so no reader sees half a binary.
	temp := path + ".tmp"
	args := append([]string{"build", "-o", temp}, buildFlags(root)...)
	ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", append(args, "./cmd/komodo")...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0", "GOOS="+target.GOOS, "GOARCH="+target.Arch, "GOTOOLCHAIN="+toolchain)
	proc.Group(cmd)
	cmd.Cancel = func() error {
		proc.KillGroup(cmd)
		return nil
	}
	cmd.WaitDelay = waitDelay
	stderr := proc.NewBoundedWriter(proc.MaxOutput)
	cmd.Stderr = stderr
	runErr := cmd.Run()
	proc.KillGroup(cmd)
	if errors.Is(runErr, exec.ErrWaitDelay) {
		runErr = nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		_ = os.Remove(temp)
		return "", fmt.Errorf("build %s: timed out after %s: %s", target.Name, CommandTimeout, strings.TrimSpace(stderr.String()))
	}
	if runErr != nil {
		_ = os.Remove(temp)
		return "", fmt.Errorf("build %s: %v: %s", target.Name, runErr, strings.TrimSpace(stderr.String()))
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return "", err
	}
	return path, nil
}

// BuiltFrom is the file beside a built binary that records the commit it was built from.
const BuiltFrom = ".built-from"

// zeroOID is git's null object id, the "from" a checkout hook passes when there was no prior commit.
const zeroOID = "0000000000000000000000000000000000000000"

// emptyTreeOID is git's empty tree hash, the "from" side of a diff for a push with no prior commit.
const emptyTreeOID = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// changedBuildInputs reports whether a .go file, go.mod, or go.sum differs between two commits,
// treating the checkout hook's zero OID for a first commit as no change to rebuild.
func changedBuildInputs(root, from, to string) (bool, error) {
	if from == "" || from == zeroOID || from == to {
		return false, nil
	}
	return BuildInputsChanged(root, from, to)
}

// BuildInputsChanged reports whether a .go file, go.mod, or go.sum differs between two commits.
func BuildInputsChanged(root, from, to string) (bool, error) {
	if from == to {
		return false, nil
	}
	out, err := git.Run(root, "diff", "--name-only", from, to)
	if err != nil {
		return false, err
	}
	for _, name := range strings.Split(out, "\n") {
		if name == "go.mod" || name == "go.sum" || strings.HasSuffix(name, ".go") {
			return true, nil
		}
	}
	return false, nil
}

// resolveCommit returns ref's full commit SHA, or ref unchanged when it is empty, a name git cannot
// resolve, or the checkout hook's zero OID, so a name such as HEAD is never stamped literally.
func resolveCommit(root, ref string) string {
	if ref == "" || ref == zeroOID {
		return ref
	}
	if sha, err := git.Run(root, "rev-parse", "--verify", ref); err == nil {
		return sha
	}
	return ref
}

// SyncEnv marks a merge sync drives, so the post-merge hook's own rebuild step no-ops.
const SyncEnv = "KOMODO_SYNC"

// Rebuild builds this host's binary and stamps bin/.built-from with to, only when a .go file,
// go.mod or go.sum differs between from and to.
func Rebuild(root, from, to string, out io.Writer) error {
	if os.Getenv(SyncEnv) != "" {
		return nil
	}
	from, to = resolveCommit(root, from), resolveCommit(root, to)
	changed, err := changedBuildInputs(root, from, to)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	gitDir, err := git.Run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	_, _, err = Stamp(root, gitDir, to, BuildLocal, Install, out)
	return err
}

// publish puts a stamped build at the one path every hook runs; tests swap it.
var publish = mount.Publish

// Stamp builds this host's binary; a clean tree also installs the git hooks, records to, and publishes it.
// A dirty tree's build is no commit's, so it drops the stamp and publishes nothing.
func Stamp(
	root, gitDir, to string,
	build func(string, io.Writer) (string, error),
	install func(string) ([]string, error),
	out io.Writer,
) (path string, stamped bool, err error) {
	path, err = build(root, out)
	if err != nil {
		return "", false, err
	}
	stamp := filepath.Join(root, "bin", BuiltFrom)
	dirty, err := git.Run(root, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return "", false, err
	}
	if dirty != "" {
		if err := os.Remove(stamp); err != nil && !os.IsNotExist(err) {
			return "", false, err
		}
		return path, false, nil
	}
	if _, err := install(gitDir); err != nil {
		return "", false, err
	}
	if err := fsx.WriteFile(stamp, []byte(to+"\n"), 0o644); err != nil {
		return "", false, err
	}
	publish(path)
	return path, true, nil
}

// Sum returns the hex sha256 of one file.
func Sum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

// IsToolkit reports whether root is the toolkit's own checkout: cmd/komodo under the komodo module.
func IsToolkit(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "cmd", "komodo", "main.go")); err != nil {
		return false
	}
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(mod), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "module" {
			return fields[1] == "komodo"
		}
	}
	return false
}

// BuildLocalIfGoRepo builds this host's own binary in the toolkit's checkout, returning an empty
// path with no error in any other repo, so install still writes its hooks there.
func BuildLocalIfGoRepo(root string, out io.Writer) (string, error) {
	if !IsToolkit(root) {
		return "", nil
	}
	return BuildLocal(root, out)
}

// BuildLocal builds this host's own binary into root/bin and returns the path it wrote.
func BuildLocal(root string, out io.Writer) (string, error) {
	dir := filepath.Join(root, "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	target := LocalTarget()
	path, err := Build(root, dir, target)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(out, "built %s\n", target.Name)
	return path, nil
}

// hookScript is every git hook's whole body: one exec of the installed binary, which holds the hook's logic.
func hookScript(binary string) string {
	return "#!/bin/sh\n# Written by komodo gate --install; komodo git-hook holds every hook's logic.\n" +
		"exec \"" + filepath.ToSlash(binary) + "\" git-hook \"$(basename \"$0\")\" \"$@\"\n"
}

// Install writes the seven git hooks, each a one-line trampoline into the installed binary's git-hook command.
func Install(gitDir string) ([]string, error) {
	binary, err := mount.HookPath()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var written []string
	for _, name := range []string{"pre-commit", "commit-msg", "pre-push", "post-commit", "post-merge", "post-checkout", "post-rewrite"} {
		path := filepath.Join(dir, name)
		if err := fsx.WriteFile(path, []byte(hookScript(binary)), 0o755); err != nil {
			return nil, err
		}
		written = append(written, path)
	}
	return written, nil
}

// kebabName matches a plain <kebab-name>, the shape a person's own branch fragment takes.
var kebabName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// TrailerProblem reports why the commit-msg hook refuses message, empty when it may proceed.
func TrailerProblem(message string, policy guard.Policy) string {
	if policy.HasTrailer(message) {
		return "commit message carries a co-author or generated-by trailer; remove it and commit again"
	}
	return ""
}

// BranchProblem reports why the pre-commit hook refuses branch, empty when it may commit; a critical
// ref, an epic branch, and a slug backlog.IsGroupSlug already recognizes are all allowed.
func BranchProblem(branch string, policy guard.Policy) string {
	if branch == "" {
		return ""
	}
	if policy.IsCritical(branch) {
		return fmt.Sprintf("commit on %s refused; create a branch first", branch)
	}
	if guard.IsEpicBranch(branch) {
		return ""
	}
	typ, slug, ok := strings.Cut(branch, "/")
	if ok && slices.Contains(backlog.Types, typ) && (kebabName.MatchString(slug) || backlog.IsGroupSlug(slug)) {
		return ""
	}
	return fmt.Sprintf("branch %q is not <type>/<kebab-name>; rename it, or let the line cut its own", branch)
}

// PushProblem reports why the pre-push hook refuses the remote ref, empty when it may go: a branch
// BranchProblem refuses, or one a live builder's lease holds against anyone but its own run.
func PushProblem(dir, ref string, policy guard.Policy, now time.Time) string {
	branch := strings.TrimPrefix(ref, "refs/heads/")
	if problem := BranchProblem(branch, policy); problem != "" {
		return problem
	}
	if held, ok := lease.Held(dir, branch, now); ok && !held.Own() {
		return "push to " + held.Refusal()
	}
	return ""
}

// hunkHeader captures a unified diff hunk's new-file start line, from a header such as "@@ -1,2 +3,4 @@".
var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// diffAddedLines parses a unified diff and returns, per new-file path, the line numbers it adds or changes.
func diffAddedLines(diff string) map[string][]int {
	added := map[string][]int{}
	path, line := "", 0
	for _, raw := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(raw, "+++ "):
			name := strings.TrimPrefix(strings.TrimPrefix(raw, "+++ "), "b/")
			if name == "/dev/null" {
				path = ""
			} else {
				path = name
			}
		case strings.HasPrefix(raw, "@@ "):
			if match := hunkHeader.FindStringSubmatch(raw); match != nil {
				line, _ = strconv.Atoi(match[1])
			}
		case strings.HasPrefix(raw, "+") && !strings.HasPrefix(raw, "+++"):
			if path != "" {
				added[path] = append(added[path], line)
			}
			line++
		case strings.HasPrefix(raw, " "):
			line++
		}
	}
	return added
}

// StagedDiffLines returns, per path relative to root, the line numbers a staged diff adds or changes.
func StagedDiffLines(root string) (map[string][]int, error) {
	out, err := git.Run(root, "diff", "--cached", "-U0", "--no-color")
	if err != nil {
		return nil, err
	}
	return diffAddedLines(out), nil
}

// CommentsCheck builds a gate check that lints a staged diff's changed lines, or every tracked file
// when the lint's rule fingerprint has moved past the repo's last recorded sweep.
func CommentsCheck(root, require string) Check {
	return Check{Name: "komodo comments check", Run: func(out io.Writer) error {
		var problems []string
		sweep := comments.NeedsSweep(root)
		if sweep {
			swept, err := comments.Check(root, comments.TrackedFiles(root), require)
			if err != nil {
				return err
			}
			problems = swept
		} else {
			added, err := StagedDiffLines(root)
			if err != nil {
				return err
			}
			diffed, err := comments.CheckDiff(root, added, require)
			if err != nil {
				return err
			}
			problems = diffed
		}
		for _, problem := range problems {
			fmt.Fprintln(out, problem)
		}
		if len(problems) > 0 {
			return fmt.Errorf("%d comment problem(s)", len(problems))
		}
		if sweep {
			if err := comments.RecordSweep(root); err != nil {
				return err
			}
		}
		return nil
	}}
}

// FuzzTarget is one fuzz function and the package that holds it.
type FuzzTarget struct {
	Name    string
	Package string
}

// ownFuzzTargets are the parsers the gate itself fuzzes: shell commands, task grammar, and ledger lines.
var ownFuzzTargets = []FuzzTarget{
	{Name: "FuzzCheck", Package: "./internal/guard"},
	{Name: "FuzzTokenize", Package: "./internal/guard"},
	{Name: "FuzzParse", Package: "./internal/backlog"},
	{Name: "FuzzRead", Package: "./internal/ledger"},
}

// FuzzTargets are every fuzz target the gate runs: its own, plus each registered mount's own.
func FuzzTargets() []FuzzTarget {
	targets := append([]FuzzTarget{}, ownFuzzTargets...)
	for _, target := range mount.FuzzTargets() {
		targets = append(targets, FuzzTarget{Name: target.Name, Package: target.Package})
	}
	return targets
}

// FuzzChecksFor builds one check per named fuzz target, each run for the given duration such as 10s.
func FuzzChecksFor(root, duration string, targets []FuzzTarget) []Check {
	var checks []Check
	for _, target := range targets {
		checks = append(checks, Command("fuzz "+target.Name, root,
			"go", "test", "-run=^$", "-fuzz=^"+target.Name+"$", "-fuzztime="+duration, target.Package))
	}
	return checks
}

// PushedFiles lists the files a push's range from..to added or changed, diffing against the empty tree
// when from is empty or unset, as a new branch or tag push.
func PushedFiles(root, from, to string) ([]string, error) {
	if from == "" || from == zeroOID {
		from = emptyTreeOID
	}
	if from == to {
		return nil, nil
	}
	out, err := git.Run(root, "diff", "--name-only", from, to)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, name := range strings.Split(out, "\n") {
		if name != "" {
			paths = append(paths, name)
		}
	}
	return paths, nil
}

// PushedFuzzTargets returns the fuzz targets whose package is among a push's changed paths.
func PushedFuzzTargets(paths []string) []FuzzTarget {
	var touched []FuzzTarget
	for _, target := range FuzzTargets() {
		dir := strings.TrimPrefix(target.Package, "./")
		for _, path := range paths {
			if path == dir || strings.HasPrefix(path, dir+"/") {
				touched = append(touched, target)
				break
			}
		}
	}
	return touched
}

// PushChecks scopes a gate to what a push touched: build only when a .go file, go.mod or go.sum changed,
// and a fuzz check only for a touched package. With no to it runs every build check and fuzz target.
func PushChecks(root, from, to, fuzzDuration string, build []Check) ([]Check, error) {
	if to == "" {
		checks := append([]Check{}, build...)
		if fuzzDuration != "" {
			checks = append(checks, FuzzChecksFor(root, fuzzDuration, FuzzTargets())...)
		}
		return checks, nil
	}
	paths, err := PushedFiles(root, from, to)
	if err != nil {
		return build, err
	}
	touchesCode := false
	for _, path := range paths {
		if path == "go.mod" || path == "go.sum" || strings.HasSuffix(path, ".go") {
			touchesCode = true
			break
		}
	}
	var checks []Check
	if touchesCode {
		checks = append(checks, build...)
	}
	if fuzzDuration != "" {
		checks = append(checks, FuzzChecksFor(root, fuzzDuration, PushedFuzzTargets(paths))...)
	}
	return checks, nil
}

// TestArgs is the go test command line: under the race detector when cgo can build it, and, when fresh,
// uncached and in shuffled order, so a result never leans on a cache or on the order tests ran in.
func TestArgs(fresh bool) []string {
	args := []string{"go", "test"}
	if out, err := exec.Command("go", "env", "CGO_ENABLED").Output(); err == nil && strings.TrimSpace(string(out)) == "1" {
		args = append(args, "-race")
	}
	if fresh {
		args = append(args, "-count=1", "-shuffle=on")
	}
	return append(args, "./...")
}
