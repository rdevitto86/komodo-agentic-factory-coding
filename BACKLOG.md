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

#### [TSK-01.1.10] Two Go 1.27+ "modernize" transforms were proposed for `standards-go` (embedded-field composite-literal flattening, `errors.As` → `errors.AsType[T]`) — neither is confirmed against actual Go release notes or the `gopls` modernize analyzer as of this review, so they must not be written into a shared skill unverified [P: L] [TODO]
* **SUB-01.1.10.1** confirm both transforms actually exist (check the Go release notes and `gopls`'s modernize analyzer docs for the version `go.mod` would need to declare) before writing anything — this is a verification step, not yet a documentation change
  * **Done when:** `grep -q '\[TSK-01.1.10\].*\[DONE\]' BACKLOG.md` — set only once, for each transform, a cited verification result against actual Go release notes and the `gopls` modernize analyzer docs is recorded (confirmed with the required Go version gate, or not confirmed), and if confirmed, `standards-go` contains the transform gated to that version

#### [TSK-01.1.12] Open policy question: should "reflow a pre-existing wrapped line to the current convention whenever it's revisited during unrelated work" be a standing, written exception to this toolkit's own no-scope-expansion rule in `AGENTS.md`? [P: L] [TODO]
* **SUB-01.1.12.1** decide whether formatting-only drive-by fixes get a blanket exception (and if so, where that exception is written down — `standards-go`, `AGENTS.md`, or both) versus staying subject to the existing scope rule; a decision for the user, not something this review resolves on its own

#### [TSK-01.1.19] `claude-code/hooks/reviewer_guard.py` has no regression test exercising its own fail-open path (malformed JSON, non-dict `tool_input`, non-string `file_path`/`cwd`) the way `git_guard.py`'s `F7` case does — manual probing confirms `main()`'s `except BaseException: sys.exit(0)` wrapper does fail open today, but nothing in `scripts/test-hooks.sh` locks that behavior in, so a future edit could silently regress it to a hang or an unintended deny [P: L] [DONE]
* **SUB-01.1.19.1** add a case sending malformed/malformed-typed JSON to `reviewer_guard.py` and asserting `allow`
  * **Done when:** `scripts/test-hooks.sh` gains that case and passes

#### [TSK-01.1.22] `lib/git.py`'s `repo_root()` only catches `(OSError, subprocess.SubprocessError)` around `subprocess.run(cwd=start, ...)`, so a non-str/bytes/PathLike `cwd` (e.g. an int from a malformed payload) raises an uncaught `TypeError` — surfaced by `scripts/test-hooks.sh`'s new R12 case, which only passes because `reviewer_guard.py`'s outer `except BaseException` in `__main__` rescues it; `git_guard.py` calls the same `repo_root` and fails closed by design, so the same uncaught `TypeError` there depends entirely on how far up its own exception handling reaches [P: L] [TODO]
* **SUB-01.1.22.1** widen `repo_root()`'s except clause (or validate `start` before the call) so the failure mode is deliberate rather than incidental to whichever caller's outer handler happens to catch it first
  * **Done when:** `repo_root()` handles a non-path `start` without relying on caller-level rescue · S → `/assess-bugs claude-code/hooks/lib/git.py` reports it clear

#### [TSK-01.1.21] `scripts/test-hooks.sh` pins cwd two different ways for the same purpose [P: L] [TODO]
* **SUB-01.1.21.1** `bash_case`/`smoke_case` set a top-level `"cwd"` field in the JSON payload, while the pre-existing `bash_case_at` relies on `git_guard.py`'s `payload.get("cwd") or os.getcwd()` fallback by `cd`-ing the subshell into the fixture dir before invoking the hook — both work, but the file now carries two mechanisms for "pin the effective cwd" instead of one
  * **Done when:** `scripts/test-hooks.sh` uses one cwd-pinning mechanism and `bash scripts/test-hooks.sh` passes · S → `/assess-simplify scripts/test-hooks.sh` reports it clear

### [TG-01.2] Token Efficiency
* **Target Release:** V1
* **Context (2026-09-02, updated post-shipped review-fork/AGENTS.md-trim/bundled-skill-collapse/git-pr-create-diff-stat/workflow-loop-compaction-cap/config-accessibility-fix/git_guard-backtick-fix/git_guard-dedup work):** the always-on budget is healthy (`scripts/validate.sh`: 1,155 of 2,000 tokens). Root `AGENTS.md` was cut from 7,157 to 1,416 tokens, `assess-bugs`/`assess-security`/`assess-simplify`/`backlog-audit` now run as `reviewer`/`workflow-implementer` forks instead of the orchestrator window, `git-pr-create`'s P4 read now uses `--stat` instead of the full diff, `workflow-loop/SKILL.md` was trimmed under the compaction re-attach cap with a `scripts/validate.sh` check enforcing it, `config-accessibility` was shrunk with its dead `CLAUDE.local.md` reference fixed, and the `git_guard.py` backtick false-positive was fixed alongside four Critical substitution-scanner bypasses it surfaced, plus a follow-up dedup refactor (all shipped; `backlog-audit` renumbered the remaining task below). Remaining scope: further adversarial hardening of `git_guard.py`'s shell-parsing.

#### [TSK-01.2.1] git_guard.py's shell-parsing gets a bounded hardening pass against two named untested edge-case shapes [P: L] [TODO]
* **SUB-01.2.1.1** the 2026-09-01 band that fixed the `git_guard.py` backtick false-positive (and, in review, caught and fixed three unrelated Critical bypasses along the way — a `#`-comment quote-state swallow, missing backtick/`$()` substitution detection entirely, and an escaped-nested-backtick gap) deliberately stopped hardening `claude-code/hooks/git_guard.py`'s `extract_substitutions`/`split_segments` once those four were closed, rather than continuing to chase further shell-quoting edge cases in the same pass — hand-parsing arbitrary POSIX shell quoting/escaping/substitution semantics to zero residual risk is open-ended, the same reasoning already applied to the risk-accepted `grep`/`sed`/`awk`/`curl` secret-exfiltration gap in `CHANGELOG.md`'s `[0.37.2]` entry. Untested-but-plausible remaining edge cases: `$(...)` containing backslash-escaped backticks, and deeper mixed single/double-quote/substitution nesting. `scripts/test-hooks.sh` now covers `G90`-`G98` for the shapes found so far. Scope is deliberately bounded to these two named shapes, not a fresh open-ended grammar audit.
  * **Done when:** `scripts/test-hooks.sh` gains covering test cases for (1) backslash-escaped backticks inside `$(...)` and (2) deeper mixed single/double-quote/substitution nesting against `extract_substitutions`/`split_segments`/`find_backtick_end`/`find_paren_end` (fixing any real gap found), `bash scripts/test-hooks.sh` passes, and any edge case still unaddressed after this bounded pass is explicitly risk-accepted in `CHANGELOG.md` matching the existing `[0.37.2]` pattern

### [TG-01.3] Skill Namespace & Assess Refactor
* **Target Release:** V1

### [TG-01.4] Workflow Loop & Hook Reliability
* **Target Release:** V1

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

_Nothing archived yet._
