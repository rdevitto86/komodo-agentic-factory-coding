// Package install renders a host's project configuration as a plan of file changes.
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

// normaliseHooks rewrites a guard hook that runs any komodo binary to one fixed name, so drift ignores which copy runs.
func normaliseHooks(body []byte) []byte {
	return hookCommand.ReplaceAllFunc(body, func(match []byte) []byte {
		raw := hookCommand.FindSubmatch(match)[1]
		path, err := strconv.Unquote(`"` + string(raw) + `"`)
		if err != nil {
			return match
		}
		base := path[strings.LastIndexAny(path, `/\`)+1:]
		if !strings.HasPrefix(base, "komodo") {
			return match
		}
		return []byte(`"command": "komodo guard"`)
	})
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
}

// Actions describes the plan against the current tree without touching it.
func (p Plan) Actions() []Action {
	return p.actions(func(body []byte) []byte { return body })
}

// Drift describes the plan like Actions, but a guard hook naming any komodo binary matches any other.
func (p Plan) Drift() []Action {
	return p.actions(normaliseHooks)
}

// actions compares each change to the tree after passing both sides through normalise.
func (p Plan) actions(normalise func([]byte) []byte) []Action {
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

// Apply writes the plan and returns what it changed.
func (p Plan) Apply() ([]Action, error) {
	var done []Action
	for _, action := range p.Actions() {
		if action.Verb == "same" || action.Verb == "keep" {
			continue
		}
		done = append(done, action)
	}
	for _, change := range p.Changes {
		if change.Remove {
			if err := os.RemoveAll(change.Path); err != nil && !os.IsNotExist(err) {
				return done, err
			}
			continue
		}
		if change.Seed {
			if _, err := os.Stat(change.Path); err == nil {
				continue
			}
		}
		if err := os.MkdirAll(filepath.Dir(change.Path), 0o755); err != nil {
			return done, err
		}
		if err := os.WriteFile(change.Path, change.Body, change.Mode); err != nil {
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
