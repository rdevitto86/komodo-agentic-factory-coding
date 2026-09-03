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

#### [TSK-01.1.1] Machine identity for agent-run git/PR operations [P: M] [TODO]
* **SUB-01.1.1.1** agent-run commits and PRs currently authored as the user's own GitHub account (`gh auth`'s session token + local `git config user.name`) — create a machine-user account or GitHub App and wire its token into `gh`/`git` for agent-run operations
  * **Done when:** `gh auth status` inside an agent run shows the machine identity, not the user's own account, and a test commit/PR is authored under it

#### [TSK-01.1.2] Bridge: `num_ctx` truncation on large summarizer payloads [P: M] [BLOCKED]
* **Blocked By:** `external`
  * **Reason (2026-08-28):** No file to edit in this repo — the bridge server (`generateRequest`, `agents.go`) lives in the separate `~/.komodo/bridge` deploy; `bridges/komodo-bridge/` here holds only prompt files and docs.
  * **Citation:** `bridges/komodo-bridge/` (prompt files and docs only, no Go source)
  * **Recheck:** bridge source is vendored into or made reachable from this repo — `find bridges/komodo-bridge -iname '*.go'` returns a match
* **SUB-01.1.2.1** fix `generateRequest`'s payload truncation against `num_ctx` in the bridge server once its source is reachable from this repo
  * **Done when:** a large summarizer payload no longer silently truncates against `num_ctx` in `~/.komodo/bridge`

#### [TSK-01.1.3] `reviewer` agent's Bash grant can write outside BACKLOG.md unexamined [P: M] [TODO]
* **SUB-01.1.3.2** [Medium] `reviewer`'s `Edit`/`Write`/`MultiEdit`/`NotebookEdit` path to non-`BACKLOG.md` files is now hook-enforced (`comment_guard.py`'s `check_reviewer_scope`, added 2026-09-01) — but its `Bash` grant is a separate, still-open bypass: `claude-code/settings.json`'s allow list permits `echo`, `printf`, and `tee` globally, and `git_guard.py`'s own redirect/`tee`/`cp`/`mv` protections gate only on `is_code_path()`, whose `EXTENSION_FAMILY` map (`comment_guard.py`) has no entry for `.json` or `.md`. `echo '{...}' > claude-code/settings.json` therefore reaches disk unexamined by either guard — `comment_guard.py` never fires (its matcher is `Edit|Write|MultiEdit|NotebookEdit`, not `Bash`), and `git_guard.py`'s redirect check treats `.json`/`.md` as a non-code path and skips it. This isn't new in this diff (the extension map is untouched here), but this band is what first hands an unattended, diff-reading fork the `Bash` tool needed to exercise it — settings.json is the file that stores which hooks even run, so this path can silently strip `comment_guard`/`git_guard`'s own `PreToolUse` registration for the rest of the session.
  * **Done when:** `is_code_path()` (or an equivalent check reachable from `git_guard.py`'s redirect/`tee`/`cp`/`mv` guards) covers `.json` and `.md`, so a shell redirect into `claude-code/settings.json` or `BACKLOG.md` is flagged the same way a redirect into a `.py`/`.go` file already is

### [TG-01.2] Comment Guard Core
* **Target Release:** V1

#### [TSK-01.2.1] write-comments splices bypass the repo's formatter [P: L] [TODO]
* **SUB-01.2.1.1** found in band closeout review, declined for this band: `write_comments_validator.py` writes files via raw I/O, so unlike every Edit/Write-tool write in this toolkit, a splice never triggers `auto_format.py` afterward — indentation is a best-effort copy of the target line's own leading whitespace, not a guaranteed-correct format. Declined here because it needs the same formatter-detection `auto_format.py` already owns and duplicating or extracting that logic is more than this band's scope warrants; filed for a follow-up pass instead of blocking this one.
  * **Done when:** after a splice, the touched file is run through the same formatter `auto_format.py` would have applied to a normal edit on that file type

### [TG-01.3] Token Efficiency
* **Target Release:** V1
* **Context (2026-09-02, updated post-shipped review-fork/AGENTS.md-trim/bundled-skill-collapse/git-pr-create-diff-stat/workflow-loop-compaction-cap/config-accessibility-fix/git_guard-backtick-fix/git_guard-dedup work):** the always-on budget is healthy (`scripts/validate.sh`: 1,155 of 2,000 tokens). Root `AGENTS.md` was cut from 7,157 to 1,416 tokens, `assess-bugs`/`assess-security`/`assess-simplify`/`backlog-audit` now run as `reviewer`/`workflow-implementer` forks instead of the orchestrator window, `git-pr-create`'s P4 read now uses `--stat` instead of the full diff, `workflow-loop/SKILL.md` was trimmed under the compaction re-attach cap with a `scripts/validate.sh` check enforcing it, `config-accessibility` was shrunk with its dead `CLAUDE.local.md` reference fixed, and the `git_guard.py` backtick false-positive was fixed alongside four Critical substitution-scanner bypasses it surfaced, plus a follow-up dedup refactor (all shipped; `backlog-audit` renumbered the remaining task below). Remaining scope: further adversarial hardening of `git_guard.py`'s shell-parsing.

#### [TSK-01.3.1] git_guard.py's shell-parsing is not exhaustively adversarial-hardened against every quoting/escaping/substitution combination [P: L] [TODO]
* **SUB-01.3.1.1** the 2026-09-01 band that fixed the `git_guard.py` backtick false-positive (and, in review, caught and fixed three unrelated Critical bypasses along the way — a `#`-comment quote-state swallow, missing backtick/`$()` substitution detection entirely, and an escaped-nested-backtick gap) deliberately stopped hardening `claude-code/hooks/git_guard.py`'s `extract_substitutions`/`split_segments` once those four were closed, rather than continuing to chase further shell-quoting edge cases in the same pass — hand-parsing arbitrary POSIX shell quoting/escaping/substitution semantics to zero residual risk is open-ended, the same reasoning already applied to the risk-accepted `grep`/`sed`/`awk`/`curl` secret-exfiltration gap in `CHANGELOG.md`'s `[0.37.2]` entry. Untested-but-plausible remaining edge cases: `$(...)` containing backslash-escaped backticks, deeper mixed single/double-quote/substitution nesting, and other exotic POSIX escaping shapes. `scripts/test-hooks.sh` now covers `G90`-`G98` for the shapes found so far.
  * **Done when:** a dedicated, systematic pass (ideally against a real shell-grammar reference or fuzzer, not ad hoc cases) audits `extract_substitutions`/`split_segments`/`find_backtick_end`/`find_paren_end` against the POSIX shell quoting grammar and either closes every gap found or explicitly risk-accepts each one in `CHANGELOG.md`, matching the existing `[0.37.2]` pattern

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

_Nothing archived yet._
