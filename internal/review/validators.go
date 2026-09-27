// Package review measures a group's diff before any lens reads it and decides which lens findings block.
package review

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"komodo/internal/check"
	"komodo/internal/proc"
)

// Kind names the validator that took one measurement.
type Kind string

// The validators that run before the lenses.
const (
	KindTests        Kind = "tests"
	KindReproducer   Kind = "reproducer"
	KindSecrets      Kind = "secrets"
	KindAudit        Kind = "audit"
	KindSecurityLint Kind = "security-lint"
	KindCallers      Kind = "callers"
)

// outputTail is how many bytes of a command's output a measurement keeps, from the end.
const outputTail = 2000

// Validators are the tree, its diff, and the commands the repo has for each validator; an empty command is skipped.
type Validators struct {
	Worktree     string
	Diff         string
	Tests        string
	Reproducers  []string
	Audit        string
	SecurityLint string
}

// Measurement is what one validator found.
type Measurement struct {
	Kind    Kind   `json:"kind"`
	Command string `json:"command,omitempty"`
	Passed  bool   `json:"passed"`
	Detail  string `json:"detail"`
}

// Report is every measurement, in the order the validators ran.
type Report struct {
	Measurements []Measurement `json:"measurements"`
}

// exportedDecl captures the name an added Go line declares when that name is exported.
var exportedDecl = regexp.MustCompile(`^(?:func\s+(?:\([^)]*\)\s*)?|type\s+|var\s+|const\s+)([A-Z]\w*)`)

// skippedDirs are the directories a caller count never walks.
var skippedDirs = map[string]bool{".git": true, ".komodo": true, "vendor": true, "node_modules": true, "testdata": true}

// Measure runs the tests, each reproducer, the secret scan, the audit and security linter the repo has,
// and counts the callers of every exported symbol the diff changes.
func Measure(v Validators) (Report, error) {
	var report Report
	if v.Tests != "" {
		report.Measurements = append(report.Measurements, runCommand(v.Worktree, KindTests, v.Tests))
	}
	for _, command := range v.Reproducers {
		report.Measurements = append(report.Measurements, runCommand(v.Worktree, KindReproducer, command))
	}
	secrets := check.Secrets(v.Diff)
	report.Measurements = append(report.Measurements, Measurement{
		Kind: KindSecrets, Passed: len(secrets) == 0, Detail: strings.Join(secrets, "\n"),
	})
	if v.Audit != "" {
		report.Measurements = append(report.Measurements, runCommand(v.Worktree, KindAudit, v.Audit))
	}
	if v.SecurityLint != "" {
		report.Measurements = append(report.Measurements, runCommand(v.Worktree, KindSecurityLint, v.SecurityLint))
	}
	callers, err := countCallers(v.Worktree, changedExported(v.Diff))
	if err != nil {
		return Report{}, err
	}
	report.Measurements = append(report.Measurements, callers...)
	return report, nil
}

// runCommand runs one validator's command in the worktree and keeps the tail of its output.
func runCommand(worktree string, kind Kind, command string) Measurement {
	ran := proc.Shell(worktree, command, check.CommandTimeout)
	detail := strings.TrimSpace(ran.Output)
	if len(detail) > outputTail {
		detail = detail[len(detail)-outputTail:]
	}
	if !ran.OK() {
		detail = strings.TrimSpace(fmt.Sprintf("%v\n%s", ran.Err(), detail))
	}
	return Measurement{Kind: kind, Command: command, Passed: ran.OK(), Detail: detail}
}

// changedExported names each exported Go symbol a declaration on an added line introduces, sorted.
func changedExported(diff string) []string {
	seen := map[string]bool{}
	for _, line := range check.ParseAddedLines(diff) {
		if !strings.HasSuffix(line.File, ".go") {
			continue
		}
		if match := exportedDecl.FindStringSubmatch(line.Text); match != nil {
			seen[match[1]] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// countCallers measures, per symbol, the references to it across the worktree's Go files, declarations
// left out. A file that does not parse is skipped.
func countCallers(worktree string, symbols []string) ([]Measurement, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	wanted := make(map[string]bool, len(symbols))
	for _, name := range symbols {
		wanted[name] = true
	}
	refs := map[string]int{}
	files := map[string]map[string]bool{}
	err := filepath.WalkDir(worktree, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != worktree && skippedDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		parsed, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil
		}
		rel, relErr := filepath.Rel(worktree, path)
		if relErr != nil {
			return relErr
		}
		declared := map[*ast.Ident]bool{}
		ast.Inspect(parsed, func(node ast.Node) bool {
			switch decl := node.(type) {
			case *ast.FuncDecl:
				declared[decl.Name] = true
			case *ast.TypeSpec:
				declared[decl.Name] = true
			case *ast.ValueSpec:
				for _, name := range decl.Names {
					declared[name] = true
				}
			case *ast.Ident:
				if wanted[decl.Name] && !declared[decl] {
					refs[decl.Name]++
					if files[decl.Name] == nil {
						files[decl.Name] = map[string]bool{}
					}
					files[decl.Name][filepath.ToSlash(rel)] = true
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]Measurement, 0, len(symbols))
	for _, name := range symbols {
		out = append(out, Measurement{
			Kind: KindCallers, Passed: true,
			Detail: fmt.Sprintf("%s: %d references in %d files", name, refs[name], len(files[name])),
		})
	}
	return out, nil
}

// Markdown renders the report as the settled-fact section every lens's brief carries.
func (r Report) Markdown() string {
	var out strings.Builder
	out.WriteString("## Validators\n\nMeasured before any lens ran. This is settled fact: cite it, never rerun or dispute it.\n")
	for _, each := range r.Measurements {
		verdict := "passed"
		if !each.Passed {
			verdict = "failed"
		}
		fmt.Fprintf(&out, "\n### %s: %s\n", each.Kind, verdict)
		if each.Command != "" {
			fmt.Fprintf(&out, "\n`%s`\n", each.Command)
		}
		if each.Detail != "" {
			fmt.Fprintf(&out, "\n```\n%s\n```\n", each.Detail)
		}
	}
	return out.String()
}
