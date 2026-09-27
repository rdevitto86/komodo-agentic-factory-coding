package claude

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"komodo/internal/detect"
	"komodo/internal/guard"
	"komodo/internal/mount"
	"komodo/internal/toolkit"
)

// shellVerb is the Komodo verb a role names to run shell commands.
const shellVerb = "shell"

// commandClasses are the fixed shell command prefixes each class a role names allows.
var commandClasses = map[string][]string{
	"files":        {"ls", "cat", "head", "tail", "wc", "find", "mkdir", "touch", "cp", "mv", "rm"},
	"git-read":     {"git status", "git diff", "git log", "git show", "git blame"},
	"komodo-check": {"komodo check"},
}

// languageCommands are the build, test, lint and format prefixes a detected language adds to those classes.
var languageCommands = map[string]map[string][]string{
	"Go": {
		"build": {"go build", "go run"}, "test": {"go test"},
		"lint": {"go vet", "golangci-lint"}, "format": {"gofmt", "go fmt"},
	},
	"TypeScript": {
		"build": {"npm run build", "npx tsc"}, "test": {"npm test", "npx vitest", "npx jest"},
		"lint": {"npm run lint", "npx eslint"}, "format": {"npm run format", "npx prettier"},
	},
	"JavaScript": {
		"build": {"npm run build"}, "test": {"npm test", "npx vitest", "npx jest"},
		"lint": {"npm run lint", "npx eslint"}, "format": {"npm run format", "npx prettier"},
	},
	"Python": {
		"test": {"pytest", "python -m pytest"}, "lint": {"ruff check"}, "format": {"ruff format", "black"},
	},
	"Rust": {
		"build": {"cargo build"}, "test": {"cargo test"}, "lint": {"cargo clippy"}, "format": {"cargo fmt"},
	},
}

// gitWrites are the git subcommands only the conductor runs, refused to every role with a shell.
var gitWrites = []string{"add", "commit", "switch", "checkout", "branch", "reset", "rebase", "merge", "stash", "push"}

// roleTools returns the role's verbs, adding a shell when its frontmatter names command classes to run in one.
// A request carrying no tools gets none.
func roleTools(root string, req mount.StartRequest) []string {
	if len(req.Tools) == 0 || len(roleCommands(root, req.Role)) == 0 || slices.Contains(req.Tools, shellVerb) {
		return req.Tools
	}
	return append(slices.Clone(req.Tools), shellVerb)
}

// rolePermissions returns the allow and deny rules a role's session runs under. A role naming command
// classes gets only those shell prefixes; the build and test classes add the worktree's detected commands.
func rolePermissions(root, worktree string, req mount.StartRequest) (allow, deny []string) {
	classes := roleCommands(root, req.Role)
	shell := false
	for _, verb := range roleTools(root, req) {
		if verb == shellVerb {
			shell = true
			if len(classes) > 0 {
				profile, _ := detect.Detect(worktree)
				allow = append(allow, shellRules(classes, profile)...)
				continue
			}
		}
		allow = append(allow, tools[verb]...)
	}
	for _, refused := range guard.LineRefusedPaths {
		deny = append(deny, fmt.Sprintf("Edit(%s)", refused))
	}
	if shell {
		for _, sub := range gitWrites {
			deny = append(deny, fmt.Sprintf("%s(git %s:*)", shellTool, sub))
		}
	}
	return unique(allow), deny
}

// shellRules renders each class's command prefixes, fixed and detected, as this host's shell rules.
func shellRules(classes []string, profile detect.Profile) []string {
	var rules []string
	for _, class := range classes {
		prefixes := append([]string{}, commandClasses[class]...)
		switch class {
		case "build":
			prefixes = append(prefixes, profile.Compile)
		case "test":
			prefixes = append(prefixes, profile.Verify)
		}
		for _, language := range profile.Languages {
			prefixes = append(prefixes, languageCommands[language][class]...)
		}
		for _, prefix := range prefixes {
			if prefix != "" {
				rules = append(rules, fmt.Sprintf("%s(%s:*)", shellTool, prefix))
			}
		}
	}
	return rules
}

// roleCommands reads the command classes a role's frontmatter names, or none when it has no role file.
func roleCommands(root, role string) []string {
	data, err := fs.ReadFile(toolkit.FS(root), path.Join("roles", role+".md"))
	if err != nil {
		return nil
	}
	head, _, _ := strings.Cut(strings.TrimPrefix(string(data), "---\n"), "\n---\n")
	for _, line := range strings.Split(head, "\n") {
		value, found := strings.CutPrefix(line, "commands:")
		if !found {
			continue
		}
		var classes []string
		for _, item := range strings.Split(strings.Trim(strings.TrimSpace(value), "[]"), ",") {
			if item = strings.TrimSpace(item); item != "" {
				classes = append(classes, item)
			}
		}
		return classes
	}
	return nil
}

// unique keeps the first occurrence of each rule, in order.
func unique(rules []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, rule := range rules {
		if !seen[rule] {
			seen[rule] = true
			out = append(out, rule)
		}
	}
	return out
}
