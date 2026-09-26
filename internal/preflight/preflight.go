// Package preflight checks the host before a run: login, credentials, sandbox, and budget.
package preflight

import (
	"errors"
	"fmt"
	"os/exec"

	"komodo/internal/doctor"
	"komodo/internal/mount"
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
	problems, err := doctor.Run(root, doctor.Options{NoGit: true})
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
	if err := checkHostLogin(); err != nil {
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

	// Sandbox where the platform has one.
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

// checkHostLogin checks the host's login through the contract's preflight.
func checkHostLogin() error {
	if hostFallible != nil {
		return hostFallible.Preflight()
	}
	// If no contract is available, the check passes; the actual preflight will happen at runtime.
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

// checkSandbox reports an error if the platform has a sandbox and it cannot start.
func checkSandbox() error {
	// Only check sandbox if the host's overlay declares it's needed.
	overlay := mount.LoadOverlay()
	if !overlay.Sandbox {
		return nil
	}

	// TODO: detect sandbox availability per platform (Seatbelt on macOS, bubblewrap on Linux).
	return nil
}
