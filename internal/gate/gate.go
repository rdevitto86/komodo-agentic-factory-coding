// Package gate is the local precheck: mechanical, model-free, before every commit and push.
package gate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
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

// Command builds a check that runs one command in the repo root, pinned to the repo's toolchain.
func Command(name, root string, args ...string) Check {
	return Check{Name: name, Run: func(out io.Writer) error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		cmd.Stdout, cmd.Stderr = out, out
		cmd.Env = childEnv()
		if toolchain, err := Toolchain(root); err == nil {
			cmd.Env = append(cmd.Env, "GOTOOLCHAIN="+toolchain)
		}
		return cmd.Run()
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

// LocalTarget is the binary this host builds for its own platform.
func LocalTarget() Target {
	name := fmt.Sprintf("komodo-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return Target{Name: name, GOOS: runtime.GOOS, Arch: runtime.GOARCH}
}

// changelogVersionRe matches the first version heading in the changelog, which names the build.
var changelogVersionRe = regexp.MustCompile(`(?m)^## (\S+)`)

// buildFlags are the flags that make a rebuild of one commit byte-identical, stamping its version and commit.
func buildFlags(root string) []string {
	version, commit := "dev", "unknown"
	if data, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md")); err == nil {
		if match := changelogVersionRe.FindSubmatch(data); match != nil {
			version = string(match[1])
		}
	}
	if out, err := exec.Command("git", "-C", root, "rev-parse", "--short=12", "HEAD").Output(); err == nil {
		commit = strings.TrimSpace(string(out))
	}
	ldflags := fmt.Sprintf("-s -w -X main.version=%s -X main.commit=%s", version, commit)
	return []string{"-trimpath", "-buildvcs=false", "-ldflags", ldflags}
}

// Build compiles one target into dir, pinned to the repo's toolchain, and returns the path it wrote.
func Build(root, dir string, target Target) (string, error) {
	toolchain, err := Toolchain(root)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, target.Name)
	args := append([]string{"build", "-o", path}, buildFlags(root)...)
	cmd := exec.Command("go", append(args, "./cmd/komodo")...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0", "GOOS="+target.GOOS, "GOARCH="+target.Arch, "GOTOOLCHAIN="+toolchain)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build %s: %v: %s", target.Name, err, strings.TrimSpace(stderr.String()))
	}
	return path, nil
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

const hookScript = `#!/bin/sh
# Runs the local gate from the checkout being committed, else this host's built binary. Written by komodo gate --install.
set -e
# The toolkit's own checkout gates from its source, so the guard table and comment rules are the ones committed.
top=$(git rev-parse --show-toplevel 2>/dev/null || true)
if [ -n "$top" ] && [ -f "$top/cmd/komodo/main.go" ]; then
  cd "$top"
  if [ "$(basename "$0")" = "pre-push" ]; then
    exec go run ./cmd/komodo gate --fuzz 10s
  fi
  exec go run ./cmd/komodo gate
fi
# The shared git dir sits in the main checkout, where bin/ lives, even when committing from a worktree.
common=$(git rev-parse --path-format=absolute --git-common-dir)
root=${common%/.git}
case "$(uname -s)-$(uname -m)" in
  Darwin-arm64) bin="$root/bin/komodo-darwin-arm64" ;;
  Darwin-x86_64) bin="$root/bin/komodo-darwin-amd64" ;;
  Linux-x86_64) bin="$root/bin/komodo-linux-amd64" ;;
  Linux-aarch64) bin="$root/bin/komodo-linux-arm64" ;;
  MINGW*|MSYS*|CYGWIN*) bin="$root/bin/komodo-windows-amd64.exe" ;;
  *) echo "gate: no binary for this platform ($(uname -s)-$(uname -m)); run go build ./cmd/komodo yourself" >&2; exit 1 ;;
esac
if [ ! -x "$bin" ]; then
  echo "gate: no binary at $bin; run 'komodo gate --install' to build it" >&2
  exit 1
fi
# A push carries more weight than a commit, so it also fuzzes the parsers for a few seconds each.
if [ "$(basename "$0")" = "pre-push" ]; then
  exec "$bin" gate --fuzz 10s
fi
exec "$bin" gate
`

// Install writes the pre-commit and pre-push hooks that run this gate.
func Install(gitDir string) ([]string, error) {
	dir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var written []string
	for _, name := range []string{"pre-commit", "pre-push"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(hookScript), 0o755); err != nil {
			return nil, err
		}
		written = append(written, path)
	}
	return written, nil
}

// FuzzTarget is one fuzz function and the package that holds it.
type FuzzTarget struct {
	Name    string
	Package string
}

// FuzzTargets are the parsers the gate fuzzes: shell commands, task grammar, and ledger lines.
var FuzzTargets = []FuzzTarget{
	{Name: "FuzzCheck", Package: "./internal/guard"},
	{Name: "FuzzLex", Package: "./internal/guard"},
	{Name: "FuzzParse", Package: "./internal/backlog"},
	{Name: "FuzzRead", Package: "./internal/ledger"},
}

// FuzzChecks builds one check per fuzz target, each run for the given duration such as 10s.
func FuzzChecks(root, duration string) []Check {
	var checks []Check
	for _, target := range FuzzTargets {
		checks = append(checks, Command("fuzz "+target.Name, root,
			"go", "test", "-run=^$", "-fuzz=^"+target.Name+"$", "-fuzztime="+duration, target.Package))
	}
	return checks
}

// TestArgs is the go test command line: under the race detector when cgo can build it, else plain.
func TestArgs() []string {
	out, err := exec.Command("go", "env", "CGO_ENABLED").Output()
	if err == nil && strings.TrimSpace(string(out)) == "1" {
		return []string{"go", "test", "-race", "./..."}
	}
	return []string{"go", "test", "./..."}
}
