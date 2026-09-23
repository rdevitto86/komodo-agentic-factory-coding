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
	"sort"
	"strings"
)

// ManifestName is the checksum file that pins every prebuilt binary.
const ManifestName = "MANIFEST.sha256"

// Target is one prebuilt binary: its file name and the platform it is built for.
type Target struct {
	Name string
	GOOS string
	Arch string
}

// Targets are the three platforms the toolkit ships a binary for.
var Targets = []Target{
	{Name: "komodo-darwin-arm64", GOOS: "darwin", Arch: "arm64"},
	{Name: "komodo-windows-amd64.exe", GOOS: "windows", Arch: "amd64"},
	{Name: "komodo-linux-amd64", GOOS: "linux", Arch: "amd64"},
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

// Command builds a check that runs one command in the repo root.
func Command(name, root string, args ...string) Check {
	return Check{Name: name, Run: func(out io.Writer) error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		cmd.Stdout, cmd.Stderr = out, out
		return cmd.Run()
	}}
}

// Binaries builds the check that rebuilds every target and compares it to the manifest.
func Binaries(root string) Check {
	return Check{Name: "binaries match " + ManifestName, Run: func(out io.Writer) error {
		return VerifyBinaries(root, out)
	}}
}

// buildFlags are the flags that make a rebuild byte-identical.
var buildFlags = []string{"-trimpath", "-buildvcs=false", "-ldflags", "-s -w"}

// Build compiles one target into dir and returns the path it wrote.
func Build(root, dir string, target Target) (string, error) {
	path := filepath.Join(dir, target.Name)
	args := append([]string{"build", "-o", path}, buildFlags...)
	cmd := exec.Command("go", append(args, "./cmd/komodo")...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+target.GOOS, "GOARCH="+target.Arch)
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

// ReadManifest parses a sha256 manifest into name to digest.
func ReadManifest(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sums := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		sums[strings.TrimPrefix(fields[1], "*")] = fields[0]
	}
	return sums, nil
}

// WriteManifest rebuilds every target into bin/ and writes the checksum manifest.
func WriteManifest(root string, out io.Writer) error {
	dir := filepath.Join(root, "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	sums := map[string]string{}
	for _, target := range Targets {
		path, err := Build(root, dir, target)
		if err != nil {
			return err
		}
		sum, err := Sum(path)
		if err != nil {
			return err
		}
		sums[target.Name] = sum
		fmt.Fprintf(out, "built %s\n", target.Name)
	}
	return writeSums(filepath.Join(dir, ManifestName), sums)
}

// writeSums renders a manifest sorted by name.
func writeSums(path string, sums map[string]string) error {
	names := make([]string, 0, len(sums))
	for name := range sums {
		names = append(names, name)
	}
	sort.Strings(names)
	var body strings.Builder
	for _, name := range names {
		fmt.Fprintf(&body, "%s  %s\n", sums[name], name)
	}
	return os.WriteFile(path, []byte(body.String()), 0o644)
}

// VerifyBinaries rebuilds every target and fails when one differs from the manifest.
func VerifyBinaries(root string, out io.Writer) error {
	manifest := filepath.Join(root, "bin", ManifestName)
	want, err := ReadManifest(manifest)
	if err != nil {
		return fmt.Errorf("read %s: %w", manifest, err)
	}
	dir, err := os.MkdirTemp("", "komodo-gate")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	for _, target := range Targets {
		expected, ok := want[target.Name]
		if !ok {
			return fmt.Errorf("%s names no %s", ManifestName, target.Name)
		}
		path, err := Build(root, dir, target)
		if err != nil {
			return err
		}
		got, err := Sum(path)
		if err != nil {
			return err
		}
		if got != expected {
			return fmt.Errorf("%s changed: manifest %s, rebuild %s; run `komodo gate --rebuild`", target.Name, expected[:12], got[:12])
		}
		shipped := filepath.Join(root, "bin", target.Name)
		onDisk, err := Sum(shipped)
		if err != nil {
			return fmt.Errorf("read %s: %w", shipped, err)
		}
		if onDisk != expected {
			return fmt.Errorf("%s on disk does not match %s; run `komodo gate --rebuild`", target.Name, ManifestName)
		}
		fmt.Fprintf(out, "  %s %s\n", target.Name, got[:12])
	}
	return nil
}

const hookScript = `#!/bin/sh
# Runs the local gate through the platform's prebuilt binary. Written by komodo gate --install.
set -e
root=$(git rev-parse --show-toplevel)
case "$(uname -s)-$(uname -m)" in
  Darwin-arm64) bin="$root/bin/komodo-darwin-arm64" ;;
  Linux-x86_64) bin="$root/bin/komodo-linux-amd64" ;;
  *) bin="$root/bin/komodo-windows-amd64.exe" ;;
esac
if [ ! -x "$bin" ]; then
  echo "gate: no binary at $bin" >&2
  exit 1
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
