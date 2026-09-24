package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FuzzCheck proves the guard never panics or hangs, and that a harmless prefix never hides a denial.
func FuzzCheck(f *testing.F) {
	for _, item := range Table(DefaultPolicy()) {
		if command, ok := item.Input["command"].(string); ok {
			f.Add(command)
		}
	}
	registerFakeHost()
	root := f.TempDir()
	policy := DefaultPolicy()
	f.Fuzz(func(t *testing.T, command string) {
		denied := Check(bashRequest(root, command), policy, "feat/x").Deny
		if Check(bashRequest(root, command), policy, "feat/x").Deny != denied {
			t.Fatalf("two checks of %q disagree", command)
		}
		if !denied {
			return
		}
		for _, prefix := range []string{"true; ", "echo hi && ", "true\n"} {
			if strings.HasPrefix(command, "#") || strings.ContainsAny(prefix+command, "\x00") {
				continue
			}
			if !Check(bashRequest(root, prefix+command), policy, "feat/x").Deny {
				t.Fatalf("prefix %q let %q through", prefix, command)
			}
		}
	})
}

// FuzzLex proves the lexer ends on any input and never loses a word to a panic.
func FuzzLex(f *testing.F) {
	for _, seed := range []string{"", "'", "\"", "$(", "`", "${", "<<", "<<EOF\n", "$'\\x", "{a,{b,c}}", "a\\", "<(", "2>&"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, command string) {
		line := lex(command)
		_ = parse(line.tokens)
		_ = lexWords(command)
	})
}

// bashRequest builds a request for the fake host's shell tool.
func bashRequest(root, command string) Request {
	return Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": command}}
}

// TestAFileThatSourcesItselfStops proves the depth cap ends a loop the shell itself would never end.
func TestAFileThatSourcesItselfStops(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	if err := os.WriteFile(filepath.Join(root, "loop.sh"), []byte("source loop.sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	decision := Check(bashRequest(root, "source loop.sh"), DefaultPolicy(), "feat/x")
	if !decision.Deny || !containsAny(decision.Findings, "nest deeper") {
		t.Fatalf("a self-sourcing file = %+v", decision)
	}
}

// TestAShellReadsTheScriptFileItNames proves bash script.sh is judged by the script's own commands.
func TestAShellReadsTheScriptFileItNames(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	if err := os.WriteFile(filepath.Join(root, "ship.sh"), []byte("git push origin main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Check(bashRequest(root, "bash -o pipefail ship.sh"), DefaultPolicy(), "feat/x").Deny {
		t.Fatal("a script file a shell runs passed unread")
	}
}
