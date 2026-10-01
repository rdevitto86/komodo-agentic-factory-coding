package guard

import "testing"

// TestAHeredocBodyIsNeverReadAsCommandsOrWrites proves a heredoc's body, carrying prose a commit
// message writes, never yields a git call, a write target, or any other finding of its own.
func TestAHeredocBodyIsNeverReadAsCommandsOrWrites(t *testing.T) {
	command := "git commit -F - <<'MSG'\n" +
		"See `go generate > /constants.ts` for the generated types.\n" +
		"MSG"
	items := nested(command)
	if len(items) != 1 {
		t.Fatalf("a heredoc body split into %d call(s), want the one git commit call: %v", len(items), items)
	}
	if got := items[0].words; len(got) < 2 || got[0] != "git" || got[1] != "commit" {
		t.Fatalf("the call was %v, want git commit", got)
	}
	if len(items[0].writes) != 0 {
		t.Fatalf("the heredoc body's own text was read as a write target: %v", items[0].writes)
	}
}

// TestAHeredocBodysBacktickFragmentIsNeverRefused proves the false positive the brief logged, a
// commit message's own example code read as a write outside the worktree, is now never found.
func TestAHeredocBodysBacktickFragmentIsNeverRefused(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	t.Setenv(RoleEnv, "builder")
	command := "git commit -F - <<'MSG'\n" +
		"See `go generate > /constants.ts` for the generated types.\n" +
		"MSG"
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": command}},
		DefaultPolicy(), "feat/x")
	if decision.Deny {
		t.Fatalf("a heredoc body's own example code was refused as a path: %v", decision.Findings)
	}
}

// TestAShellHeredocsBodyIsStillJudgedAsCommands proves a heredoc attached to a shell interpreter
// still reads its body as commands, since the shell will really run each one.
func TestAShellHeredocsBodyIsStillJudgedAsCommands(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	t.Setenv(RoleEnv, "builder")
	command := "bash <<'EOF'\necho x > ~/.claude/CLAUDE.md\nEOF"
	policy := DefaultPolicy()
	policy.ConfigPaths = append(policy.ConfigPaths, "~/.claude/**")
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": command}},
		policy, "feat/x")
	if !decision.Deny {
		t.Fatalf("a shell heredoc's write to a host config was allowed")
	}
}

// TestAnSHDashSHeredocsBodyIsStillJudgedAsCommands proves sh -s, reading its commands from stdin,
// still has its heredoc body read as commands, not skipped as prose.
func TestAnSHDashSHeredocsBodyIsStillJudgedAsCommands(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	command := "sh -s <<EOF\ngit push origin main\nEOF"
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": command}},
		DefaultPolicy(), "feat/x")
	if !decision.Deny {
		t.Fatalf("a sh -s heredoc's push to main was allowed")
	}
}

// TestAWrappedShellHeredocsBodyIsStillJudgedAsCommands proves a wrapper such as env or sudo in
// front of a shell still leaves its heredoc body read as commands.
func TestAWrappedShellHeredocsBodyIsStillJudgedAsCommands(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	for _, command := range []string{
		"env bash <<EOF\ngit push origin main\nEOF",
		"sudo sh <<EOF\ngit push origin main\nEOF",
	} {
		decision := Check(
			Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": command}},
			DefaultPolicy(), "feat/x")
		if !decision.Deny {
			t.Fatalf("%q: a wrapped shell heredoc's push to main was allowed", command)
		}
	}
}

// TestAGitCommitHeredocsBacktickFragmentStillPasses proves the logged false positive stays fixed
// once the consumer-aware skip lands: git, a prose consumer, still has its body skipped.
func TestAGitCommitHeredocsBacktickFragmentStillPasses(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	t.Setenv(RoleEnv, "builder")
	command := "git commit -F - <<'MSG'\n" +
		"See `go generate > /constants.ts` for the generated types.\n" +
		"MSG"
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": command}},
		DefaultPolicy(), "feat/x")
	if decision.Deny {
		t.Fatalf("the logged backtick fragment was refused again: %v", decision.Findings)
	}
}

// TestACatHeredocPipedIntoAShellIsStillJudged proves a prose consumer piped into a shell loses
// the allowlist, since its heredoc body is really about to run as that shell's commands.
func TestACatHeredocPipedIntoAShellIsStillJudged(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	command := "cat <<EOF | bash\ngit push origin main\nEOF"
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": command}},
		DefaultPolicy(), "feat/x")
	if !decision.Deny {
		t.Fatalf("a cat heredoc piped into bash was allowed")
	}
}

// TestAHeredocInsideCommandSubstitutionIsStillJudged proves a heredoc nested inside $(...) never
// qualifies for the skip, even naming a prose consumer, since it is really about to run.
func TestAHeredocInsideCommandSubstitutionIsStillJudged(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	command := `bash -c "$(cat <<EOF
git push origin main
EOF
)"`
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": command}},
		DefaultPolicy(), "feat/x")
	if !decision.Deny {
		t.Fatalf("a heredoc inside a command substitution was allowed")
	}
}

// TestACatHeredocRedirectedToAFileIsSkipped proves cat's heredoc body is skipped, including a
// line that merely looks like a redirect, once its own stdout goes straight to a file.
func TestACatHeredocRedirectedToAFileIsSkipped(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	t.Setenv(RoleEnv, "builder")
	command := "cat <<EOF > notes.md\nsome text with a > /x line in it\nEOF"
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": command}},
		DefaultPolicy(), "feat/x")
	if decision.Deny {
		t.Fatalf("a cat heredoc redirected to a file was refused: %v", decision.Findings)
	}
}

// TestACatHeredocIntoAProcessSubstitutionIsStillJudged proves a redirect into a process
// substitution is never a plain-path file write, so cat's body stays tokenized as commands.
func TestACatHeredocIntoAProcessSubstitutionIsStillJudged(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	command := "cat <<EOF > >(bash)\ngit push origin main\nEOF"
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": command}},
		DefaultPolicy(), "feat/x")
	if !decision.Deny {
		t.Fatalf("a cat heredoc redirected into a process substitution was allowed")
	}
}

// TestTwoHeredocsOnOneCommandSkipNeither proves a second heredoc on the same command line turns
// off the skip for both, so neither one's body is ever read for the other's delimiter.
func TestTwoHeredocsOnOneCommandSkipNeither(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	command := "git commit -F - <<EOF; bash <<EOF2\n" +
		"git push origin main\n" +
		"EOF2\n" +
		"msg\n" +
		"EOF"
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": command}},
		DefaultPolicy(), "feat/x")
	if !decision.Deny {
		t.Fatalf("two heredocs on one command let a push to main through")
	}
}
