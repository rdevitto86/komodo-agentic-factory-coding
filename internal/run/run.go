// Package run is the headless launcher: it scrubs the environment and drives a host non-interactively.
package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"komodo/internal/git"
	"komodo/internal/guard"
	"komodo/internal/ledger"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/pr"
	"komodo/internal/proc"
	"komodo/internal/profile"
)

// GroupBudget is how long one headless group may run before the launcher kills it.
const GroupBudget = 90 * time.Minute

// Skill is the skill a headless run always enters through.
const Skill = "run"

// dropped are the environment variables that would hand a headless run a push credential.
var dropped = []string{
	"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GIT_ASKPASS", "SSH_AUTH_SOCK",
	"GIT_CONFIG_PARAMETERS",
}

// Options are what one headless run needs: where, what, and how long.
type Options struct {
	Root       string
	Target     string
	Budget     time.Duration
	DryRun     bool
	Env        []string
	Stdout     io.Writer
	Stderr     io.Writer
	PR         *pr.Client
	Executable string
}

// Scrub returns the environment with every push credential removed and git left unable to prompt.
func Scrub(base []string) []string {
	out := make([]string, 0, len(base)+6)
	for _, entry := range base {
		key, _, found := strings.Cut(entry, "=")
		if !found || contains(dropped, key) || credentialShaped(key) || isOverride(key) {
			continue
		}
		out = append(out, entry)
	}
	return append(out,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=credential.helper",
		"GIT_CONFIG_VALUE_0=",
		"GIT_SSH_COMMAND=ssh -F "+os.DevNull+" -o BatchMode=yes -o IdentitiesOnly=yes -o IdentityFile="+os.DevNull,
		"GH_CONFIG_DIR="+filepath.Join(os.TempDir(), "komodo-gh-noauth"),
	)
}

// credentialShaped reports whether a key names a forge's secret, such as GITHUB_PAT or GITLAB_TOKEN,
// so a push credential is dropped while the model host keeps its own login.
func credentialShaped(key string) bool {
	forge, secret := false, false
	for _, part := range strings.Split(key, "_") {
		forge = forge || forgeWords[part]
		secret = secret || secretWords[part]
	}
	return forge && secret
}

// forgeWords and secretWords are the name parts that together mark a push credential.
var (
	forgeWords  = map[string]bool{"GIT": true, "GITHUB": true, "GH": true, "GITLAB": true, "GL": true, "BITBUCKET": true}
	secretWords = map[string]bool{"TOKEN": true, "PAT": true, "SECRET": true, "PASSWORD": true, "KEY": true}
)

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

// Launch drives the host non-interactively on one target and returns its exit code; with no
// target it drains every ready group in order.
func Launch(options Options) (int, error) {
	if options.Target == "" {
		return drain(options)
	}
	code, _, err := launchTarget(options)
	return code, err
}

// launchTarget runs the host on one target, then finishes any push a scrubbed ship handed off,
// returning the host's exit code and the pull request it opened.
func launchTarget(options Options) (int, string, error) {
	name, args, err := Command(options.Root, options.Target)
	if err != nil {
		return 1, "", err
	}
	code, err := launch(options, name, args)
	url := ""
	if !options.DryRun {
		created, shipErr := finishShip(options)
		url = created
		if shipErr != nil && err == nil {
			err = shipErr
		}
	}
	return code, url, err
}

// drain launches the next ready group, one at a time, until nothing is ready, a group ends
// unshipped, or the whole budget is spent, printing one line per group.
func drain(options Options) (int, error) {
	stdout := options.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	if options.DryRun {
		return 0, listDrain(options.Root, stdout)
	}
	total, err := drainBudget(options.Root, options.Budget)
	if err != nil {
		return 1, err
	}
	started := time.Now()
	ran := map[string]bool{}
	executable := ""
	for {
		// Sync fetches origin, rebuilds a stale binary, and re-renders drifted config.
		built, err := Sync(SyncOptions{Root: options.Root, Stdout: options.Stdout})
		if err != nil {
			return 1, err
		}
		if built != "" {
			executable = built
		}

		// Every open group drains first, oldest start first, then each ready group in file order.
		order, err := drainOrder(options.Root)
		if err != nil {
			return 1, err
		}
		if len(order) == 0 {
			fmt.Fprintln(stdout, "drain done: nothing is ready")
			return 0, nil
		}
		next := order[0]
		if ran[next] {
			fmt.Fprintf(stdout, "%s stopped: it came up again after it shipped\n", next)
			return 1, nil
		}
		remaining := total - time.Since(started)
		if remaining <= 0 {
			fmt.Fprintf(stdout, "%s stopped: the whole %s budget is spent\n", next, total)
			return 124, nil
		}
		group := options
		group.Target = next
		group.Budget = min(GroupBudget, remaining)
		group.Executable = executable
		launched := time.Now()
		code, url, err := launchTarget(group)
		if err != nil {
			fmt.Fprintf(stdout, "%s stopped: %v\n", next, err)
			return max(code, 1), err
		}
		if !shipped(options.Root, next, launched) {
			fmt.Fprintf(stdout, "%s stopped: it ended without shipping (exit %d)\n", next, code)
			return max(code, 1), nil
		}
		if url == "" {
			url = "no pull request was handed off"
		}
		fmt.Fprintf(stdout, "%s shipped: %s\n", next, url)
		ran[next] = true
	}
}

// listDrain prints the groups a drain would run, in order, the open run first, and launches nothing.
func listDrain(root string, stdout io.Writer) error {
	order, err := drainOrder(root)
	if err != nil {
		return err
	}
	if len(order) == 0 {
		fmt.Fprintln(stdout, "drain would run nothing: nothing is ready")
	}
	for index, group := range order {
		fmt.Fprintf(stdout, "%d. %s\n", index+1, group)
	}
	return nil
}

// drainBudget is the whole drain's budget: the one given, else GroupBudget for every group the drain plans.
func drainBudget(root string, given time.Duration) (time.Duration, error) {
	if given > 0 {
		return given, nil
	}
	order, err := drainOrder(root)
	if err != nil {
		return 0, err
	}
	return GroupBudget * time.Duration(max(len(order), 1)), nil
}

// drainOrder is the groups a drain plans to run, in order: every open run first, then each ready group.
func drainOrder(root string) ([]string, error) {
	var order []string
	for _, state := range line.OpenRuns(root) {
		order = append(order, state.Group)
	}
	groups, err := line.ReadyGroups(root)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		if !contains(order, group.ID) {
			order = append(order, group.ID)
		}
	}
	return order, nil
}

// shipped reports whether the ledger records a finished ship for the group at or after since.
func shipped(root, group string, since time.Time) bool {
	entries, err := line.Book(root).All()
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Station == "ship" && entry.Group == group && entry.Outcome == "done" && !entry.At.Before(since) {
			return true
		}
	}
	return false
}

// launch runs one resolved command under the budget, in its own process group, in a scrubbed
// environment, teeing its stdout to the host's own usage events file.
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
	executable := options.Executable
	if executable == "" {
		var err error
		executable, err = mount.Executable()
		if err != nil {
			return 1, fmt.Errorf("cannot find the running komodo binary: %w", err)
		}
	}
	env, err := withBinPath(Scrub(base), options.Root, executable)
	if err != nil {
		return 1, err
	}
	command.Env = env
	proc.Group(command)
	command.Cancel = func() error {
		proc.KillGroup(command)
		return nil
	}
	command.WaitDelay = 5 * time.Second
	out := stdout
	if events, err := eventsFile(options); err == nil && events != nil {
		defer events.Close()
		out = io.MultiWriter(stdout, events)
	}
	command.Stdout, command.Stderr = out, stderr
	err = command.Run()
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

// symlink links a path to a target; a test swaps it to act like Windows without Developer Mode.
var symlink = os.Symlink

// withBinPath puts komodo on the run's PATH as root/.komodo/bin, then the inherited PATH, then the repo's own root/bin.
func withBinPath(env []string, root, executable string) ([]string, error) {
	link := filepath.Join(root, line.StateDir, "bin")
	name := "komodo"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.MkdirAll(link, 0o755); err != nil {
		return nil, err
	}
	target := filepath.Join(link, name)
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	first := link
	if err := symlink(executable, target); err != nil {
		if err := copyExecutable(executable, target); err != nil {
			first = filepath.Dir(executable)
		}
	}
	sep := string(os.PathListSeparator)
	out := make([]string, 0, len(env)+1)
	found := false
	for _, entry := range env {
		if key, value, ok := strings.Cut(entry, "="); ok && key == "PATH" {
			entry = "PATH=" + first + sep + value + sep + filepath.Join(root, "bin")
			found = true
		}
		out = append(out, entry)
	}
	if !found {
		out = append(out, "PATH="+first+sep+filepath.Join(root, "bin"))
	}
	return out, nil
}

// copyExecutable copies the running binary to target, removing a partial copy when it fails.
func copyExecutable(executable, target string) error {
	data, err := os.ReadFile(executable)
	if err != nil {
		return err
	}
	if err := os.WriteFile(target, data, 0o755); err != nil {
		_ = os.Remove(target)
		return err
	}
	return nil
}

// eventsFile opens the events file the installed mount names for one target, or returns
// nothing when there is no target or the mount reads no events.
func eventsFile(options Options) (*os.File, error) {
	if options.Target == "" {
		return nil, nil
	}
	host, ok := mount.Get(profile.Select(options.Root).Host)
	if !ok || host.EventsPath == nil {
		return nil, nil
	}
	path := host.EventsPath(options.Root, options.Target)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.Create(path)
}

// finishShip pushes and opens the pull request the target's group handed off from a scrubbed ship,
// in the launcher's own credentialed environment, stamps ship done, removes the handoff, and returns the pull request.
func finishShip(options Options) (string, error) {
	group := line.GroupFor(options.Root, options.Target)
	if group == "" {
		return "", nil
	}
	path := line.HandoffPath(options.Root, group)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var handoff line.ShipHandoff
	if err := json.Unmarshal(data, &handoff); err != nil {
		return "", err
	}
	if handoff.Group != group {
		return "", fmt.Errorf("the handoff under %s names group %q; nothing was pushed", group, handoff.Group)
	}
	if err := pushable(options.Root, handoff.Branch); err != nil {
		return "", err
	}
	if err := gitPush(options.Root, handoff.Branch); err != nil {
		return "", err
	}
	client := options.PR
	if client == nil {
		client = pr.New(options.Root)
	}
	url, err := client.Create(handoff.Base, handoff.Branch, handoff.Title, handoff.Body, handoff.Draft)
	if err != nil {
		return "", err
	}
	_, warnings := line.ApplyLabels(client, url, handoff.Labels)
	if options.Stderr != nil {
		for _, warning := range warnings {
			fmt.Fprintf(options.Stderr, "ship: %s\n", warning)
		}
	}
	worktree := handoff.Worktree
	if worktree == "" {
		worktree = options.Root
	}
	// An agent can write after_publish into ship.json, so it runs scrubbed, never with the push credentials.
	if handoff.AfterPublish != "" {
		if published := line.RunCommandEnv(worktree, handoff.AfterPublish, Scrub(os.Environ())); !published.OK() {
			return url, fmt.Errorf("after_publish: %s", line.FailureText(published))
		}
	}
	line.Stamp(options.Root, ledger.Entry{Group: handoff.Group, Station: "ship", Outcome: "done"})
	return url, os.Remove(path)
}

// pushable refuses a handoff branch that is a refspec, an option, an invalid name, or a critical ref,
// since an agent can write ship.json and the launcher pushes with real credentials.
func pushable(root, branch string) error {
	if branch == "" || strings.HasPrefix(branch, "-") || strings.ContainsAny(branch, ":+ ") {
		return fmt.Errorf("ship.json names %q, which is not a plain branch; nothing was pushed", branch)
	}
	if _, err := git.Run(root, "check-ref-format", "--branch", branch); err != nil {
		return fmt.Errorf("ship.json names %q, which is not a valid branch; nothing was pushed", branch)
	}
	if guard.Load(root, root).IsCritical(branch) {
		return fmt.Errorf("ship.json names the critical ref %q; landing is the human's merge button", branch)
	}
	return nil
}

// gitPush pushes one branch to origin from the run's root, in the launcher's ambient environment.
func gitPush(root, branch string) error {
	_, err := git.Run(root, "push", "-u", "origin", "refs/heads/"+branch+":refs/heads/"+branch)
	return err
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
