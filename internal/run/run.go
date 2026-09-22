// Package run is the headless launcher: it scrubs the environment and drives a host non-interactively.
package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/mount"
	"komodo/internal/profile"
)

// GroupBudget is how long one headless group may run before the launcher kills it.
const GroupBudget = 90 * time.Minute

// Skill is the skill a headless run always enters through.
const Skill = "run"

// dropped are the environment variables that would hand a headless run a push credential.
var dropped = []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GIT_ASKPASS", "SSH_AUTH_SOCK"}

// Options are what one headless run needs: where, what, and how long.
type Options struct {
	Root   string
	Target string
	Budget time.Duration
	DryRun bool
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
}

// Scrub returns the environment with every push credential removed and git left unable to prompt.
func Scrub(base []string) []string {
	out := make([]string, 0, len(base)+6)
	for _, entry := range base {
		key, _, found := strings.Cut(entry, "=")
		if !found || contains(dropped, key) || isOverride(key) {
			continue
		}
		out = append(out, entry)
	}
	return append(out,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=credential.helper",
		"GIT_CONFIG_VALUE_0=",
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes -o IdentitiesOnly=yes -o IdentityFile="+os.DevNull,
		"GH_CONFIG_DIR="+filepath.Join(os.TempDir(), "komodo-gh-noauth"),
	)
}

// isOverride reports whether Scrub sets this key itself, so an inherited value never survives.
func isOverride(key string) bool {
	switch key {
	case "GIT_TERMINAL_PROMPT", "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0",
		"GIT_CONFIG_VALUE_0", "GIT_SSH_COMMAND", "GH_CONFIG_DIR":
		return true
	}
	return false
}

// Command resolves the host from the profile and returns what a headless run would invoke.
func Command(root, target string) (string, []string, error) {
	selected := profile.Select(root)
	host, ok := mount.Get(selected.Host)
	if !ok || host.Headless == nil {
		return "", nil, errors.New("no mount is installed here; run komodo install")
	}
	name, args := host.Headless(Skill, target)
	return name, args, nil
}

// Launch drives the host non-interactively and returns its exit code.
func Launch(options Options) (int, error) {
	name, args, err := Command(options.Root, options.Target)
	if err != nil {
		return 1, err
	}
	return launch(options, name, args)
}

// launch runs one resolved command under the budget in a scrubbed environment.
func launch(options Options, name string, args []string) (int, error) {
	stdout, stderr := options.Stdout, options.Stderr
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	if options.DryRun {
		fmt.Fprintf(stdout, "%s %s\n", name, strings.Join(args, " "))
		return 0, nil
	}
	budget := options.Budget
	if budget <= 0 {
		budget = GroupBudget
	}
	base := options.Env
	if base == nil {
		base = os.Environ()
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = options.Root
	command.Env = Scrub(base)
	command.Stdout, command.Stderr = stdout, stderr
	err := command.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return 124, fmt.Errorf("the run passed its %s budget and was killed", budget)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	if err != nil {
		return 1, err
	}
	return 0, nil
}

// contains reports whether the slice already holds the value.
func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
