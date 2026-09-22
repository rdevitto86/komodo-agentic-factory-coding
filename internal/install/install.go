// Package install renders a host's project configuration as a plan of file changes.
package install

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	Why     string
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

// AddRemoval appends a path the install deletes when it is present.
func (p *Plan) AddRemoval(path, why string) {
	p.Changes = append(p.Changes, Change{Path: path, Remove: true, Why: why})
}

// Action is what one change would do to the tree as it stands.
type Action struct {
	Verb string
	Path string
	Why  string
}

// Actions describes the plan against the current tree without touching it.
func (p Plan) Actions() []Action {
	var out []Action
	for _, change := range p.Changes {
		relative := change.Path
		if rel, err := filepath.Rel(p.Root, change.Path); err == nil && !strings.HasPrefix(rel, "..") {
			relative = rel
		}
		if change.Remove {
			if _, err := os.Stat(change.Path); err == nil {
				out = append(out, Action{"remove", relative, change.Why})
			}
			continue
		}
		existing, err := os.ReadFile(change.Path)
		switch {
		case change.Seed && err == nil:
			out = append(out, Action{"keep", relative, "seeded once, never overwritten"})
		case err != nil:
			out = append(out, Action{"create", relative, change.Why})
		case !bytes.Equal(existing, change.Body):
			out = append(out, Action{"update", relative, change.Why})
		default:
			out = append(out, Action{"same", relative, change.Why})
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
