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

- [L] `docs/design-decisions.md:128`'s "no SKILL.md is invisible" enumeration (`standards-gcp`/`standards-azure`/`standards-rust`/`standards-csharp`/`standards-hardware`/`standards-cpp`) omits `standards-c` even though `standards-c` is itself parked as `SKILL.md.off` — the two `SUB-01.1.7.5`/`SUB-01.1.8.4` citations to line 128 as the source of that fact point at a line whose own list doesn't name `standards-c`, while line 158 of the same doc does · S → `/assess-bugs docs/design-decisions.md` reports it clear

#### [TSK-01.1.10] Two Go 1.27+ "modernize" transforms were proposed for `standards-go` (embedded-field composite-literal flattening, `errors.As` → `errors.AsType[T]`) — neither is confirmed against actual Go release notes or the `gopls` modernize analyzer as of this review, so they must not be written into a shared skill unverified [P: L] [TODO]
* **SUB-01.1.10.1** confirm both transforms actually exist (check the Go release notes and `gopls`'s modernize analyzer docs for the version `go.mod` would need to declare) before writing anything — this is a verification step, not yet a documentation change

#### [TSK-01.1.12] Open policy question: should "reflow a pre-existing wrapped line to the current convention whenever it's revisited during unrelated work" be a standing, written exception to this toolkit's own no-scope-expansion rule in `AGENTS.md`? [P: L] [TODO]
* **SUB-01.1.12.1** decide whether formatting-only drive-by fixes get a blanket exception (and if so, where that exception is written down — `standards-go`, `AGENTS.md`, or both) versus staying subject to the existing scope rule; a decision for the user, not something this review resolves on its own

#### [TSK-01.1.19] `claude-code/hooks/reviewer_guard.py` has no regression test exercising its own fail-open path (malformed JSON, non-dict `tool_input`, non-string `file_path`/`cwd`) the way `git_guard.py`'s `F7` case does — manual probing confirms `main()`'s `except BaseException: sys.exit(0)` wrapper does fail open today, but nothing in `scripts/test-hooks.sh` locks that behavior in, so a future edit could silently regress it to a hang or an unintended deny [P: L] [TODO]
* **SUB-01.1.19.1** add a case sending malformed/malformed-typed JSON to `reviewer_guard.py` and asserting `allow`
  * **Done when:** `scripts/test-hooks.sh` gains that case and passes

- [M] `scripts/test-hooks.sh`'s plain `bash_case`/`smoke_case` invocations (`S1`/`S5`-`S7`, `G100`, `G125`-`G131`, `G134`-`G135`, etc.) send no `cwd` in the payload, so `git_guard.py`'s new cwd-derived `is_guarded_path`/`repo_root_of` resolves against `os.getcwd()` of whatever process invokes the test script — these deny assertions only pass because the suite happens to be run from inside this repo's own git worktree; run from a checkout without `.git` (a tarball, a stripped CI workspace) or any other non-repo cwd, and every one of them silently flips from deny to allow-then-fail, unlike the new `bash_case_at` cases that pin cwd explicitly · S → `/assess-bugs scripts/test-hooks.sh` reports it clear

### [TG-01.2] Token Efficiency
* **Target Release:** V1
* **Context (2026-09-02, updated post-shipped review-fork/AGENTS.md-trim/bundled-skill-collapse/git-pr-create-diff-stat/workflow-loop-compaction-cap/config-accessibility-fix/git_guard-backtick-fix/git_guard-dedup work):** the always-on budget is healthy (`scripts/validate.sh`: 1,155 of 2,000 tokens). Root `AGENTS.md` was cut from 7,157 to 1,416 tokens, `assess-bugs`/`assess-security`/`assess-simplify`/`backlog-audit` now run as `reviewer`/`workflow-implementer` forks instead of the orchestrator window, `git-pr-create`'s P4 read now uses `--stat` instead of the full diff, `workflow-loop/SKILL.md` was trimmed under the compaction re-attach cap with a `scripts/validate.sh` check enforcing it, `config-accessibility` was shrunk with its dead `CLAUDE.local.md` reference fixed, and the `git_guard.py` backtick false-positive was fixed alongside four Critical substitution-scanner bypasses it surfaced, plus a follow-up dedup refactor (all shipped; `backlog-audit` renumbered the remaining task below). Remaining scope: further adversarial hardening of `git_guard.py`'s shell-parsing.

#### [TSK-01.2.1] git_guard.py's shell-parsing is not exhaustively adversarial-hardened against every quoting/escaping/substitution combination [P: L] [TODO]
* **SUB-01.2.1.1** the 2026-09-01 band that fixed the `git_guard.py` backtick false-positive (and, in review, caught and fixed three unrelated Critical bypasses along the way — a `#`-comment quote-state swallow, missing backtick/`$()` substitution detection entirely, and an escaped-nested-backtick gap) deliberately stopped hardening `claude-code/hooks/git_guard.py`'s `extract_substitutions`/`split_segments` once those four were closed, rather than continuing to chase further shell-quoting edge cases in the same pass — hand-parsing arbitrary POSIX shell quoting/escaping/substitution semantics to zero residual risk is open-ended, the same reasoning already applied to the risk-accepted `grep`/`sed`/`awk`/`curl` secret-exfiltration gap in `CHANGELOG.md`'s `[0.37.2]` entry. Untested-but-plausible remaining edge cases: `$(...)` containing backslash-escaped backticks, deeper mixed single/double-quote/substitution nesting, and other exotic POSIX escaping shapes. `scripts/test-hooks.sh` now covers `G90`-`G98` for the shapes found so far.
  * **Done when:** a dedicated, systematic pass (ideally against a real shell-grammar reference or fuzzer, not ad hoc cases) audits `extract_substitutions`/`split_segments`/`find_backtick_end`/`find_paren_end` against the POSIX shell quoting grammar and either closes every gap found or explicitly risk-accepts each one in `CHANGELOG.md`, matching the existing `[0.37.2]` pattern

#### [TSK-01.2.2] `scripts/validate.sh`'s budget pass counts every non-disabled skill's listing cost identically, without exempting a skill that carries `paths:` — per `AGENTS.md`, a `paths:`-gated skill is not in the base always-on listing until a matching file is touched, so the reported total overstates the real always-on cost (25 of 61 `SKILL.md` files currently carry `paths:`) [P: M] [TODO]
* **SUB-01.2.2.1** in the budget loop (`scripts/validate.sh`'s embedded Python, around the `for entry in sorted(os.listdir(skills_dir))` loop), skip or separately report a skill whose frontmatter has a `paths:` key, since it isn't part of the true always-on total the `BUDGET` ceiling is meant to gate
  * **Done when:** `bash scripts/validate.sh` reports the budget total with `paths:`-gated skills excluded (or broken out as a separate, non-gated figure), matching the always-on-listing definition in `AGENTS.md`

### [TG-01.3] Skill Namespace & Assess Refactor
* **Target Release:** V1

#### [TSK-01.3.1] `repo-assess` cannot execute as written — its Process step 4 mandates invoking `assess-readiness`, `assess-code-quality` and `assess-testing`, and all three carry `disable-model-invocation: true`, which `docs/design-decisions.md` itself states is "the only thing that controls cross-skill reachability"; the Skill tool refuses them outright ("cannot be used with Skill tool due to disable-model-invocation… Do not replicate this skill's workflow by other means"), so 3 of the 9 category scores are unobtainable and the composite — defined as the unweighted average of all nine — cannot be computed. Confirmed live this session: `Skill(assess-readiness)` returned that refusal, and only the six reachable sub-assessments ran [P: C] [TODO]
* **SUB-01.3.1.1** decide the resolution and record it in `docs/design-decisions.md`: either drop `disable-model-invocation: true` from the three sub-skills (keeping the human-decision gate only on `repo-assess` itself, which is the one the user actually types), or rewrite `repo-assess` to score the six reachable dimensions and instruct the user to run the other three by hand — the first preserves the composite, the second preserves the typed-only rule, and only the user can pick which of those two properties matters more
  * **Done when:** `docs/design-decisions.md` names the chosen resolution and `claude-code/skills/repo-assess/SKILL.md` matches it
* **SUB-01.3.1.2** add a `scripts/validate.sh` check that fails when a skill body names a sibling skill it cannot reach — a `/skill-name` or `` `skill-name` `` invocation instruction pointing at a skill whose frontmatter carries `disable-model-invocation: true`
  * **Done when:** `bash scripts/validate.sh` fails on a deliberately-introduced unreachable cross-skill invocation and passes once it is removed

### [TG-01.4] Workflow Loop & Hook Reliability
* **Target Release:** V1
* **Context (2026-09-08):** Two independent sessions surfaced evidence that the loop and the hook layer both lack circuit breakers. A single-file review chain ran 8 forked adversarial passes for work scoped as 3 bugs (this session, ~65% of a Max budget). A `workflow-loop` skill diagnosis from a different session found 4 concrete waste patterns in unrelated bands (premise-blind forking, missed fan-out, an unpinned mechanism choice, backlog re-reads). A live in-place hook edit produced two total Bash-tool outages because `git_guard.py`'s crash handler denies every command indiscriminately, not just risky ones. None of these are one-off bugs — they're gaps in the loop's and the hook's own safety design.

#### [TSK-01.4.1] `workflow-implementer`'s standing rule "run the fork even when the code already appears to exist on disk" collapses two different situations into one instruction — it correctly stops a caller from skipping a fork just because code looks present, but nothing separately checks whether the task's own premise (the defect or gap it describes) still holds against current code, so a stale backlog entry can reach an implementer that builds against a premise the repo has already outgrown; confirmed cost elsewhere: a full implement + benchmark + review + revert cycle that shipped nothing because the described defect was already gone [P: H] [TODO]
* **SUB-01.4.1.1** add a premise-verification step to `workflow-implement/SKILL.md`, before the existing "run the fork even when code appears to exist" rule: read the function/file the task names and confirm the defect it describes is still present — a task whose premise the repo has outgrown goes back to `BACKLOG.md` re-aimed or removed, never forked as-is; a premise that still holds plus code that looks already-written is inherited state to verify, not a task to skip
  * **Done when:** `grep -qi "verify the task's premise" claude-code/skills/workflow-implement/SKILL.md`

#### [TSK-01.4.2] `git_guard.py` is live via symlink (`claude-code/hooks/` is symlinked into `~/.claude/hooks/`, per this repo's own `AGENTS.md`), so a direct, non-atomic write to the source file is a live outage window for every session, this one included — a half-saved state mid-edit is the most likely explanation for the post-mortem's second failure (`NameError: name 'spans' is not defined`, not confirmed at the time since the outage blocked further investigation) [P: M] [TODO]
* **SUB-01.4.2.1** document in `AGENTS.md` (or wherever hook-editing guidance belongs) that a file under `claude-code/hooks/` is live the instant it's saved — editing it should go through a copy-then-atomic-rename step (write to a temp file in the same directory, `mv` into place) rather than a direct in-place tool write whenever the edit is nontrivial enough to risk a half-written intermediate state
  * **Done when:** `grep -qi "atomic" claude-code/AGENTS.md`

#### [TSK-01.4.3] A `workflow-implement` brief that offers a design/mechanism choice without a preference lets the fork pick, and a wrong pick costs a multi-round fix cycle discovered only at review — confirmed elsewhere: a brief left the choice between two request-body-limiting mechanisms open, the fork picked the one that doesn't abort the connection and rejects a body of exactly the cap, and it took two further implement rounds (surfaced by review, not by the brief) to correct [P: M] [TODO]
* **SUB-01.4.3.1** add a rule to `workflow-loop/SKILL.md`'s P2.1 section (briefing an implement fork): when a task involves a genuine mechanism/design choice, not merely "write this function," the brief states the chosen mechanism and why, rather than leaving it to the fork's judgment — if truly undecided, that's a P0/P2.0 decision to make before forking, not something to hand off ambiguously
  * **Done when:** `grep -qi "states the chosen mechanism" claude-code/skills/workflow-loop/SKILL.md`

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

_Nothing archived yet._
