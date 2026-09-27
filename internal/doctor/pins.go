package doctor

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"komodo/internal/gate"
	"komodo/internal/git"
	"komodo/internal/profile"
	"komodo/internal/toolkit"
)

// modelHasVersion matches a digit, which every full model ID carries and a bare alias never does.
var modelHasVersion = regexp.MustCompile(`[0-9]`)

// bareModelWords are generic, vendor-free words that name no version, such as an alias would use.
var bareModelWords = map[string]bool{"latest": true, "default": true}

// goToolchain is the Go toolchain the machine's go command runs in root, "go1.27.1"; a test swaps it.
var goToolchain = func(root string) string {
	cmd := exec.Command("go", "env", "GOVERSION")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// cliVersion runs a host's own CLI with --version and returns what it printed, trimmed; a test swaps it.
var cliVersion = func(name string) (string, error) {
	out, err := exec.Command(name, "--version").Output()
	return strings.TrimSpace(string(out)), err
}

// checkPins reports the host CLI version, the profile's model IDs, the go.mod toolchain and the
// built komodo binary that differ from the pin every machine must run (REQ-2, decision 0006).
func checkPins(root string) []Problem {
	current := profile.Select(root)
	var problems []Problem
	problems = append(problems, checkHostVersion(current)...)
	problems = append(problems, checkModelIDs(current)...)
	problems = append(problems, checkToolchain(root)...)
	problems = append(problems, checkRelease(root)...)
	problems = append(problems, checkPinnedRelease(root, current.Mode)...)
	return problems
}

// checkHostVersion reports when the installed host's own CLI reports a version other than its pin.
func checkHostVersion(current profile.Profile) []Problem {
	if current.Host == "" {
		return nil
	}
	installed, err := cliVersion(current.Host)
	if err != nil {
		return []Problem{{"pins", current.Host, "could not read its version: " + err.Error()}}
	}
	if !strings.Contains(installed, current.HostVersion) {
		return []Problem{{"pins", current.Host,
			fmt.Sprintf("reports %s; the line is pinned to %s", installed, current.HostVersion)}}
	}
	return nil
}

// checkModelIDs reports a role whose resolved machine names a bare alias instead of a full model ID.
func checkModelIDs(current profile.Profile) []Problem {
	if current.Host == "" {
		return nil
	}
	names := make([]string, 0, len(current.Roles))
	for name := range current.Roles {
		names = append(names, name)
	}
	sort.Strings(names)
	var problems []Problem
	for _, name := range names {
		machine, ok := current.Machine(name)
		if !ok || machine.Local() {
			continue
		}
		if machine.Model == "" || bareModelWords[strings.ToLower(machine.Model)] || !modelHasVersion.MatchString(machine.Model) {
			problems = append(problems, Problem{"pins", name,
				fmt.Sprintf("model %q is not a full ID (decision 0006)", machine.Model)})
		}
	}
	return problems
}

// checkToolchain reports when this machine's Go toolchain differs from go.mod's toolchain pin.
func checkToolchain(root string) []Problem {
	declared, err := gate.Toolchain(root)
	if err != nil {
		return nil
	}
	if running := goToolchain(root); running != "" && running != declared {
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
	return filepath.Join(home, ".komodo", "bin", gate.LocalTarget().Name), nil
}

// ReleaseVersion runs one komodo binary's version command and returns the version it names, with no v.
func ReleaseVersion(path string) (string, error) {
	out, err := exec.Command(path, "version").Output()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
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
