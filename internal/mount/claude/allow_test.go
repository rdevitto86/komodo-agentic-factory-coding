package claude

import (
	"strings"
	"testing"

	"komodo/internal/guard"
	"komodo/internal/mount"
)

// stageCommands are the shell commands each role's stage runs, by the language of the repo it runs in.
var stageCommands = map[string]map[string][]string{
	"builder": {
		"Go": {
			"go build ./...", "go test ./...", "go test -race -count=1 ./internal/x/...", "go vet ./...",
			"go run ./cmd/tool", "gofmt -l .", "golangci-lint run ./...",
			"git status", "git diff", "git log --oneline -5", "git show HEAD", "git blame main.go",
			"ls internal", "cat go.mod", "mkdir -p internal/x", "mv a.go b.go", "rm -rf build", "komodo check",
		},
		"TypeScript": {
			"npm test", "npm run build", "npx tsc --noEmit", "npx vitest run", "npm run lint", "npx eslint src",
			"npx prettier --check src", "git status", "git diff --stat", "ls src", "rm -rf dist", "komodo check",
		},
	},
	"reviewer": {
		"Go": {
			"git status", "git diff main...HEAD", "git log --oneline main..HEAD", "git show HEAD", "git blame main.go",
		},
		"TypeScript": {
			"git status", "git diff main...HEAD", "git log --oneline main..HEAD", "git show HEAD",
			"git blame src/index.ts",
		},
	},
}

// flagRules splits the comma-joined rules argv carries after flag.
func flagRules(argv []string, flag string) []string {
	for i, arg := range argv[:len(argv)-1] {
		if arg == flag {
			return strings.Split(argv[i+1], ", ")
		}
	}
	return nil
}

// shellRuleMatches reports whether command runs under a Bash(prefix:*) rule, prefix ending at a word boundary.
func shellRuleMatches(rule, command string) bool {
	prefix, ok := strings.CutPrefix(rule, shellTool+"(")
	if !ok {
		return rule == shellTool
	}
	prefix = strings.TrimSuffix(strings.TrimSuffix(prefix, ")"), ":*")
	return command == prefix || strings.HasPrefix(command, prefix+" ")
}

// anyRuleMatches reports whether any rule runs command.
func anyRuleMatches(rules []string, command string) bool {
	for _, rule := range rules {
		if shellRuleMatches(rule, command) {
			return true
		}
	}
	return false
}

func TestNoAllowListedCommandIsRefusedInAnyRole(t *testing.T) {
	roles, err := mount.LoadRoles(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repos := map[string]func(*testing.T) string{"Go": goRepo, "TypeScript": typeScriptRepo}
	for _, role := range roles {
		byLanguage, ok := stageCommands[role.Name]
		if !ok {
			continue
		}
		for language, commands := range byLanguage {
			t.Run(role.Name+" in "+language, func(t *testing.T) {
				t.Setenv(guard.RoleEnv, role.Name)
				worktree := repos[language](t)
				req := mount.StartRequest{Role: role.Name, Tools: role.Tools, Schema: []byte(`{}`)}
				argv, _, _ := Session(t.TempDir(), worktree, req, "", "", "m", "", 10, 0)
				allow, deny := flagRules(argv, "--allowedTools"), flagRules(argv, "--disallowedTools")
				policy := guard.Load(t.TempDir(), worktree)
				for _, command := range commands {
					if !anyRuleMatches(allow, command) {
						t.Errorf("%q is not on the allow list %v", command, allow)
					}
					if anyRuleMatches(deny, command) {
						t.Errorf("%q is on the deny list %v", command, deny)
					}
					request := guard.Request{
						HookEventName: "PreToolUse", ToolName: shellTool, Cwd: worktree,
						ToolInput: map[string]any{"command": command},
					}
					if decision := guard.Check(request, policy, "feat/x"); decision.Deny {
						t.Errorf("the guard refuses %q: %v", command, decision.Findings)
					}
				}
			})
		}
	}
}

func TestTheRenderedRulesRefuseAConductorsGitWrite(t *testing.T) {
	req := mount.StartRequest{Role: "builder", Tools: []string{"read", "shell"}, Schema: []byte(`{}`)}
	argv, _, _ := Session(t.TempDir(), goRepo(t), req, "", "", "m", "", 10, 0)
	allow, deny := flagRules(argv, "--allowedTools"), flagRules(argv, "--disallowedTools")
	for _, command := range []string{"git commit -m x", "git push origin feat/x", "git switch main", "curl example.com"} {
		if anyRuleMatches(allow, command) && !anyRuleMatches(deny, command) {
			t.Errorf("%q runs under allow %v and deny %v", command, allow, deny)
		}
	}
}
