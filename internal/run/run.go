// Package run is the headless launcher: it scrubs the environment and drives a host non-interactively.
package run

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/doctor"
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
	Root   string
	Target string
	Budget time.Duration
	DryRun bool
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
	PR     *pr.Client
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
	for {
		if err := refreshRoot(options.Root); err != nil {
			return 1, err
		}
		plan, err := line.PlanForStation(options.Root, "")
		if err != nil {
			return 1, err
		}
		if plan == nil {
			fmt.Fprintln(stdout, "drain done: nothing is ready")
			return 0, nil
		}
		if ran[plan.Group] {
			fmt.Fprintf(stdout, "%s stopped: it came up again after it shipped\n", plan.Group)
			return 1, nil
		}
		remaining := total - time.Since(started)
		if remaining <= 0 {
			fmt.Fprintf(stdout, "%s stopped: the whole %s budget is spent\n", plan.Group, total)
			return 124, nil
		}
		group := options
		group.Target = plan.Group
		group.Budget = min(GroupBudget, remaining)
		launched := time.Now()
		code, url, err := launchTarget(group)
		if err != nil {
			fmt.Fprintf(stdout, "%s stopped: %v\n", plan.Group, err)
			return max(code, 1), err
		}
		if !shipped(options.Root, plan.Group, launched) {
			fmt.Fprintf(stdout, "%s stopped: it ended without shipping (exit %d)\n", plan.Group, code)
			return max(code, 1), nil
		}
		if url == "" {
			url = "no pull request was handed off"
		}
		fmt.Fprintf(stdout, "%s shipped: %s\n", plan.Group, url)
		ran[plan.Group] = true
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

// drainOrder is the groups a drain plans to run, in order, the open run first.
func drainOrder(root string) ([]string, error) {
	var order []string
	if state, err := line.LoadRun(root); err == nil && line.RunIsOpen(root) {
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

// refreshRoot re-renders every installed host's project config at the root when the doctor reports drift.
func refreshRoot(root string) error {
	problems, err := doctor.Run(root, doctor.Options{NoGit: true})
	if err != nil {
		return err
	}
	for _, problem := range problems {
		if problem.Check == "drift" {
			return line.RenderProject(root, root)
		}
	}
	return nil
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
	command.Env = withBinPath(Scrub(base), options.Root)
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

// withBinPath puts the repo's own bin/ first on PATH, so the host finds komodo where the gate built it.
func withBinPath(env []string, root string) []string {
	bin := filepath.Join(root, "bin")
	out := make([]string, 0, len(env)+1)
	found := false
	for _, entry := range env {
		if key, value, ok := strings.Cut(entry, "="); ok && key == "PATH" {
			entry = "PATH=" + bin + string(os.PathListSeparator) + value
			found = true
		}
		out = append(out, entry)
	}
	if !found {
		out = append(out, "PATH="+bin)
	}
	return out
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

// finishShip pushes and opens the pull request a scrubbed ship handed off, in the launcher's own
// credentialed environment, stamps ship done, removes the handoff, and returns the pull request.
func finishShip(options Options) (string, error) {
	path := filepath.Join(options.Root, line.StateDir, "ship.json")
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
	if known, err := client.Labels(); err == nil {
		_ = client.Label(url, pr.KeepKnown(handoff.Labels, known))
	}
	worktree := handoff.Worktree
	if worktree == "" {
		worktree = options.Root
	}
	if _, err := line.FileFindings(worktree, handoff.Group, handoff.Minor); err != nil {
		return url, err
	}
	if handoff.AfterPublish != "" {
		if published := line.RunCommand(worktree, handoff.AfterPublish); !published.OK() {
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
	if err := exec.Command("git", "-C", root, "check-ref-format", "--branch", branch).Run(); err != nil {
		return fmt.Errorf("ship.json names %q, which is not a valid branch; nothing was pushed", branch)
	}
	if guard.Load(root, root).IsCritical(branch) {
		return fmt.Errorf("ship.json names the critical ref %q; landing is the human's merge button", branch)
	}
	return nil
}

// gitPush pushes one branch to origin from the run's root, in the launcher's ambient environment.
func gitPush(root, branch string) error {
	cmd := exec.Command("git", "push", "-u", "origin", "refs/heads/"+branch+":refs/heads/"+branch)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git push: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
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
