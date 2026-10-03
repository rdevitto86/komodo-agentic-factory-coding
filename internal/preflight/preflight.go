// Package preflight checks the host before a run: login, credentials, sandbox, and budget.
package preflight

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"komodo/internal/doctor"
	"komodo/internal/mount"
	"komodo/internal/profile"
)

// Check holds one failed preflight check and the fix the run should name.
type Check struct {
	Name string
	Fix  string
}

// Options are the flags that affect which checks run.
type Options struct {
	NoShip bool
}

// Run executes every preflight check and returns what failed.
func Run(root string, options Options) ([]Check, error) {
	var failures []Check

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

	// The sandbox is required; a platform without one refuses the run.
	if err := checkSandbox(); err != nil {
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

// checkForgeCredential reports an error if no forge credential is available.
func checkForgeCredential() error {
	// Check if gh is available and authenticated.
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return errors.New("no forge credential available; run `gh auth login` to authenticate with the forge")
	}
	return nil
}

// goos is the platform the sandbox check judges; a test swaps it.
var goos = runtime.GOOS

// checkSandbox reports an error when this platform has no OS sandbox or its tool is not on PATH,
// since every line session runs sandboxed and the line refuses to run without it.
func checkSandbox() error {
	switch goos {
	case "darwin":
		if _, err := exec.LookPath("sandbox-exec"); err != nil {
			return errors.New("line sessions run sandboxed, but sandbox-exec is not on PATH")
		}
	case "linux":
		if _, err := exec.LookPath("bwrap"); err != nil {
			return errors.New("line sessions run sandboxed, but bubblewrap (bwrap) is not on PATH; install it")
		}
	default:
		return fmt.Errorf("line sessions run sandboxed, but %s has none; run the line on macOS, Linux or WSL2", goos)
	}
	return nil
}
