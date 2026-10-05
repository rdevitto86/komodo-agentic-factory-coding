// Package install renders a host's project or user-level configuration as a plan of file changes.
package install

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"komodo/internal/changelog"
	"komodo/internal/fsx"
	"komodo/internal/git"
)

// Change is one file the install writes, seeds, or removes.
type Change struct {
	Path    string
	Body    []byte
	Mode    fs.FileMode
	Seed    bool
	Remove  bool
	Project bool
	// Scoped marks a project file one role's session loads alone, never the primary session.
	Scoped bool
	Why    string
}

// Plan is every change one host's install would make.
type Plan struct {
	Host    string
	Root    string
	Changes []Change
	// Marker is the file whose presence means this plan's layer is already installed, or "" for a repo plan.
	Marker string
	// Fix is the command that reapplies the plan, named in a drift report.
	Fix string
}

// Installed reports whether the plan's layer was installed before: always for a repo plan, else when its marker exists.
func (p Plan) Installed() bool {
	if p.Marker == "" {
		return true
	}
	_, err := os.Stat(p.Marker)
	return err == nil
}

// Add appends one rendered file to the plan.
func (p *Plan) Add(path string, body []byte, why string) {
	p.Changes = append(p.Changes, Change{Path: path, Body: body, Mode: 0o644, Why: why})
}

// AddProject appends one rendered file that also belongs to the project-only render: the repo
// skills, the standards skills, and the rules file, which the project render writes on their own.
func (p *Plan) AddProject(path string, body []byte, why string) {
	p.Changes = append(p.Changes, Change{Path: path, Body: body, Mode: 0o644, Project: true, Why: why})
}

// AddScoped adds a project file that only one role's session loads, such as that role's plugin.
func (p *Plan) AddScoped(path string, body []byte, why string) {
	p.Changes = append(p.Changes, Change{Path: path, Body: body, Mode: 0o644, Project: true, Scoped: true, Why: why})
}

// Project narrows the plan to the project render's changes, gitignored copies rebuilt every time.
func (p Plan) Project() Plan {
	narrowed := Plan{Host: p.Host, Root: p.Root}
	for _, change := range p.Changes {
		if change.Project {
			narrowed.Changes = append(narrowed.Changes, change)
		}
	}
	return narrowed
}

// AddSeed appends a file written once and never overwritten.
func (p *Plan) AddSeed(path string, body []byte, why string) {
	p.Changes = append(p.Changes, Change{Path: path, Body: body, Mode: 0o644, Seed: true, Why: why})
}

// AddIgnore appends entry to the root's .gitignore when no line already names it, keeping every existing line.
func (p *Plan) AddIgnore(entry, why string) {
	path := filepath.Join(p.Root, ".gitignore")
	existing, _ := os.ReadFile(path)
	pending := -1
	for index, change := range p.Changes {
		if change.Path == path && !change.Remove {
			pending, existing = index, change.Body
		}
	}
	bare := strings.Trim(entry, "/")
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.Trim(strings.TrimSpace(line), "/") == bare {
			return
		}
	}
	newline := "\n"
	if bytes.Contains(existing, []byte("\r\n")) {
		newline = "\r\n"
	}
	body := append([]byte{}, existing...)
	if len(body) > 0 && body[len(body)-1] != '\n' {
		body = append(body, newline...)
	}
	body = append(body, []byte(entry+newline)...)
	if pending >= 0 {
		p.Changes[pending].Body = body
		return
	}
	p.Add(path, body, why)
}

// installerDefault matches the KOMODO_VERSION fallback in install.sh's ${…:-v…} or install.ps1's else { 'v…' }.
var installerDefault = regexp.MustCompile(`(?:KOMODO_VERSION:-|else \{ ')(v` + changelog.SemVer + `)`)

// defaultVersionProblem names an installer whose default KOMODO_VERSION is not v plus latest, or "" when it matches.
func defaultVersionProblem(rel, body, latest string) string {
	match := installerDefault.FindStringSubmatch(body)
	if match == nil {
		return fmt.Sprintf("%s: names no default KOMODO_VERSION; want v%s, the newest changelog version", rel, latest)
	}
	if match[1] != "v"+latest {
		return fmt.Sprintf("%s: defaults KOMODO_VERSION to %s; want v%s, the newest changelog version", rel, match[1], latest)
	}
	return ""
}

// hookCommand matches a JSON "command" string that runs some binary's guard subcommand.
var hookCommand = regexp.MustCompile(`"command"\s*:\s*"((?:[^"\\]|\\.)*) guard"`)

// HookBinaries lists every binary path a rendered or installed file runs as the guard hook.
func HookBinaries(body []byte) []string {
	var out []string
	for _, match := range hookCommand.FindAllSubmatch(body, -1) {
		if path, err := strconv.Unquote(`"` + string(match[1]) + `"`); err == nil {
			out = append(out, path)
		}
	}
	return out
}

// komodoCommand matches a JSON "command" string that runs some binary's guard or hook subcommand, with its arguments.
var komodoCommand = regexp.MustCompile(`"command"\s*:\s*"((?:[^"\\]|\\.)*?) ((?:guard|hook)(?: (?:[^"\\]|\\.)*)?)"`)

// komodoBinary reports whether a path's file name is some komodo binary.
func komodoBinary(path string) bool {
	return strings.HasPrefix(path[strings.LastIndexAny(path, `/\`)+1:], "komodo")
}

// KomodoHook reports whether a hook command runs some komodo binary's guard or hook subcommand.
func KomodoHook(command string) bool {
	parts := komodoCommand.FindStringSubmatch(`"command": ` + strconv.Quote(command))
	if parts == nil {
		return false
	}
	path, err := strconv.Unquote(`"` + parts[1] + `"`)
	return err == nil && komodoBinary(path)
}

// GlobalRender builds one host's user-level plan under home from the toolkit root serves, running binary as its hooks.
type GlobalRender func(root, home, binary string) (Plan, error)

var (
	globalLock sync.Mutex
	globals    = map[string]GlobalRender{}
)

// RegisterGlobal records how one host renders its user-level config, keyed by host name.
func RegisterGlobal(host string, render GlobalRender) {
	globalLock.Lock()
	defer globalLock.Unlock()
	globals[host] = render
}

// Global returns the user-level render one host registered.
func Global(host string) (GlobalRender, bool) {
	globalLock.Lock()
	defer globalLock.Unlock()
	render, ok := globals[host]
	return render, ok
}

// GlobalPlan renders one host's user-level plan into the current user's home directory.
func GlobalPlan(host, root, binary string) (Plan, error) {
	render, ok := Global(host)
	if !ok {
		return Plan{}, fmt.Errorf("host %q has no user-level config to install", host)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Plan{}, err
	}
	return render(root, home, binary)
}

// AddRemoval appends a path the install deletes when it is present.
func (p *Plan) AddRemoval(path, why string) {
	p.Changes = append(p.Changes, Change{Path: path, Remove: true, Why: why})
}

// Action is what one change would do to the tree as it stands.
type Action struct {
	Verb string
	Path string
	Why  string
	Seed bool
	// Tracked marks a kept file git tracks whose content differs from what the plan would render.
	Tracked bool
}

// Actions describes the plan against the current tree without touching it.
func (p Plan) Actions() []Action {
	return p.actions(func(body []byte) []byte { return body })
}

// actions compares each change to the tree after passing both sides through normalise.
func (p Plan) actions(normalise func([]byte) []byte) []Action {
	tracked := trackedFiles(p.Root)
	var out []Action
	for _, change := range p.Changes {
		relative := change.Path
		if rel, err := filepath.Rel(p.Root, change.Path); err == nil && !strings.HasPrefix(rel, "..") {
			relative = rel
		}
		if change.Remove {
			if _, err := os.Stat(change.Path); err == nil {
				out = append(out, Action{Verb: "remove", Path: relative, Why: change.Why})
			}
			continue
		}
		existing, err := os.ReadFile(change.Path)
		switch {
		case change.Seed && err == nil:
			out = append(out, Action{Verb: "keep", Path: relative, Why: "seeded once, never overwritten", Seed: true})
		case err == nil && tracked[relative]:
			if bytes.Equal(normalise(existing), normalise(change.Body)) {
				out = append(out, Action{Verb: "keep", Path: relative, Why: "git tracks this file; install leaves it alone"})
				continue
			}
			out = append(out, Action{Verb: "keep", Path: relative, Tracked: true,
				Why: "git tracks this file and its content differs; install leaves it alone"})
		case err != nil:
			out = append(out, Action{Verb: "create", Path: relative, Why: change.Why, Seed: change.Seed})
		case !bytes.Equal(normalise(existing), normalise(change.Body)):
			out = append(out, Action{Verb: "update", Path: relative, Why: change.Why})
		default:
			out = append(out, Action{Verb: "same", Path: relative, Why: change.Why})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// trackedFiles lists the paths root's git index tracks, relative to root, or nil outside a git repo.
func trackedFiles(root string) map[string]bool {
	out, err := git.Run(root, "ls-files")
	if err != nil {
		return nil
	}
	tracked := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if line != "" {
			tracked[filepath.FromSlash(line)] = true
		}
	}
	return tracked
}

// Apply writes the plan and returns what it changed; a change Actions marks same or kept, such as a
// seeded file that exists or a path git tracks, is never written.
func (p Plan) Apply() ([]Action, error) {
	verbByPath := map[string]string{}
	var done []Action
	for _, action := range p.Actions() {
		verbByPath[action.Path] = action.Verb
		if action.Verb == "same" || action.Verb == "keep" {
			continue
		}
		done = append(done, action)
	}
	for _, change := range p.Changes {
		relative := change.Path
		if rel, err := filepath.Rel(p.Root, change.Path); err == nil && !strings.HasPrefix(rel, "..") {
			relative = rel
		}
		if change.Remove {
			if err := os.RemoveAll(change.Path); err != nil && !os.IsNotExist(err) {
				return done, err
			}
			continue
		}
		if verb := verbByPath[relative]; verb == "same" || verb == "keep" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(change.Path), 0o755); err != nil {
			return done, err
		}
		if err := fsx.WriteFile(change.Path, change.Body, change.Mode); err != nil {
			return done, err
		}
	}
	return done, nil
}

// Print writes the plan's actions, which is what --dry-run shows.
func (p Plan) Print(out io.Writer) {
	actions := p.Actions()
	changed := 0
	for _, action := range actions {
		if action.Verb == "same" {
			continue
		}
		changed++
		fmt.Fprintf(out, "%-7s %s\n", action.Verb, action.Path)
	}
	fmt.Fprintf(out, "%s: %d file(s), %d would change\n", p.Host, len(actions), changed)
}
