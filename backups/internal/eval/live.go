package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"komodo/internal/mount"
)

// canaryCase is the case that needs a personal host instructions file to plant its canary in.
const canaryCase = "canary"

// LiveCases runs each case against its own fresh live clone of the suite's first group under work, and
// fails when any case fails; with no instructions file, it skips the canary case with a note.
func LiveCases(ctx context.Context, options CaseOptions, live LiveOptions, work string) error {
	if len(live.Suite.Groups) == 0 {
		return errors.New("the suite holds no group for the eval cases to clone")
	}
	if options.Stdout == nil {
		options.Stdout = io.Discard
	}
	live.Group = live.Suite.Groups[0]
	cases := make([]Case, 0, len(options.Cases))
	for _, each := range options.Cases {
		if each.Name == canaryCase && live.Instructions == "" {
			fmt.Fprintf(options.Stdout, "case %s (%s): skipped: no personal host instructions file to plant it in\n",
				each.Name, each.Requirement)
			continue
		}
		cases = append(cases, each)
	}
	options.Cases = cases
	options.Env = func(ctx context.Context, index int) (Env, error) {
		each := live
		each.Dir = filepath.Join(work, fmt.Sprintf("case-%d", index))
		return NewLive(ctx, each)
	}
	outcomes, err := RunCases(ctx, options)
	if err != nil {
		return err
	}
	failed := 0
	for _, outcome := range outcomes {
		if !outcome.Passed() {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d eval cases failed", failed, len(outcomes))
	}
	return nil
}

// LiveOptions are what a live env needs: the golden group it clones, the komodo it mounts, and where it goes.
type LiveOptions struct {
	Suite      Suite
	Group      Group
	Executable string
	// Host is the mount the clone installs, the default mount when empty.
	Host string
	// Instructions is this machine's personal host instructions file, where the canary is planted.
	Instructions string
	Dir          string
	Clone        Clone
}

// Live is a fresh clone of one golden group on this machine, mounted and committed on main with hooks off.
type Live struct {
	dir          string
	group        string
	executable   string
	instructions string
}

// NewLive clones the group's repo at its pinned commit, turns its hooks off, places the group, mounts the
// host, and commits and pushes both to the clone's local origin.
func NewLive(ctx context.Context, options LiveOptions) (*Live, error) {
	if options.Clone == nil {
		options.Clone = CloneAt
	}
	repo := options.Suite.Repo(options.Group)
	if err := options.Clone(ctx, repo.URL, options.Group.Commit, options.Dir); err != nil {
		return nil, fmt.Errorf("clone: %w", err)
	}
	if err := command(ctx, options.Dir, cloneTimeout, "git", "config", "core.hooksPath", os.DevNull); err != nil {
		return nil, err
	}
	if err := appendGroup(options.Dir, filepath.Join(options.Suite.Dir, options.Group.File)); err != nil {
		return nil, err
	}
	install := []string{"install"}
	if options.Host != "" {
		install = append(install, "--host", options.Host)
	}
	if err := command(ctx, options.Dir, 0, options.Executable, install...); err != nil {
		return nil, err
	}
	live := &Live{
		dir: options.Dir, group: options.Group.ID, executable: options.Executable, instructions: options.Instructions,
	}
	if err := live.commit(ctx, "eval: "+options.Group.ID); err != nil {
		return nil, err
	}
	return live, nil
}

// Dir is the clone's root.
func (l *Live) Dir() string { return l.dir }

// Group is the golden group the clone holds ready.
func (l *Live) Group() string { return l.group }

// Komodo runs the mounted komodo in the clone with extra appended to this process's environment.
func (l *Live) Komodo(ctx context.Context, extra []string, args ...string) Ran {
	return invoke(ctx, l.dir, extra, l.executable, args...)
}

// Git runs git in the clone as the eval's owner identity.
func (l *Live) Git(ctx context.Context, args ...string) Ran {
	return invoke(ctx, l.dir, nil, "git", append(append([]string{}, evalIdentity...), args...)...)
}

// AddGroup appends a group's section to the clone's backlog and commits and pushes it on main before ctx ends.
func (l *Live) AddGroup(ctx context.Context, id, body string) error {
	scratch, err := l.Scratch(ctx)
	if err != nil {
		return err
	}
	section := filepath.Join(scratch, id+".md")
	if err := os.WriteFile(section, []byte(body), 0o644); err != nil {
		return err
	}
	if err := appendGroup(l.dir, section); err != nil {
		return err
	}
	return l.commit(ctx, "eval: "+id)
}

// commit stages everything in the clone, commits it with message, and pushes main to the local origin.
func (l *Live) commit(ctx context.Context, message string) error {
	steps := [][]string{
		{"add", "-A"},
		append(append([]string{}, evalIdentity...), "commit", "-q", "-m", message),
		{"push", "-q", "origin", "main"},
	}
	for _, args := range steps {
		if err := command(ctx, l.dir, cloneTimeout, "git", args...); err != nil {
			return err
		}
	}
	return nil
}

// Scratch makes a new empty directory beside the clone.
func (l *Live) Scratch(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return os.MkdirTemp(filepath.Dir(l.dir), filepath.Base(l.dir)+"-scratch-")
}

// PathWithout keeps each PATH directory holding none of names and swaps each other one for a mirror of its
// entries less names.
func (l *Live) PathWithout(names ...string) (string, error) {
	var kept []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		holds := false
		for _, name := range names {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				holds = true
			}
		}
		if !holds {
			kept = append(kept, dir)
			continue
		}
		mirrored, err := l.Scratch(context.Background())
		if err != nil {
			return "", err
		}
		if err := mirror(dir, mirrored, names...); err != nil {
			return "", err
		}
		kept = append(kept, mirrored)
	}
	return strings.Join(kept, string(os.PathListSeparator)), nil
}

// Credential writes the forge token gh holds into a scratch gh config only the run reads; remove deletes it.
func (l *Live) Credential(ctx context.Context) ([]string, func() error, error) {
	ctx, cancel := context.WithTimeout(ctx, cloneTimeout)
	defer cancel()
	token := invoke(ctx, l.dir, nil, "gh", "auth", "token")
	if token.Code != 0 || strings.TrimSpace(token.Output) == "" {
		return nil, nil, fmt.Errorf("gh auth token gave no forge credential to hand the run: %s", token.Output)
	}
	config, err := l.Scratch(ctx)
	if err != nil {
		return nil, nil, err
	}
	hosts := "github.com:\n    oauth_token: " + strings.TrimSpace(token.Output) + "\n    git_protocol: https\n"
	if err := os.WriteFile(filepath.Join(config, "hosts.yml"), []byte(hosts), 0o600); err != nil {
		return nil, nil, err
	}
	extra := []string{"GH_TOKEN=", "GITHUB_TOKEN=", "GH_CONFIG_DIR=" + config}
	return extra, func() error { return os.RemoveAll(config) }, nil
}

// Overlay mirrors this user's home into a scratch one whose komodo overlay is the real one with body's
// keys laid over it, so the host's own login stays.
func (l *Live) Overlay(ctx context.Context, body string) ([]string, error) {
	given := map[string]any{}
	if err := json.Unmarshal([]byte(body), &given); err != nil {
		return nil, fmt.Errorf("the overlay: %w", err)
	}
	own, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	overlay := mount.OverlayPath()
	rel, err := filepath.Rel(own, overlay)
	if err != nil {
		return nil, err
	}
	merged := map[string]any{}
	if data, err := os.ReadFile(overlay); err == nil {
		if err := json.Unmarshal(data, &merged); err != nil {
			return nil, fmt.Errorf("%s: %w", overlay, err)
		}
	}
	maps.Copy(merged, given)
	home, err := l.Scratch(ctx)
	if err != nil {
		return nil, err
	}
	if err := mirror(own, home, strings.Split(filepath.ToSlash(rel), "/")[0]); err != nil {
		return nil, err
	}
	target := filepath.Join(home, rel)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, err
	}
	if err := mirror(filepath.Dir(overlay), filepath.Dir(target), filepath.Base(overlay)); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(target, append(data, '\n'), 0o644); err != nil {
		return nil, err
	}
	return []string{"HOME=" + home, "USERPROFILE=" + home}, nil
}

// Plant appends text to the personal host instructions file; restore puts back what it held, or removes it.
func (l *Live) Plant(ctx context.Context, text string) (func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path := l.instructions
	if path == "" {
		return nil, errors.New("no personal host instructions file to plant the canary in")
	}
	was, err := os.ReadFile(path)
	existed := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	planted := append(append([]byte{}, was...), []byte("\n"+text+"\n")...)
	if err := os.WriteFile(path, planted, 0o644); err != nil {
		return nil, err
	}
	return func() error {
		if !existed {
			return os.Remove(path)
		}
		return os.WriteFile(path, was, 0o644)
	}, nil
}

// mirror links every entry of source into target except skip; a missing source links nothing.
func mirror(source, target string, skip ...string) error {
	entries, err := os.ReadDir(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if slices.Contains(skip, entry.Name()) {
			continue
		}
		if err := os.Symlink(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// invoke runs one program in dir with extra appended to this process's environment, killing its process
// tree when ctx ends, and returns what it printed and its exit code, -1 when it never exited.
func invoke(ctx context.Context, dir string, extra []string, name string, args ...string) Ran {
	output, err := runProcess(ctx, dir, extra, name, args...)
	var exit *exec.ExitError
	switch {
	case err == nil:
		return Ran{Output: output}
	case errors.As(err, &exit) && exit.ExitCode() >= 0:
		return Ran{Output: output, Code: exit.ExitCode()}
	}
	return Ran{Output: output + err.Error(), Code: -1}
}
