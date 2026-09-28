package release

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"komodo/internal/gate"
	"komodo/internal/git"
	"komodo/internal/line"
	"komodo/internal/pr"
)

// SumsFile is the checksum manifest a release ships beside its binaries, which install verifies.
const SumsFile = "SHA256SUMS"

// buildAssets, runTests and forge build, test and publish; a test swaps them to skip the compile and the forge.
var (
	buildAssets = BuildAssets
	runTests    = goTest
	forge       = pr.Run
)

// Publish builds every target into dist, runs the tests, writes SHA256SUMS and publishes a GitHub Release
// for the changelog's newest version, whose tag must point at a clean HEAD; it returns the release URL.
func Publish(root, dist string, out io.Writer) (string, error) {
	if sessionEnv() {
		return "", fmt.Errorf("refusing to publish from a line session; komodo release publish runs on the owner's machine")
	}
	text, err := ReadChangelog(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return "", err
	}
	version := Latest(text)
	if version == "" {
		return "", fmt.Errorf("the changelog names no version to release")
	}
	name := TagName(version)
	if err := tagAtCleanHead(root, name); err != nil {
		return "", err
	}

	paths, err := buildAssets(root, dist, out)
	if err != nil {
		return "", err
	}
	if err := runTests(root, out); err != nil {
		return "", fmt.Errorf("tests failed; nothing was published: %w", err)
	}
	sums, err := WriteSums(dist, paths)
	if err != nil {
		return "", err
	}
	args := []string{
		"release", "create", name, "--verify-tag", "--title", "komodo " + version, "--notes", releaseNotes(text, version),
	}
	if strings.Contains(version, "-") {
		args = append(args, "--prerelease")
	}
	args = append(args, paths...)
	// The forge credential is read here alone; the build and the tests ran without it.
	url, err := forge(root, append(args, sums)...)
	if err != nil {
		return "", err
	}
	fmt.Fprintln(out, "published", name)
	return url, nil
}

// WriteSums writes SHA256SUMS into dir, one "hash  name" line per path sorted by name, and returns its path.
func WriteSums(dir string, paths []string) (string, error) {
	sorted := append([]string(nil), paths...)
	sort.Slice(sorted, func(i, j int) bool { return filepath.Base(sorted[i]) < filepath.Base(sorted[j]) })
	lines := make([]string, 0, len(sorted))
	for _, path := range sorted {
		sum, err := gate.Sum(path)
		if err != nil {
			return "", err
		}
		lines = append(lines, sum+"  "+filepath.Base(path))
	}
	path := filepath.Join(dir, SumsFile)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// tagAtCleanHead refuses a dirty tracked tree or a tag off HEAD, since the binaries build from HEAD.
func tagAtCleanHead(root, name string) error {
	dirty, err := git.Run(root, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if dirty != "" {
		return fmt.Errorf("refusing to publish %s from a working tree with uncommitted changes", name)
	}
	head, err := git.Run(root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	tagged, err := git.Run(root, "rev-parse", "--verify", "--quiet", name+"^{commit}")
	if err != nil || tagged == "" {
		return fmt.Errorf("no tag %s to publish; run komodo tag on the default branch first", name)
	}
	if tagged != head {
		return fmt.Errorf("%s points at %s, not HEAD (%s); check out the tag to publish it", name, tagged, head)
	}
	return nil
}

// releaseNotes is the changelog body under one version's heading, the release's description.
func releaseNotes(text, version string) string {
	for _, item := range Versions(text) {
		if item.Number == version {
			return item.Body
		}
	}
	return ""
}

// sessionEnv reports whether this process runs with the line's scrubbed environment, which a session gets.
func sessionEnv() bool {
	return os.Getenv("GIT_TERMINAL_PROMPT") == "0" &&
		os.Getenv("GIT_CONFIG_KEY_0") == "credential.helper" &&
		os.Getenv("GIT_CONFIG_VALUE_0") == ""
}

// goTest runs the whole test suite pinned to the repo's toolchain, with every forge credential scrubbed.
func goTest(root string, out io.Writer) error {
	args := gate.TestArgs()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = root
	cmd.Env = line.Scrub(os.Environ())
	if toolchain, err := gate.Toolchain(root); err == nil {
		cmd.Env = append(cmd.Env, "GOTOOLCHAIN="+toolchain)
	}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(args, " "), err)
	}
	return nil
}
