package guard

import (
	"encoding/json"

	"komodo/internal/mount"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// worktree builds a directory that looks like a git worktree root.
func worktree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// outsideTemp builds a worktree root outside the temp directories, so the table's above-the-root
// rows are not writes the temp allowance lets through.
func outsideTemp(t *testing.T) string {
	t.Helper()
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Skip("no user cache directory to root the table outside temp")
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(cache, "komodo-guard-table-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// registerFakeHost puts one mount in the registry so the policy protects a host home, and its
// tool names and denial encoding drive Check the way a real mount's would.
func registerFakeHost() {
	mount.Register(mount.Host{Name: "testhost", ConfigPaths: []string{"~/.testhost/**"}})
	mount.RegisterGuard("testhost", mount.GuardTools{
		WriteTools:   map[string]bool{"Write": true, "Edit": true, "MultiEdit": true, "NotebookEdit": true},
		PathFields:   []string{"file_path", "notebook_path"},
		ShellTool:    "Bash",
		CommandField: "command",
		ConfigPaths:  []string{".testhost/settings.json"},
		Deny:         fakeDenyPayload,
	})
}

// fakeDenyPayload mirrors the JSON shape a real host's PreToolUse hook reads.
func fakeDenyPayload(reason string) []byte {
	out, err := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		},
	})
	if err != nil {
		return nil
	}
	return out
}

func TestTableHoldsAtLeastSixtyCommandsHalfAllowed(t *testing.T) {
	registerFakeHost()
	table := Table(DefaultPolicy())
	if len(table) < 60 {
		t.Fatalf("the table runs %d commands; the gate needs at least 60", len(table))
	}
	denied := 0
	for _, item := range table {
		if item.Deny {
			denied++
		}
	}
	allowed := len(table) - denied
	if denied < len(table)/3 || allowed < len(table)/3 {
		t.Fatalf("%d denied and %d allowed is not a balanced table", denied, allowed)
	}
}

func TestEveryTableRowHoldsAndEachDenialIsNamed(t *testing.T) {
	registerFakeHost()
	root := outsideTemp(t)
	if wrong := RunTable(root, DefaultPolicy()); len(wrong) != 0 {
		t.Fatalf("%d row(s) wrong:\n%s", len(wrong), strings.Join(wrong, "\n"))
	}
}

func TestEveryDeniedRowNamesAFinding(t *testing.T) {
	registerFakeHost()
	for _, item := range Table(DefaultPolicy()) {
		if item.Deny && item.Finding == "" {
			t.Errorf("%q is denied with no finding named", item.Name)
		}
	}
}

func TestCheckIsSilentOnAReadTool(t *testing.T) {
	root := worktree(t)
	request := Request{HookEventName: "PreToolUse", ToolName: "Read", Cwd: root,
		ToolInput: map[string]any{"file_path": "/etc/hosts"}}
	if Check(request, DefaultPolicy(), "feat/x").Deny {
		t.Fatal("the guard denied a read")
	}
}

func TestAHostsConfigPathsReachThePolicy(t *testing.T) {
	registerFakeHost()
	policy := DefaultPolicy()
	found := false
	for _, path := range policy.ConfigPaths {
		if path == "~/.testhost/**" {
			found = true
		}
	}
	if !found {
		t.Fatalf("a registered mount's paths did not reach the policy: %v", policy.ConfigPaths)
	}
}

func TestPolicyAddsButNeverRemoves(t *testing.T) {
	toolkit := t.TempDir()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(toolkit, "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	shipped := `{"critical_refs":["main","master"],"config_paths":["bin/**"],"trailer_patterns":["(?i)co-authored[-]by"]}`
	if err := os.WriteFile(filepath.Join(toolkit, "komodo", "policy.json"), []byte(shipped), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	extra := `{"critical_refs":["release"],"config_paths":[]}`
	if err := os.WriteFile(filepath.Join(repo, ".komodo", "policy.json"), []byte(extra), 0o644); err != nil {
		t.Fatal(err)
	}
	policy := Load(toolkit, repo)
	if !policy.IsCritical("release") {
		t.Fatal("the repo's added ref is not protected")
	}
	if !policy.IsCritical("main") || !policy.IsCritical("master") {
		t.Fatal("a repo removed a shipped critical ref")
	}
}

// TestShippedPolicyNeverDropsAMountsConfigPath proves the shipped policy widens the default
// rather than replacing it, so a registered mount's own path survives a policy.json with none.
func TestShippedPolicyNeverDropsAMountsConfigPath(t *testing.T) {
	registerFakeHost()
	toolkit := t.TempDir()
	if err := os.MkdirAll(filepath.Join(toolkit, "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	shipped := `{"critical_refs":["main"],"config_paths":["komodo/policy.json"],"trailer_patterns":[]}`
	if err := os.WriteFile(filepath.Join(toolkit, "komodo", "policy.json"), []byte(shipped), 0o644); err != nil {
		t.Fatal(err)
	}
	policy := Load(toolkit, t.TempDir())
	found := false
	for _, path := range policy.ConfigPaths {
		if path == "~/.testhost/**" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the shipped policy dropped a mount's own path: %v", policy.ConfigPaths)
	}
}

// TestOverlayCriticalRefsReachesLoad proves the machine overlay's critical refs merge in.
func TestOverlayCriticalRefsReachesLoad(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	overlay := `{"critical_refs":["release/2.0"]}`
	if err := os.WriteFile(filepath.Join(home, ".komodo", "config.json"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}
	policy := Load(t.TempDir(), t.TempDir())
	if !policy.IsCritical("release/2.0") {
		t.Fatal("the machine overlay's critical ref never reached the guard")
	}
}

func TestIsCriticalHonoursATrailingStar(t *testing.T) {
	policy := DefaultPolicy()
	policy.CriticalRefs = append(policy.CriticalRefs, "release/*")
	if !policy.IsCritical("release/1.0") || policy.IsCritical("feat/release") {
		t.Fatal("the star pattern matched the wrong refs")
	}
	if !policy.IsCritical("origin/main") || !policy.IsCritical("refs/heads/main") {
		t.Fatal("a qualified ref was not recognised")
	}
}

func TestHasTrailerCatchesEveryShippedPattern(t *testing.T) {
	policy := DefaultPolicy()
	for _, message := range []string{
		"feat: x\n\nCo-authored-by: A <a@b.c>",
		"feat: x\n\nco-authored-by: a",
		"feat: x\n\nGenerated-with: a tool",
		"feat: x\n\nGenerated-by: a tool",
		"feat: x\n\n\U0001F916",
	} {
		if !policy.HasTrailer(message) {
			t.Errorf("no trailer found in %q", message)
		}
	}
	for _, message := range []string{
		"feat: x\n\nWhat it does.",
		"feat: x\n\nThe files generated by stringer are not tracked.",
	} {
		if policy.HasTrailer(message) {
			t.Fatalf("a clean message was read as carrying a trailer: %q", message)
		}
	}
}

func TestWorktreeRootFindsTheNearestGit(t *testing.T) {
	root := worktree(t)
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := WorktreeRoot(deep); got != root {
		t.Fatalf("root = %s, want %s", got, root)
	}
	if got := WorktreeRoot(""); got != "" {
		t.Fatalf("an empty cwd has no root, got %s", got)
	}
}

func TestSplitWordsKeepsQuotedArguments(t *testing.T) {
	words, err := splitWords(`git commit -m "feat: a thing"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 4 || words[3] != "feat: a thing" {
		t.Fatalf("words = %q", words)
	}
	if _, err := splitWords(`git commit -m "unterminated`); err == nil {
		t.Fatal("an unterminated quote must error so the caller falls back")
	}
}

func TestAMalformedCommandStillFallsBackToFields(t *testing.T) {
	root := worktree(t)
	request := Request{ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": `git push origin main "`}}
	if !Check(request, DefaultPolicy(), "feat/x").Deny {
		t.Fatal("an unterminated quote let a push to main through")
	}
}

// TestTrailerThroughAMessageFileIsCaught proves -F reads the file's own content for a trailer.
func TestTrailerThroughAMessageFileIsCaught(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	msgfile := filepath.Join(root, "msg.txt")
	if err := os.WriteFile(msgfile, []byte("feat: x\n\nCo-authored-by: A <a@b.c>"), 0o644); err != nil {
		t.Fatal(err)
	}
	request := Request{ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": "git commit -F msg.txt"}}
	if !Check(request, DefaultPolicy(), "feat/x").Deny {
		t.Fatal("a trailer read from a message file passed")
	}
}

// TestCheckHonoursARegisteredHostsOwnToolNames proves Check never hard-codes a tool name,
// reading it from whatever mount registered the write and shell tools instead.
func TestCheckHonoursARegisteredHostsOwnToolNames(t *testing.T) {
	mount.RegisterGuard("otherhost", mount.GuardTools{
		WriteTools:   map[string]bool{"Scribble": true},
		PathFields:   []string{"target"},
		ShellTool:    "Run",
		CommandField: "line",
	})
	root := worktree(t)
	write := Request{ToolName: "Scribble", Cwd: root, ToolInput: map[string]any{"target": "/etc/hosts"}}
	if !Check(write, DefaultPolicy(), "feat/x").Deny {
		t.Fatal("a write through a registered host's own tool name passed")
	}
	run := Request{ToolName: "Run", Cwd: root, ToolInput: map[string]any{"line": "git push origin main"}}
	if !Check(run, DefaultPolicy(), "feat/x").Deny {
		t.Fatal("a push through a registered host's own shell tool name passed")
	}
}

func TestReasonNamesEveryFinding(t *testing.T) {
	text := Reason([]string{"one", "two"})
	if !strings.Contains(text, "- one") || !strings.Contains(text, "- two") {
		t.Fatalf("reason = %q", text)
	}
	if !strings.Contains(text, "Inside your worktree you are free") {
		t.Fatal("the denial does not say what is still allowed")
	}
}

func TestFindingsAreNotRepeated(t *testing.T) {
	root := worktree(t)
	request := Request{ToolName: "Bash", Cwd: root,
		ToolInput: map[string]any{"command": "git push origin main && git push origin main"}}
	decision := Check(request, DefaultPolicy(), "feat/x")
	if len(decision.Findings) != 1 {
		t.Fatalf("findings = %v", decision.Findings)
	}
}

func TestHookDeniesWithTheDecisionBothHostsAccept(t *testing.T) {
	root := worktree(t)
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Write","cwd":"` + root + `","tool_input":{"file_path":"/etc/hosts"}}`
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(payload), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want 0 with a JSON denial", code)
	}
	var decoded struct {
		Hook struct {
			Event  string `json:"hookEventName"`
			Decide string `json:"permissionDecision"`
			Reason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("output is not JSON: %q", out.String())
	}
	if decoded.Hook.Event != "PreToolUse" || decoded.Hook.Decide != "deny" {
		t.Fatalf("decision = %+v", decoded.Hook)
	}
	if !strings.Contains(decoded.Hook.Reason, "outside the worktree") {
		t.Fatalf("reason = %q", decoded.Hook.Reason)
	}
}

func TestHookIsSilentOnAnAllowedCall(t *testing.T) {
	root := worktree(t)
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","cwd":"` + root + `","tool_input":{"command":"go test ./..."}}`
	var out, errOut strings.Builder
	if code := Hook(root, strings.NewReader(payload), &out, &errOut); code != 0 || out.String() != "" {
		t.Fatalf("exit = %d, out = %q", code, out.String())
	}
}

func TestHookFailsOpenOnABadPayload(t *testing.T) {
	var out, errOut strings.Builder
	if code := Hook(t.TempDir(), strings.NewReader("{not json"), &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(errOut.String(), "allowing") {
		t.Fatalf("stderr = %q", errOut.String())
	}
	if strings.Count(errOut.String(), "\n") != 1 {
		t.Fatalf("an internal error logs one line, got %q", errOut.String())
	}
}
