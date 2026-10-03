package guard

import "testing"

// TestTeeCpMvAndSedInPlaceAreWriteTargets proves REQ-41's refusal holds for the common ways to
// edit a file besides a shell redirect: tee, cp and mv's destination, and sed -i's own operand.
func TestTeeCpMvAndSedInPlaceAreWriteTargets(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	t.Setenv(RoleEnv, "builder")
	for _, command := range []string{
		"sed -i s/a/b/ eval/golden.json",
		"tee docs/prd.md",
		"cp x eval/case.json",
		"mv x eval/case.json",
	} {
		decision := Check(
			Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": command}},
			DefaultPolicy(), "feat/x")
		if !decision.Deny {
			t.Fatalf("%q is allowed; want the write-target rule to catch it", command)
		}
	}
}

// TestSedInPlaceWithAnExplicitScriptFlagStillFindsItsFile proves sed -i -e 'script' file reads
// the file as the write target, not the script the -e flag already consumed.
func TestSedInPlaceWithAnExplicitScriptFlagStillFindsItsFile(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	t.Setenv(RoleEnv, "builder")
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root,
			ToolInput: map[string]any{"command": "sed -i -e s/a/b/ eval/golden.json"}},
		DefaultPolicy(), "feat/x")
	if !decision.Deny {
		t.Fatal("sed -i -e s/a/b/ eval/golden.json is allowed; want the file operand caught")
	}
}

// TestSedWithoutInPlaceIsNeverAWriteTarget proves a plain sed, printing to stdout, names no write
// target; only -i or --in-place edits a file.
func TestSedWithoutInPlaceIsNeverAWriteTarget(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	t.Setenv(RoleEnv, "builder")
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root,
			ToolInput: map[string]any{"command": "sed s/a/b/ eval/golden.json"}},
		DefaultPolicy(), "feat/x")
	if decision.Deny {
		t.Fatalf("a plain sed with no -i is refused: %v", decision.Findings)
	}
}

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

// TestAnInterpretersStdinScriptNamingACriticalPushIsRefused proves a python, node, ruby, or perl
// stdin body is judged for the shell command it hands a system call, such as a hidden push to main.
func TestAnInterpretersStdinScriptNamingACriticalPushIsRefused(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	for _, command := range []string{
		"python3 <<EOF\nimport os\nos.system('git push origin main')\nEOF",
		"node <<EOF\nrequire('child_process').execSync('git push origin main')\nEOF",
		"ruby <<EOF\nsystem('git push origin main')\nEOF",
		"perl <<EOF\nsystem('git push origin main')\nEOF",
	} {
		decision := Check(
			Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root, ToolInput: map[string]any{"command": command}},
			DefaultPolicy(), "feat/x")
		if !decision.Deny {
			t.Fatalf("%q is allowed; want the push to main caught inside its stdin script", command)
		}
	}
}

// TestALineSessionsInterpreterStdinScriptIsRefusedOutright proves a line session is refused any
// python, node, ruby, or perl body read from stdin, whether or not its content names a violation.
func TestALineSessionsInterpreterStdinScriptIsRefusedOutright(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	t.Setenv(RoleEnv, "builder")
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root,
			ToolInput: map[string]any{"command": "python3 <<EOF\nprint('hello')\nEOF"}},
		DefaultPolicy(), "feat/x")
	if !decision.Deny {
		t.Fatal("a line session's harmless python stdin script is allowed; want it refused outright")
	}
}

// TestTheOrchestratorsHarmlessInterpreterStdinScriptIsAllowed proves the orchestrator's stdin
// script is still free when it names no critical-ref push or host config path.
func TestTheOrchestratorsHarmlessInterpreterStdinScriptIsAllowed(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root,
			ToolInput: map[string]any{"command": "python3 <<EOF\nprint('hello')\nEOF"}},
		DefaultPolicy(), "feat/x")
	if decision.Deny {
		t.Fatalf("the orchestrator's harmless python stdin script is refused: %v", decision.Findings)
	}
}

// TestAnInterpreterGivenAScriptFileIsNeverReadAsStdin proves an interpreter run with a script
// file argument is left alone; only a bare interpreter reads its program from the heredoc.
func TestAnInterpreterGivenAScriptFileIsNeverReadAsStdin(t *testing.T) {
	registerFakeHost()
	root := worktree(t)
	decision := Check(
		Request{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: root,
			ToolInput: map[string]any{"command": "python3 run.py <<EOF\nignored\nEOF"}},
		DefaultPolicy(), "feat/x")
	if decision.Deny {
		t.Fatalf("python3 run.py with a heredoc is refused: %v", decision.Findings)
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
