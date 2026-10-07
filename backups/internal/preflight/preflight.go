// Package preflight checks the host before a run: login, credentials, sandbox, and budget.
package preflight

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"komodo/internal/doctor"
	"komodo/internal/install"
	"komodo/internal/mount"
	"komodo/internal/proc"
	"komodo/internal/profile"
)

// toolTimeout bounds how long a tool's own status check may run before it is killed; a test lowers it.
var toolTimeout = 30 * time.Second

// waitDelay bounds how long a killed command's output pipes may stay open after it exits.
const waitDelay = 5 * time.Second

// Check holds one failed preflight check and the fix the run should name.
type Check struct {
	Name string
	Fix  string
}

// Options are the flags that affect which checks run.
type Options struct {
	NoShip bool
	// Warn, when set, receives a non-blocking notice, such as no sandbox existing on native Windows.
	Warn func(string)
}

// Run executes every preflight check and returns what failed.
func Run(root string, options Options) ([]Check, error) {
	var failures []Check

	// A missing tool fails fast, with the fix named, before doctor or any check that assumes it runs.
	failures = append(failures, checkTools()...)

	// Doctor runs first.
	problems, err := doctor.Run(root, doctor.Options{NoGit: true, RepoOnly: true})
	if err != nil {
		return nil, fmt.Errorf("doctor failed: %w", err)
	}
	for _, problem := range problems {
		failures = append(failures, Check{
			Name: "doctor: " + problem.Check,
			Fix:  problem.Detail,
		})
	}

	// Host login through the contract's preflight.
	if err := checkHostLogin(root); err != nil {
		failures = append(failures, Check{
			Name: "host login",
			Fix:  err.Error(),
		})
	}

	// Forge credential is checked present, never read into a session; --no-ship skips this check.
	if !options.NoShip {
		if err := checkForgeCredential(); err != nil {
			failures = append(failures, Check{
				Name: "forge credential",
				Fix:  err.Error(),
			})
		}
	}

	// The sandbox is required on a platform that has one; native Windows has none and only warns.
	if err := checkSandbox(root, options); err != nil {
		failures = append(failures, Check{
			Name: "sandbox",
			Fix:  err.Error(),
		})
	}

	// TODO: check budget when API billing is implemented.

	return failures, nil
}

// hostFallible is a host that can report preflight failures; it's defined by tests.
var hostFallible HostContract

// HostContract is the subset of mount.Contract that preflight checks.
type HostContract interface {
	Preflight() error
}

// SetHostFallible lets tests inject a host contract; this avoids modifying the mount registry.
func SetHostFallible(h HostContract) {
	hostFallible = h
}

// checkHostLogin checks the host's login through the contract's preflight, or the profile's
// mount when no contract is under test.
func checkHostLogin(root string) error {
	if hostFallible != nil {
		return hostFallible.Preflight()
	}
	selected := profile.Select(root)
	host, ok := mount.Get(selected.Host)
	if !ok || host.LoggedIn == nil {
		return nil
	}
	// The plan probe fails for key billing and hosts with no plan, so the login has its own check.
	loggedIn, err := host.LoggedIn()
	if err != nil {
		return fmt.Errorf("could not ask %s for its login: %w", selected.Host, err)
	}
	if !loggedIn {
		return fmt.Errorf("not logged in to %s; log in and run again", selected.Host)
	}
	return nil
}

// requiredTools are the binaries the harness needs on PATH, each with the fix a missing one names.
var requiredTools = []struct{ name, fix string }{
	{"git", "git is not on PATH; install git and run again"},
	{"gh", "gh is not on PATH; install the GitHub CLI and run again"},
	{"go", "go is not on PATH; install Go and run again"},
}

// checkTools reports a failure naming the fix for every required binary missing from PATH.
func checkTools() []Check {
	var failures []Check
	for _, tool := range requiredTools {
		if _, err := exec.LookPath(tool.name); err != nil {
			failures = append(failures, Check{Name: tool.name, Fix: tool.fix})
		}
	}
	return failures
}

// checkForgeCredential reports an error if no forge credential is available; a hung gh is killed,
// process group included, once toolTimeout passes, and the error names the command and its stderr.
func checkForgeCredential() error {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "auth", "status")
	proc.Group(cmd)
	cmd.Cancel = func() error {
		proc.KillGroup(cmd)
		return nil
	}
	cmd.WaitDelay = waitDelay
	stderr := proc.NewBoundedWriter(proc.MaxOutput)
	cmd.Stderr = stderr
	err := cmd.Run()
	proc.KillGroup(cmd)
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("gh auth status: timed out after %s: %s", toolTimeout, strings.TrimSpace(stderr.String()))
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("gh auth status: no forge credential available; run `gh auth login` to authenticate with the forge: %s", detail)
	}
	return nil
}

// goos is the platform the sandbox check judges; a test swaps it.
var goos = runtime.GOOS

// procVersionPath is where the Linux check reads the kernel banner WSL names itself in; a test swaps it.
var procVersionPath = "/proc/version"

// checkSandbox reports an error when this platform has no OS sandbox or its tool is not on PATH;
// native Windows has none and runs unsandboxed instead of refusing, noted through options.Warn.
func checkSandbox(root string, options Options) error {
	switch goos {
	case "darwin":
		if _, err := exec.LookPath("sandbox-exec"); err != nil {
			return errors.New("harness sessions run sandboxed, but sandbox-exec is not on PATH")
		}
	case "linux":
		if data, err := os.ReadFile(procVersionPath); err == nil && install.OnWindowsDrive(root, string(data)) {
			return fmt.Errorf("%s is on the Windows filesystem under WSL; clone it under your Linux home, such as ~/src, and run again", root)
		}
		if _, err := exec.LookPath("bwrap"); err != nil {
			return errors.New("harness sessions run sandboxed, but bubblewrap (bwrap) is not on PATH; install it")
		}
	case "windows":
		if options.Warn != nil {
			options.Warn("native Windows has no sandbox; harness sessions run unsandboxed")
		}
	default:
		return fmt.Errorf("harness sessions run sandboxed, but %s has none; run the harness on macOS, Linux or WSL2", goos)
	}
	return nil
}
