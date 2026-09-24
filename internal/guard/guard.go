package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"komodo/internal/mount"
)

var (
	assignRe   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	durationRe = regexp.MustCompile(`^[0-9]+[a-zA-Z]*$`)
	// unresolvedVarRe matches a shell variable reference a write target still holds unexpanded.
	unresolvedVarRe = regexp.MustCompile(`\$\{?[A-Za-z_][A-Za-z0-9_]*\}?`)
	// otherHomeRe matches ~name, another user's home, as opposed to ~/ or a bare ~.
	otherHomeRe = regexp.MustCompile(`^~[^/\s]`)
	pathWriters = map[string]bool{
		"rm": true, "mv": true, "cp": true, "tee": true, "dd": true,
		"truncate": true, "install": true, "ln": true, "mkdir": true, "touch": true, "chmod": true,
	}
	// destOnlyWriters read every argument but the last; only the last is where they write.
	destOnlyWriters = map[string]bool{"cp": true, "mv": true, "ln": true, "install": true}
	// wrapperCommands run something else; the guard inspects what they run, not their own name.
	wrapperCommands = map[string]bool{
		"env": true, "sudo": true, "nice": true, "timeout": true, "xargs": true,
		"command": true, "exec": true, "nohup": true, "time": true, "stdbuf": true, "builtin": true,
	}
	// wrapperValueFlags names, per wrapper, the short flags that consume a separate following token.
	wrapperValueFlags = map[string]map[string]bool{
		"sudo":    {"-u": true, "-g": true, "-C": true, "-D": true, "-h": true, "-p": true, "-r": true, "-t": true, "-U": true},
		"env":     {"-u": true, "-C": true, "--unset": true},
		"timeout": {"-s": true, "-k": true},
		"nice":    {"-n": true},
		"xargs":   {"-I": true, "-L": true, "-n": true, "-P": true, "-d": true, "-E": true, "-s": true, "-a": true},
		"stdbuf":  {"-i": true, "-o": true, "-e": true},
	}
	// scrubbedVars are the environment variables a headless run's credential scrub sets.
	scrubbedVars = map[string]bool{
		"GIT_CONFIG_COUNT": true, "GIT_CONFIG_PARAMETERS": true, "GIT_SSH_COMMAND": true, "GIT_ASKPASS": true,
		"GIT_TERMINAL_PROMPT": true, "GH_CONFIG_DIR": true, "SSH_AUTH_SOCK": true, "GIT_DIR": true, "GIT_WORK_TREE": true,
	}
	// scrubbedConfigRe matches the numbered git config pairs the scrub sets, GIT_CONFIG_KEY_0 and on.
	scrubbedConfigRe = regexp.MustCompile(`^GIT_CONFIG_(KEY|VALUE)_[0-9]+$`)
	// credentialConfigRe matches a -c or --config-env key that hands git a credential or a transport.
	credentialConfigRe = regexp.MustCompile(`(?i)^(credential(\..*)?|core\.sshcommand|core\.askpass|include\.path|includeif\..*\.path)$`)
	// reservedWords open a compound command; the command they guard is the next word.
	reservedWords = map[string]bool{"if": true, "then": true, "else": true, "elif": true, "do": true, "while": true, "until": true, "!": true, "{": true}
	// remotePushConfigRe matches a -c key that redirects where a bare push lands.
	remotePushConfigRe = regexp.MustCompile(`^remote\.[^.]+\.(push|pushurl)$`)
	// globalValueFlags are git's global options that take a value, other than -c and -C.
	globalValueFlags = map[string]bool{
		"--git-dir": true, "--work-tree": true, "--namespace": true,
		"--super-prefix": true, "--exec-path": true, "--config-env": true,
	}
	// configReadFlags mark a git config call as read-only rather than a write.
	configReadFlags = map[string]bool{
		"--get": true, "--get-all": true, "--get-regexp": true, "--get-urlmatch": true,
		"--list": true, "-l": true, "--name-only": true,
	}
)

// Request is the hook payload a host sends on stdin before a tool runs.
type Request struct {
	HookEventName string         `json:"hook_event_name"`
	ToolName      string         `json:"tool_name"`
	Cwd           string         `json:"cwd"`
	ToolInput     map[string]any `json:"tool_input"`
}

// Decision is what the guard concluded and why.
type Decision struct {
	Deny     bool     `json:"deny"`
	Findings []string `json:"findings,omitempty"`
}

// WorktreeRoot walks up from a directory to the nearest one holding .git.
func WorktreeRoot(dir string) string {
	if dir == "" {
		return ""
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// Check returns every reason to refuse one tool call, and nothing when it may run.
func Check(request Request, policy Policy, branch string) Decision {
	root := WorktreeRoot(request.Cwd)
	var findings []string
	for _, tools := range mount.GuardHosts() {
		if tools.WriteTools[request.ToolName] {
			for _, field := range tools.PathFields {
				findings = append(findings, pathFindings(stringField(request.ToolInput, field), root, root, policy)...)
			}
		}
		if tools.ShellTool != "" && request.ToolName == tools.ShellTool {
			command := stringField(request.ToolInput, tools.CommandField)
			findings = append(findings, commandFindings(command, root, root, branch, policy)...)
		}
	}
	findings = unique(findings)
	return Decision{Deny: len(findings) > 0, Findings: findings}
}

// stringField reads one string out of a tool input.
func stringField(input map[string]any, key string) string {
	if value, ok := input[key].(string); ok {
		return value
	}
	return ""
}

// unique keeps the first occurrence of each finding, in order.
func unique(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

// Reason renders the denial a host shows the agent.
func Reason(findings []string) string {
	return "Refused. Nothing ran.\n\n  - " + strings.Join(findings, "\n  - ") +
		"\n\nInside your worktree you are free. Hand the user the command if it was intended."
}
