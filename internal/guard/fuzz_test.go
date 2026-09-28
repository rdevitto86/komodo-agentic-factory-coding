package guard

import (
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
			if strings.ContainsAny(prefix+command, "\x00") {
				continue
			}
			if !Check(bashRequest(root, prefix+command), policy, "feat/x").Deny {
				t.Fatalf("prefix %q let %q through", prefix, command)
			}
		}
	})
}

// FuzzTokenize proves the tokenizer ends on any input and never panics.
func FuzzTokenize(f *testing.F) {
	for _, seed := range []string{"", "'", "\"", "$(", "`", "<<", "a\\", "2>&"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, command string) {
		_ = tokenize(command)
	})
}

// bashRequest builds a request for the fake host's shell tool.
func bashRequest(root, command string) Request {
	return Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": command}}
}
