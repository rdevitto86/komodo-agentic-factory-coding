# Project Backlog

## Convention Legend
* **Priority Tagging:** `[C]` Critical | `[H]` High | `[M]` Medium | `[L]` Low
* **Status Indicators:** `[TODO]` | `[IN_PROGRESS]` | `[BLOCKED]` | `[DONE]`
* **Hierarchy ID:** `EPIC-XX` -> `TG-XX.Y` (Task Group) -> `TSK-XX.Y.Z` (Task) -> `SUB-XX.Y.Z.N` (Subtask)

Format and rules live in the `backlog-modify` skill — load it before editing this file. `[DONE]` tasks stay until a sweep (`/backlog-audit`) moves them to `CHANGELOG.md` and removes them — this is not a log to hand-curate.

---

## [EPIC-01] Now, V1
*Goal: keep this toolkit's own hooks, docs, and skills correct and internally consistent.*

### [TG-01.1] Cross-Cutting
* **Target Release:** V1

#### [TSK-01.1.2] Bridge: `num_ctx` truncation on large summarizer payloads [P: M] [BLOCKED]
* **Blocked By:** `external`
  * **Reason (2026-08-28):** No file to edit in this repo — the bridge server (`generateRequest`, `agents.go`) lives in the separate `~/.komodo/bridge` deploy; `bridges/komodo-bridge/` here holds only prompt files and docs.
  * **Citation:** `bridges/komodo-bridge/` (prompt files and docs only, no Go source)
  * **Recheck:** bridge source is vendored into or made reachable from this repo — `find bridges/komodo-bridge -iname '*.go'` returns a match
* **SUB-01.1.2.1** fix `generateRequest`'s payload truncation against `num_ctx` in the bridge server once its source is reachable from this repo
  * **Done when:** a large summarizer payload no longer silently truncates against `num_ctx` in `~/.komodo/bridge`

#### [TSK-01.1.20] `docs/design-decisions.md:128`'s "no SKILL.md is invisible" enumeration omits `standards-c` [P: L] [DONE]
* **SUB-01.1.20.1** the enumeration (`standards-gcp`/`standards-azure`/`standards-rust`/`standards-csharp`/`standards-hardware`/`standards-cpp`) omits `standards-c` even though `standards-c` is itself parked as `SKILL.md.off` — the two `SUB-01.1.7.5`/`SUB-01.1.8.4` citations to line 128 as the source of that fact point at a line whose own list doesn't name `standards-c`, while line 158 of the same doc does
  * **Done when:** `docs/design-decisions.md`'s line-128 enumeration includes `standards-c`, consistent with line 158 · S → `/assess-bugs docs/design-decisions.md` reports it clear

#### [TSK-01.1.10] Two Go "modernize" transforms proposed for `standards-go` (embedded-field composite-literal flattening, `errors.As` → `errors.AsType[T]`) needed verification against real Go/gopls release material before being written into a shared skill [P: L] [DONE]
* **SUB-01.1.10.1** confirm both transforms actually exist (check the Go release notes and `gopls`'s modernize analyzer docs for the version `go.mod` would need to declare) before writing anything — this is a verification step, not yet a documentation change
  * **Done when:** `grep -q '\[TSK-01.1.10\].*\[DONE\]' BACKLOG.md` — set only once, for each transform, a cited verification result against actual Go release notes and the `gopls` modernize analyzer docs is recorded (confirmed with the required Go version gate, or not confirmed), and if confirmed, `standards-go` contains the transform gated to that version
  * **Verification result (both CONFIRMED):** (1) `errors.AsType[T]` — a real generic replacement for `errors.As` (`func AsType[E error](err error) (E, bool)`), added to the standard-library `errors` package in **Go 1.26**; walks the error tree via `Unwrap` without reflection. `gopls`'s `errorsastype` modernize analyzer rewrites eligible `errors.As` calls to it. Not deprecating `errors.As`. (2) Embedded-field composite-literal flattening — real; **Go 1.27** allows a composite literal to directly initialize a field promoted from an embedded struct type (`T{U: U{x: 1}}` → `T{x: 1}`), and `gopls`'s `embedlit` modernize analyzer flags/rewrites the old nested form. Confirmed via `go.dev/gopls/analyzers` and Go 1.26 release coverage of `errors.AsType`; a prior draft of this task's verification pass reached the opposite (unconfirmed) conclusion from model recall alone without a live source check — corrected here against actual documentation. Both now documented in `claude-code/skills/standards-go/SKILL.md`, each gated to its required `go.mod` floor.

#### [TSK-01.1.12] Open policy question: should "reflow a pre-existing wrapped line to the current convention whenever it's revisited during unrelated work" be a standing, written exception to this toolkit's own no-scope-expansion rule in `AGENTS.md`? [P: L] [TODO]
* **SUB-01.1.12.1** decide whether formatting-only drive-by fixes get a blanket exception (and if so, where that exception is written down — `standards-go`, `AGENTS.md`, or both) versus staying subject to the existing scope rule; a decision for the user, not something this review resolves on its own

#### [TSK-01.1.19] `claude-code/hooks/reviewer_guard.py` has no regression test exercising its own fail-open path (malformed JSON, non-dict `tool_input`, non-string `file_path`/`cwd`) the way `git_guard.py`'s `F7` case does — manual probing confirms `main()`'s `except BaseException: sys.exit(0)` wrapper does fail open today, but nothing in `scripts/test-hooks.sh` locks that behavior in, so a future edit could silently regress it to a hang or an unintended deny [P: L] [DONE]
* **SUB-01.1.19.1** add a case sending malformed/malformed-typed JSON to `reviewer_guard.py` and asserting `allow`
  * **Done when:** `scripts/test-hooks.sh` gains that case and passes

#### [TSK-01.1.22] `lib/git.py`'s `repo_root()` only catches `(OSError, subprocess.SubprocessError)` around `subprocess.run(cwd=start, ...)`, so a non-str/bytes/PathLike `cwd` (e.g. an int from a malformed payload) raises an uncaught `TypeError` — surfaced by `scripts/test-hooks.sh`'s new R12 case, which only passes because `reviewer_guard.py`'s outer `except BaseException` in `__main__` rescues it; `git_guard.py` calls the same `repo_root` and fails closed by design, so the same uncaught `TypeError` there depends entirely on how far up its own exception handling reaches [P: L] [TODO]
* **SUB-01.1.22.1** widen `repo_root()`'s except clause (or validate `start` before the call) so the failure mode is deliberate rather than incidental to whichever caller's outer handler happens to catch it first
  * **Done when:** `repo_root()` handles a non-path `start` without relying on caller-level rescue · S → `/assess-bugs claude-code/hooks/lib/git.py` reports it clear

#### [TSK-01.1.23] `standards-go/SKILL.md`'s two new TSK-01.1.10 bullets hardcode `Go 1.27+`/`Go 1.26+` version numbers [P: L] [TODO]
* **SUB-01.1.23.1** the same file's own "Version floor is whatever `go.mod` declares. Read it; never assume a release." convention (line 18), and root `AGENTS.md`'s "No static references" table, whose worked example is literally "Instead of `Go 1.26` write the floor `go.mod` declares" — but the two new bullets (`embedlit` composite-literal flattening, `errors.AsType[T]`) name the literal versions `Go 1.27+`/`Go 1.26+` directly rather than phrasing the gate relative to the floor
  * **Done when:** the two bullets phrase their version gate without a hardcoded release number (e.g. "once `go.mod`'s floor reaches the release that added it"), consistent with line 18 and `AGENTS.md`'s own example · S → `/assess-bugs claude-code/skills/standards-go/SKILL.md` reports it clear

#### [TSK-01.1.21] `scripts/test-hooks.sh` pins cwd two different ways for the same purpose [P: L] [DONE]
* **SUB-01.1.21.1** `bash_case`/`smoke_case` set a top-level `"cwd"` field in the JSON payload, while the pre-existing `bash_case_at` relies on `git_guard.py`'s `payload.get("cwd") or os.getcwd()` fallback by `cd`-ing the subshell into the fixture dir before invoking the hook — both work, but the file now carries two mechanisms for "pin the effective cwd" instead of one
  * **Done when:** `scripts/test-hooks.sh` uses one cwd-pinning mechanism and `bash scripts/test-hooks.sh` passes · S → `/assess-simplify scripts/test-hooks.sh` reports it clear

#### [TSK-01.1.24] `git_guard.py`'s `extract_substitutions` fix for TSK-01.2.1's escaped-backtick-in-`$()` bypass over-applies the unescape, denying inert literal-backtick text that real bash never re-parses [P: H] [DONE]
* **SUB-01.1.24.1** fixed: `extract_substitutions` now tracks each `$()` capture's enclosing top-level segment (reset at the same `;`/`\n`/`|`/`&&`/`||` boundaries `split_segments` uses) and only unescapes a nested backtick when `segment_wants_reparse` finds the segment's leading command is `eval`, or a `SHELL_WRAPPERS` command with a literal `-c` token — the two shapes that actually re-parse the captured text a second time. A bare backtick capture (`` `...` ``) is untouched — `\`` there is always live nesting syntax regardless of eval, so it keeps unescaping unconditionally. `echo "$(echo \`echo git push --force\` is dangerous)"` (no re-parsing wrapper) is now allowed; `eval "$(echo \`git push --force\`)"` (G151) still denies
  * **Done when:** `git_guard.py` only treats an escaped backtick inside a `$()` capture as live when the capture is itself subject to a re-parse (e.g. its result flows into `eval`/`sh -c`/similar), so `echo "$(echo \`git push --force\` is dangerous)"` with no re-parsing wrapper is allowed while `eval "$(echo \`git push --force\`)"` (G151) still denies · S → `/assess-bugs claude-code/hooks/git_guard.py` reports it clear — met: `bash scripts/test-hooks.sh` passes, G151 denies, new G153 confirms the inert form is allowed

#### [TSK-01.1.25] `git_guard.py`'s `segment_wants_reparse` only recognizes `eval` as a literal leading token, missing indirect re-parse paths [P: H] [DONE]
* **SUB-01.1.25.1** fixed the statically-decidable half: `segment_wants_reparse` now strips a `command` prefix (with or without its value-less flags, via `strip_leading_flags`/`PASSTHROUGH_VALUE_FLAGS["command"]`) and re-checks the token after it for `eval`/`SHELL_WRAPPERS`+`-c`, so `command eval "$(echo x\`git push origin main\`y)"` denies the same as a bare `eval` would (new `G154` in `scripts/test-hooks.sh`). The variable/alias/function-indirection shape (`RUN=eval; $RUN "$(...)"`) is out of scope for a line-local scanner — it needs cross-statement data-flow tracking — and is risk-accepted in `CHANGELOG.md`'s `[Unreleased]` section rather than attempted here
  * **Done when:** `segment_wants_reparse` (or its caller) detects `eval`/`sh -c`-equivalents reached through a recognized passthrough wrapper (e.g. `command`, `builtin`) and flags an unresolvable indirect leading command (e.g. a bare `$VAR` in command position) as reparse-risk rather than silently passing it through unescaped · S → `/assess-security claude-code/hooks/git_guard.py` reports it clear — met for the `command`-prefix case: `bash scripts/test-hooks.sh` passes including `G154`; the variable/alias/function-indirection case is risk-accepted, not fixed

#### [TSK-01.1.26] `segment_wants_reparse`'s `command`-prefix handling treats `command -v`/`-V` (existence/type checks that never execute their argument) the same as `command eval` (which does execute it), producing a verified false-positive deny [P: M] [TODO]
* **SUB-01.1.26.1** `PASSTHROUGH_VALUE_FLAGS["command"]` is `()`, so `strip_leading_flags` walks past `-v`/`-V` as unrecognized flags without excluding them; `segment_wants_reparse('command -v eval ')` returns `True` even though real bash's `command -v eval` only reports whether `eval` exists and never re-parses its arguments — confirmed end-to-end: `analyze({'tool_name': 'Bash', 'tool_input': {'command': 'command -v eval "$(echo \\`git push --force\\`)"'}})` returns `['git push --force rewrites published history']`, denying a command bash would execute as a no-op existence check. `CHANGELOG.md`'s `[Unreleased]` entry ("looks past a `command` prefix ... at what it actually runs") doesn't mention this gap and overstates precision for check-only flags
  * **Done when:** `segment_wants_reparse` does not classify a `command -v`/`command -V` invocation as reparse-True, and the `git_guard.py` false positive above is allowed · S → `/assess-security claude-code/hooks/git_guard.py` reports it clear

#### [TSK-01.1.27] `extract_substitutions`'s new segment-boundary tracking never advances `segment_start` past a `#`-comment's closing newline, so `segment_wants_reparse` sees stale text from before the comment and misses `eval`/`-c` on the far side of it — a verified `git push --force` bypass [P: C] [DONE]
* **SUB-01.1.27.1** the `in_comment` branch added alongside `segment_start` (mirroring `split_segments`'s existing comment handling) consumes the comment's closing `\n` without resetting `segment_start` the way `split_segments`'s own comment branch does (`segments.append(...); seg_start = index`) — so when a `$()` capture appears in a segment that starts right after a comment line, `segment_wants_reparse` is handed a prefix stretching back into the comment and the segment before it, its first token is whatever led that earlier segment (not `eval`), and it returns `False`. Confirmed: `eval "$(echo \`git push --force\`)"` alone denies, but `echo hi # comment\neval "$(echo \`git push --force\`)"` — logically identical, `eval` still re-parses the capture — returns `[]` (allowed), letting the escaped-backtick `git push --force` execute undetected
  * **Done when:** `extract_substitutions` resets `segment_start` to the position right after a comment's closing newline (matching `split_segments`'s comment-exit behavior), `echo hi # comment\neval "$(echo \`git push --force\`)"` denies, and a new `scripts/test-hooks.sh` case locks it in · S → `/assess-security claude-code/hooks/git_guard.py` reports it clear

### [TG-01.2] Token Efficiency
* **Target Release:** V1
* **Context (2026-09-02, updated post-shipped review-fork/AGENTS.md-trim/bundled-skill-collapse/git-pr-create-diff-stat/workflow-loop-compaction-cap/config-accessibility-fix/git_guard-backtick-fix/git_guard-dedup work):** the always-on budget is healthy (`scripts/validate.sh`: 1,155 of 2,000 tokens). Root `AGENTS.md` was cut from 7,157 to 1,416 tokens, `assess-bugs`/`assess-security`/`assess-simplify`/`backlog-audit` now run as `reviewer`/`workflow-implementer` forks instead of the orchestrator window, `git-pr-create`'s P4 read now uses `--stat` instead of the full diff, `workflow-loop/SKILL.md` was trimmed under the compaction re-attach cap with a `scripts/validate.sh` check enforcing it, `config-accessibility` was shrunk with its dead `CLAUDE.local.md` reference fixed, and the `git_guard.py` backtick false-positive was fixed alongside four Critical substitution-scanner bypasses it surfaced, plus a follow-up dedup refactor (all shipped; `backlog-audit` renumbered the remaining task below). Remaining scope: further adversarial hardening of `git_guard.py`'s shell-parsing.

#### [TSK-01.2.1] git_guard.py's shell-parsing gets a bounded hardening pass against two named untested edge-case shapes [P: L] [DONE]
* **SUB-01.2.1.1** bounded pass complete against the two named shapes. Shape 1 (`$(...)` containing a backslash-escaped backtick, e.g. `eval "$(echo \`git push --force\`)"`) was a real bypass: a bare, un-re-parsed `$(echo \`git push --force\`)` is inert in real bash (the escaped backtick stays literal, matching `git_guard.py`'s prior behavior), but once that substitution's output is re-parsed by `eval` (or an equivalent second pass), the backslash is stripped a second time and the backtick becomes a live nested substitution — `extract_substitutions` wasn't modeling that, so the hidden `git push --force` slipped through. Fixed by unescaping a `$()` capture the same way a backtick capture already was, in both the top-level and double-quote branches of `extract_substitutions`, so the recursive scan sees the same real nested substitution a re-parse would produce. Shape 2 (a single-quoted segment containing a double-quoted `$()`-substitution, or vice versa, nested 2+ levels) was already handled correctly — confirmed with `sh -c 'echo "$(git push --force)"'` nested inside an outer `$()`, denied without any code change. `scripts/test-hooks.sh` now covers both via `G151`/`G152`; no residual gap needed a `CHANGELOG.md` risk-acceptance entry. Final state, closing this band's trail: the follow-on hardening this pass's own residual surfaced (TSK-01.1.24, then TSK-01.1.25's `command`-prefix gap) is now closed, and TSK-01.1.25's last residual — `eval`/`sh -c` reached through variable, alias, or shell-function indirection rather than a literal leading token — is risk-accepted in `CHANGELOG.md`'s `[Unreleased]` section, since a line-local scanner can't do the cross-statement data-flow tracking that shape would need.
  * **Done when:** `scripts/test-hooks.sh` gains covering test cases for (1) backslash-escaped backticks inside `$(...)` and (2) deeper mixed single/double-quote/substitution nesting against `extract_substitutions`/`split_segments`/`find_backtick_end`/`find_paren_end` (fixing any real gap found), `bash scripts/test-hooks.sh` passes, and any edge case still unaddressed after this bounded pass is explicitly risk-accepted in `CHANGELOG.md` matching the existing `[0.37.2]` pattern — met: both `G151`/`G152` pass, no risk-acceptance entry needed

### [TG-01.3] Skill Namespace & Assess Refactor
* **Target Release:** V1

### [TG-01.4] Workflow Loop & Hook Reliability
* **Target Release:** V1

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

_Nothing archived yet._
