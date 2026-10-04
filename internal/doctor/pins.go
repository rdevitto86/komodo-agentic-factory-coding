package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"komodo/internal/gate"
	"komodo/internal/git"
	"komodo/internal/mount"
	"komodo/internal/proc"
	"komodo/internal/profile"
	"komodo/internal/toolkit"
)

// waitDelay bounds how long a killed command's output pipes may stay open after it exits.
const waitDelay = 5 * time.Second

// toolTimeout bounds how long a host tool may run before its process group is killed; a test lowers it.
var toolTimeout = proc.DefaultTimeout

// modelHasVersion matches a digit, which every full model ID carries and a bare alias never does.
var modelHasVersion = regexp.MustCompile(`[0-9]`)

// bareModelWords are generic, vendor-free words that name no version, such as an alias would use.
var bareModelWords = map[string]bool{"latest": true, "default": true}

// goToolchain is the machine's Go toolchain in root, "go1.27.1", or an error naming the command and its stderr.
var goToolchain = func(root string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "env", "GOVERSION")
	cmd.Dir = root
	proc.Group(cmd)
	cmd.Cancel = func() error {
		proc.KillGroup(cmd)
		return nil
	}
	cmd.WaitDelay = waitDelay
	stdout, stderr := proc.NewBoundedWriter(proc.MaxOutput), proc.NewBoundedWriter(proc.MaxOutput)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	proc.KillGroup(cmd)
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("go env GOVERSION: timed out after %s: %s", toolTimeout, strings.TrimSpace(stderr.String()))
	}
	if err != nil {
		return "", fmt.Errorf("go env GOVERSION: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// checkPins reports the profile's model IDs, the go.mod toolchain and the built komodo binary
// that differ from the pin every machine must run (REQ-2, decision 0001).
func checkPins(root string) []Problem {
	current := profile.Select(root)
	var problems []Problem
	problems = append(problems, checkModelIDs(current)...)
	problems = append(problems, checkToolchain(root)...)
	problems = append(problems, checkRelease(root)...)
	problems = append(problems, checkPinnedRelease(root, current.Mode)...)
	return problems
}

// checkModelIDs reports a tier, including the reviewer's, whose machine names a bare alias instead of a full model ID.
func checkModelIDs(current profile.Profile) []Problem {
	if current.Host == "" {
		return nil
	}
	tiers := map[string]mount.Machine{
		"light":    current.Tiers.Light,
		"standard": current.Tiers.Standard,
		"heavy":    current.Tiers.Heavy,
		"reviewer": current.Tiers.Reviewer,
	}
	names := make([]string, 0, len(tiers))
	for name := range tiers {
		names = append(names, name)
	}
	sort.Strings(names)
	var problems []Problem
	for _, name := range names {
		machine := tiers[name]
		if machine.Local() {
			continue
		}
		if machine.Model == "" || bareModelWords[strings.ToLower(machine.Model)] || !modelHasVersion.MatchString(machine.Model) {
			problems = append(problems, Problem{"pins", name,
				fmt.Sprintf("model %q is not a full ID (decision 0001)", machine.Model)})
		}
	}
	return problems
}

// checkToolchain reports when this machine's Go toolchain differs from go.mod's toolchain pin,
// or when this machine's toolchain could not be read at all.
func checkToolchain(root string) []Problem {
	declared, err := gate.Toolchain(root)
	if err != nil {
		return nil
	}
	running, err := goToolchain(root)
	if err != nil {
		return []Problem{{"pins", "go.mod", "could not read this machine's Go toolchain: " + err.Error()}}
	}
	if running != "" && running != declared {
		return []Problem{{"pins", "go.mod",
			fmt.Sprintf("the toolchain is pinned to %s; this machine runs %s", declared, running)}}
	}
	return nil
}

// checkRelease reports when the built komodo binary's build inputs differ from HEAD's, so a merge
// with a real code change is caught, while a docs-only or repeat-run commit never trips it.
func checkRelease(root string) []Problem {
	head, err := git.Run(root, "rev-parse", "HEAD")
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(root, "bin", gate.BuiltFrom))
	if err != nil {
		return nil
	}
	built := strings.TrimSpace(string(data))
	if built == head {
		return nil
	}
	if changed, err := gate.BuildInputsChanged(root, built, head); err == nil && !changed {
		return nil
	}
	return []Problem{{"pins", filepath.Join("bin", gate.BuiltFrom),
		fmt.Sprintf("the komodo binary was built from %s, not HEAD (%s); run komodo gate --rebuild", built, head)}}
}

// PinnedRelease is the published komodo release the mode's profile pins, with no v, or "" when it pins none.
func PinnedRelease(root, mode string) string {
	data, err := fs.ReadFile(toolkit.FS(root), "profiles/"+mode+".json")
	if err != nil {
		return ""
	}
	var pinned struct {
		Release string `json:"release"`
	}
	if err := json.Unmarshal(data, &pinned); err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(pinned.Release), "v")
}

// ReleaseBinary is where a published release's binary for this platform installs, under ~/.komodo/bin.
func ReleaseBinary() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".komodo", "bin", gate.PlatformName()), nil
}

// ReleaseVersion runs one komodo binary's version command and returns the version it names, with
// no v, or an error naming the command and its stderr.
func ReleaseVersion(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "version")
	proc.Group(cmd)
	cmd.Cancel = func() error {
		proc.KillGroup(cmd)
		return nil
	}
	cmd.WaitDelay = waitDelay
	stdout, stderr := proc.NewBoundedWriter(proc.MaxOutput), proc.NewBoundedWriter(proc.MaxOutput)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	proc.KillGroup(cmd)
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("%s version: timed out after %s: %s", path, toolTimeout, strings.TrimSpace(stderr.String()))
	}
	if err != nil {
		return "", fmt.Errorf("%s version: %v: %s", path, err, strings.TrimSpace(stderr.String()))
	}
	fields := strings.Fields(stdout.String())
	if len(fields) < 2 {
		return "", fmt.Errorf("%s printed no version", path)
	}
	return strings.TrimPrefix(fields[1], "v"), nil
}

// checkPinnedRelease reports, outside the toolkit's own checkout, an installed komodo other than the profile's pin.
func checkPinnedRelease(root, mode string) []Problem {
	if _, err := os.Stat(filepath.Join(root, "cmd", "komodo", "main.go")); err == nil {
		return nil
	}
	pin := PinnedRelease(root, mode)
	if pin == "" {
		return nil
	}
	path, err := ReleaseBinary()
	if err != nil {
		return []Problem{{"pins", "komodo", "could not find the release binary: " + err.Error()}}
	}
	installed, err := ReleaseVersion(path)
	if err != nil {
		return []Problem{{"pins", path, fmt.Sprintf("could not read its version: %v; run komodo sync", err)}}
	}
	if installed != pin {
		return []Problem{{"pins", path,
			fmt.Sprintf("runs komodo %s; the profile pins release %s; run komodo sync", installed, pin)}}
	}
	return nil
}
