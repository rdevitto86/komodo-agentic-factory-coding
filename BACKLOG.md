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

#### [TSK-01.1.12] Open policy question: should "reflow a pre-existing wrapped line to the current convention whenever it's revisited during unrelated work" be a standing, written exception to this toolkit's own no-scope-expansion rule in `AGENTS.md`? [P: L] [TODO]
* **SUB-01.1.12.1** decide whether formatting-only drive-by fixes get a blanket exception (and if so, where that exception is written down — `standards-go`, `AGENTS.md`, or both) versus staying subject to the existing scope rule; a decision for the user, not something this review resolves on its own

#### [TSK-01.1.13] `lib/git.py`'s `repo_root()` only catches `(OSError, subprocess.SubprocessError)` around `subprocess.run(cwd=start, ...)`, so a non-str/bytes/PathLike `cwd` (e.g. an int from a malformed payload) raises an uncaught `TypeError` — surfaced by `scripts/test-hooks.sh`'s new R12 case, which only passes because `reviewer_guard.py`'s outer `except BaseException` in `__main__` rescues it; `git_guard.py` calls the same `repo_root` and fails closed by design, so the same uncaught `TypeError` there depends entirely on how far up its own exception handling reaches [P: L] [TODO]
* **SUB-01.1.13.1** widen `repo_root()`'s except clause (or validate `start` before the call) so the failure mode is deliberate rather than incidental to whichever caller's outer handler happens to catch it first
  * **Done when:** `repo_root()` handles a non-path `start` without relying on caller-level rescue · S → `/assess-bugs claude-code/hooks/lib/git.py` reports it clear

#### [TSK-01.1.14] `standards-go/SKILL.md`'s two new "modernize" transform bullets hardcode `Go 1.27+`/`Go 1.26+` version numbers [P: L] [TODO]
* **SUB-01.1.14.1** the same file's own "Version floor is whatever `go.mod` declares. Read it; never assume a release." convention (line 18), and root `AGENTS.md`'s "No static references" table, whose worked example is literally "Instead of `Go 1.26` write the floor `go.mod` declares" — but the two new bullets (`embedlit` composite-literal flattening, `errors.AsType[T]`) name the literal versions `Go 1.27+`/`Go 1.26+` directly rather than phrasing the gate relative to the floor
  * **Done when:** the two bullets phrase their version gate without a hardcoded release number (e.g. "once `go.mod`'s floor reaches the release that added it"), consistent with line 18 and `AGENTS.md`'s own example · S → `/assess-bugs claude-code/skills/standards-go/SKILL.md` reports it clear

#### [TSK-01.1.15] `segment_wants_reparse`'s `command`-prefix handling treats `command -v`/`-V` (existence/type checks that never execute their argument) the same as `command eval` (which does execute it), producing a verified false-positive deny [P: M] [TODO]
* **SUB-01.1.15.1** `PASSTHROUGH_VALUE_FLAGS["command"]` is `()`, so `strip_leading_flags` walks past `-v`/`-V` as unrecognized flags without excluding them; `segment_wants_reparse('command -v eval ')` returns `True` even though real bash's `command -v eval` only reports whether `eval` exists and never re-parses its arguments — confirmed end-to-end: `analyze({'tool_name': 'Bash', 'tool_input': {'command': 'command -v eval "$(echo \\`git push --force\\`)"'}})` returns `['git push --force rewrites published history']`, denying a command bash would execute as a no-op existence check. `CHANGELOG.md`'s `[Unreleased]` entry ("looks past a `command` prefix ... at what it actually runs") doesn't mention this gap and overstates precision for check-only flags
  * **Done when:** `segment_wants_reparse` does not classify a `command -v`/`command -V` invocation as reparse-True, and the `git_guard.py` false positive above is allowed · S → `/assess-security claude-code/hooks/git_guard.py` reports it clear

#### [TSK-01.1.16] `extract_substitutions` grew its own second copy of `split_segments`'s quote/comment/boundary state machine instead of reusing it [P: M] [TODO]
* **SUB-01.1.16.1** `extract_substitutions` added `segment_start` tracking (squote/dquote/comment state, `&&`/`||`/`SEGMENT_BOUNDARY_CHARS` resets) purely so `segment_wants_reparse` can recover "what command led the enclosing segment" — but `split_segments` already walks the exact same command string tracking the exact same quote/comment/boundary state to produce that same segment slicing, as a separate pass immediately after. The two hand-rolled lexers must now be kept in lockstep by hand (a comment-boundary reset was once added to one copy and missed in the other) with no shared helper enforcing it. Separately, the two `$()` capture sites inside `extract_substitutions` (the double-quote branch and the top-level branch) repeat the identical three-line `unescape = ... if segment_wants_reparse(...) else None; index = capture_and_mask(...); continue` block verbatim.
  * **Done when:** `extract_substitutions` derives its segment/leading-command context from `split_segments`'s own boundary logic (a shared helper, or `split_segments` output reused) rather than a second independent state machine, and the two duplicated `$()` capture blocks collapse to one shared call · S → `/assess-simplify claude-code/hooks/git_guard.py` reports it clear

### [TG-01.2] Token Efficiency
* **Target Release:** V1
* **Context (2026-09-09):** the always-on budget stays healthy (`scripts/validate.sh` tracks the figure). The bounded `git_guard.py` shell-parsing hardening pass this task group's last open task tracked is complete — the four Critical substitution-scanner bypasses it surfaced (comment-boundary reset, `command`-prefix reparse detection, and the two above) are closed or filed as their own `TG-01.1` tasks; the `command -v`/`-V` false-positive and the `extract_substitutions`/`split_segments` dedup remain open there. No task currently open in this task group.

### [TG-01.3] Skill Namespace & Assess Refactor
* **Target Release:** V1

### [TG-01.4] Workflow Loop & Hook Reliability
* **Target Release:** V1

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

_Nothing archived yet._
