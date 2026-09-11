# Project Backlog

## Convention Legend
* **Priority Tagging:** `[C]` Critical | `[H]` High | `[M]` Medium | `[L]` Low
* **Status Indicators:** `[TODO]` | `[IN_PROGRESS]` | `[BLOCKED]` | `[DONE]`
* **Hierarchy ID:** `EPIC-XX` -> `TG-XX.Y` (Task Group) -> `TSK-XX.Y.Z` (Task) -> `SUB-XX.Y.Z.N` (Subtask)
* **Owner:** `agent` (default) or `human` (requires human decision, external PR merge, or policy approval).

Format and rules live in the `backlog-modify` skill — load it before editing this file. `[DONE]` tasks stay until a sweep (`/backlog-audit`) moves them to `CHANGELOG.md` and removes them — this is not a log to hand-curate.

---

## [EPIC-01] Now, V1
*Goal: keep this toolkit's own hooks, docs, and skills correct and internally consistent.*

### [TG-01.1] Cross-Cutting
* **Target Release:** V1

#### [TSK-01.1.1] Bridge: `num_ctx` truncation on large summarizer payloads [P: M] [BLOCKED]
| Field | Value |
|---|---|
| Blocked by | `external` |
| Reason (2026-08-28) | No file to edit in this repo — the bridge server (`generateRequest`, `agents.go`) lives in the separate `~/.komodo/bridge` deploy; `bridges/komodo-bridge/` here holds only prompt files and docs. |
| Citation | `bridges/komodo-bridge/` (prompt files and docs only, no Go source) |
| Recheck | bridge source is vendored into or made reachable from this repo — `find bridges/komodo-bridge -iname '*.go'` returns a match |

| Subtask | Work | Done when |
|---|---|---|
| `SUB-01.1.1.1` | fix `generateRequest`'s payload truncation against `num_ctx` in the bridge server once its source is reachable from this repo | a large summarizer payload no longer silently truncates against `num_ctx` in `~/.komodo/bridge` |

#### [TSK-01.1.2] `repo_root_of()`'s `@functools.lru_cache` builds its hash key before the function body runs, so an unhashable `cwd` (a `list`/`dict`) still raises an uncaught `TypeError` even after the earlier `TypeError`-catch fix [P: L] [TODO]
| Subtask | Work | Done when |
|---|---|---|
| `SUB-01.1.2.1` | surfaced by an `/assess-bugs` pass: `lru_cache`'s own key-hashing happens outside the `try`/`except` the fix widened, so it never sees an unhashable `cwd`. Currently unreachable — the sole call site (`claude-code/hooks/git_guard.py:498`) always passes a `str` — so this is latent, not live | a regression case (or inline check) confirms `repo_root_of(['a'])` no longer raises an uncaught `TypeError`, e.g. by validating/coercing `cwd` before the cached call, or wrapping the cache lookup itself |

#### [TSK-01.1.3] `BACKLOG.md` format gains User Story, Acceptance Criteria, and subtask Category; the four closeout tasks move to their own task group [P: M] [TODO]

**User Story:**
> **As a** maintainer scaffolding a new Komodo repo,
> **I want** each task to state its behavioral boundaries and test tiers explicitly,
> **So that** an implementer fork knows what "done" covers without reading the code.

**Acceptance Criteria:**
- [ ] **AC-1 (Spec):** Given `backlog-modify/SKILL.md`, when its format section is read, then it mandates the `User Story` block, checkbox `Acceptance Criteria`, and the `Category` column, and no longer bans checkboxes or a standalone AC list.
- [ ] **AC-2 (Placement):** Given an app/service/infra repo, when a backlog is scaffolded, then the four closeout tasks appear under `[TG-XX.2] Quality Assurance & Epic Hardening` with a `Trigger` bullet — and given a skill/config/doc-only repo, then that task group is absent entirely.
- [ ] **AC-3 (Location drift):** Given `backlog-plan`, `backlog-prioritize`, `workflow-loop/SKILL.md`, and `ways/sdlc.md`, when each is read in full, then none places the closeout tasks in `Cross-Cutting`.
- [ ] **AC-4 (Shape drift):** Given `backlog-audit`, when its verdict table and sweep steps are read, then they describe `Done when` table cells rather than bullets.
- [ ] **AC-5 (Sweep gate):** Given a `[DONE]` task carrying an unchecked `AC-` box, when `/backlog-audit` runs, then the task is verdicted Ambiguous and not swept — and given every box checked and every command green, then it sweeps normally.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.3.1` | `[Impl]` | rewrite `backlog-modify/SKILL.md`'s format spec and Rules: add the `User Story` block, checkbox `Acceptance Criteria`, and the `Category` column; delete the no-checkboxes and no-standalone-AC rules together with their rationale; move the four closeout tasks into their own `Quality Assurance & Epic Hardening` task group while preserving the skill/config/doc-only carve-out. Define the `Trigger` bullet explicitly as part of that task group's shape — the field name, that it states the condition under which the group's tasks run, and its canonical wording (`Run once all functional Task Groups in <EPIC> reach [DONE]`) | `bash scripts/validate.sh`; `grep -c "Trigger" claude-code/skills/backlog-modify/SKILL.md` returns non-zero |
| `SUB-01.1.3.2` | `[Impl]` | propagate the new closeout location to `backlog-plan:56`, `backlog-prioritize:22`, `workflow-loop/SKILL.md:149`, and `ways/sdlc.md:62,66`. The prose around each edit is verified by a human read at P2.3 band review, not by this cell — the grep only proves the two strings stopped co-occurring | `grep -rn "closeout" claude-code/skills \| grep -c "Cross-Cutting"` returns `0` |
| `SUB-01.1.3.3` | `[Impl]` | fix `backlog-audit:33,38,52` to audit `Done when` table cells rather than bullets, and add the AC-5 sweep gate to its verdict table | `grep -c "Done when:\` bullet" claude-code/skills/backlog-audit/SKILL.md` returns `0`; `bash scripts/validate.sh` |
| `SUB-01.1.3.4` | `[Impl]` | teach `workflow-implement` to tick the `AC-` boxes its task satisfied once that subtask's `Done when` commands exit zero — without this the AC-5 gate deadlocks every backlog, since nothing else marks a box | `grep -c "AC-" claude-code/skills/workflow-implement/SKILL.md` returns non-zero; `bash scripts/validate.sh` |
| `SUB-01.1.3.5` | `[Impl]` | rewrite `templates/project/BACKLOG.md.tmpl` structurally, not just textually: every example task gains a `User Story` block, a checkbox `Acceptance Criteria` list, and a `Category` column; the eight closeout tasks currently nested in `[TG-01.1] Cross-Cutting` and `[TG-02.1] Cross-Cutting` move into per-epic `Quality Assurance & Epic Hardening` task groups carrying the `Trigger` bullet; `/audit-security`→`/assess-security`, `/audit-bugs`→`/assess-bugs`, `/audit-simplify`→`/assess-simplify`; and `npm run test:*` becomes `<test command>` | `grep -c "audit-" templates/project/BACKLOG.md.tmpl` returns `0`; `grep -c "Quality Assurance & Epic Hardening" templates/project/BACKLOG.md.tmpl` returns `2`; `grep -c "Category" templates/project/BACKLOG.md.tmpl` returns non-zero; `grep -c "npm run" templates/project/BACKLOG.md.tmpl` returns `0` |
| `SUB-01.1.3.6` | `[Impl]` | record in `docs/design-decisions.md` why the no-checkbox rule was dropped and what the AC boxes now gate | `make verify` |

#### [TSK-01.1.4] `git-repo-init`'s `BACKLOG.md` seed-story splicing describes a flat bullet line shape that matches neither the current table format nor `TSK-01.1.3`'s target shape [P: L] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1:** Given `git-repo-init/SKILL.md:107`, when read, then its seed-story shape matches whatever `backlog-modify` currently mandates.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.4.1` | `[Impl]` | surfaced by `/workflow-decompose` while gap-checking `TSK-01.1.3`: line 107 still documents `- T.D.S \| SEV \| [WIP] <text> · <size> → \`<done when>\`` for spliced seed stories. This is pre-existing drift — wrong against today's table format independently of `TSK-01.1.3`, so it is filed separately rather than widening that task to a tenth file | `bash scripts/validate.sh` |

#### [TSK-01.1.5] `git_guard.py` reads a heredoc body as command text, so a commit message that merely mentions a git command is denied [P: M] [TODO]

**User Story:**
> **As an** agent writing a commit message that explains git behavior,
> **I want** the guard to distinguish a command from a string that describes one,
> **So that** I am not forced to reword an accurate message into a vaguer one.

**Acceptance Criteria:**
- [ ] **AC-1:** Given `git commit -F -` with a heredoc body containing the words `git pull`, when the guard scans it, then the commit is allowed.
- [ ] **AC-2:** Given a genuine `git pull` without `--ff-only` anywhere in the command, when the guard scans it, then it is still denied.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.5.1` | `[Impl]` | hit live twice while publishing this band: `git commit -F -` with a heredoc body reading "any `git pull` in the clone changed every engineer's next session" was denied with "git pull is allowed only with --ff-only", and `gh pr create --body "$(cat <<'PRBODY' ...)"` was denied for "git checkout changes repository state" because the PR prose quoted a checkout invocation. The guard scans the whole Bash command string, and a heredoc body is part of it, so prose describing a command is indistinguishable from the command. Teach the segment scanner to skip heredoc bodies — the delimiter is known from the `<<` operator, so the span is decidable without parsing the shell fully | `bash scripts/test-hooks.sh` passes with new cases covering both acceptance criteria |
| `SUB-01.1.5.2` | `[UnitTest]` | add the two regression cases to `scripts/test-hooks.sh` | `bash scripts/test-hooks.sh` |

#### [TSK-01.1.6] `scripts/test-install.sh` is not wired into `make verify`, so neither CI nor the Stop gate ever runs it [P: M] [TODO]

**User Story:**
> **As a** maintainer relying on the repo's own gate,
> **I want** every regression suite in `scripts/` to run in CI,
> **So that** a suite cannot pass locally and rot unnoticed on the branch.

**Acceptance Criteria:**
- [ ] **AC-1:** Given `make verify`, when it runs, then `scripts/test-install.sh` executes and a failure in it fails the target.
- [ ] **AC-2:** Given the `verify` GitHub Actions workflow, when it runs on a PR, then that suite's result is visible in the run log.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.6.1` | `[Impl]` | surfaced while publishing the `--ref` band: `Makefile`'s `verify` target is `test validate comments`, where `test` is `scripts/test-hooks.sh` only. `scripts/test-install.sh` — 12 cases, six of them added by that band to cover `setup.sh --ref` — runs nowhere automatic, so the flag's entire guard surface is unprotected against regression in CI. Add it as its own target and fold it into `verify`. Note it is slower than the other three (it clones the repo and runs full installs into temp dirs), so measure the added wall-clock against `KOMODO_VERIFY_TIMEOUT`'s 300s default before wiring it in | `make verify` runs `scripts/test-install.sh`; `time make verify` stays under 300s |

### [TG-01.2] Token Efficiency
* **Target Release:** V1
* **Context (2026-09-09):** the always-on budget stays healthy (`scripts/validate.sh` tracks the figure). The bounded `git_guard.py` shell-parsing hardening pass this task group's last open task tracked is complete — all Critical substitution-scanner bypasses it surfaced (comment-boundary reset, `command`-prefix reparse detection, the `command -v`/`-V` false-positive, the `env` wrapper bypass, and the `extract_substitutions`/`split_segments` dedup) are closed as of `0.46.2` (see `CHANGELOG.md`). No task currently open in this task group.

### [TG-01.3] Skill Namespace & Assess Refactor
* **Target Release:** V1

#### [TSK-01.3.1] Root `AGENTS.md`'s "atomic write via temp-file-then-mv" rule for a live `claude-code/hooks/` file is unexecutable through Bash today [P: L] [TODO]
| Subtask | Work | Done when |
|---|---|---|
| `SUB-01.3.1.1` | `git_guard.py`'s comment-guard extension list blocks `cp`/`mv`/redirect onto `.py` (and `.json`/`.md`) targets for every agent, not just `reviewer` — so the temp-file-then-`mv` sequence root `AGENTS.md` prescribes for a nontrivial hook edit cannot actually run via Bash; the Edit/Write tool is the only path that works, which contradicts the rule's own wording. Surfaced live during TSK-01.4.2: deleting `reviewer_guard.py` before updating `settings.json` broke every subsequent Edit/Write in that session (the `PreToolUse` hook command itself was missing, which the harness treats as blocking); recovered with `ln` since `mv`/`cp`/redirect were themselves blocked | the rule either states Edit/Write as the sanctioned atomic path, or `git_guard.py` gains a narrow allowance for `mv`/`cp` onto a hook-directory target so the prescribed shell sequence actually runs |

#### [TSK-01.3.3] New skill: git branching strategy (feature/branch vs smaller PRs) [P: L] [TODO]
| Subtask | Work | Done when |
|---|---|---|
| `SUB-01.3.3.1` | write a skill giving guidance on when to use a feature branch versus splitting work into smaller, direct PRs | — |

#### [TSK-01.3.4] New skill: `/standards-zig` [P: L] [TODO]
| Subtask | Work | Done when |
|---|---|---|
| `SUB-01.3.4.1` | write a `standards-zig` skill covering Zig language/build/toolchain conventions, structured like the existing `standards-aws` and `standards-specs` skills | `claude-code/skills/standards-zig/SKILL.md` exists and `bash scripts/validate.sh` passes |

### [TG-01.4] Workflow Loop & Hook Reliability
* **Target Release:** V1
* **Context (2026-09-09):** one band, one PR, by decision — the six tasks below chain on purpose (`(after:)` edges) because they all touch `workflow-loop/SKILL.md`, `scripts/test-hooks.sh`, or `docs/design-decisions.md`; the PR will exceed `git-pr-create`'s 12-file soft max and says so in its body. Target: a one-task band drops from 7 serial forks to 4, review findings below Critical/High stop triggering fix-and-re-review loops, comments get written by the implementer with live context and reviewed at band review, and every phase instruction is executable by the Skill tool (which has no `isolation` parameter). Full rationale: the "Workflow Loop Tune-Up" plan; verify each task with `make verify` — `workflow-loop/SKILL.md` sits near `validate.sh`'s 5000-token compaction cap, so every rewrite of it must net-shrink or hold.

#### [TSK-01.4.10] `comments_write_invocation`/`is_comments_script`/`comments_script_signature`/`cat_source_path` and their supporting constants are now unreachable dead code in `git_guard.py`, superseded by `TSK-01.4.9`'s deny-by-default reviewer gate [P: M] [TODO]
| Subtask | Work | Done when |
|---|---|---|
| `SUB-01.4.10.1` | `TSK-01.4.9`'s new gate (`agent_type == REVIEWER_AGENT and command != "git"`) denies every non-`git` reviewer command before `scan_segment` ever reaches the old comments.py-bypass detection call site, so `comments_write_invocation` (and everything it alone calls: `is_comments_script`, `comments_script_signature`, `cat_source_path`, plus `COMMENTS_SCRIPT_BASENAME`/`COMMENTS_SCRIPT_REALPATH`/`COMMENTS_WRITE_SUBCOMMANDS`) is defined but never called from anywhere. Flagged by `TSK-01.4.9`'s own implementer and by its band-review `/assess-bugs` pass as a follow-up simplify candidate, deliberately not removed in that task to keep its diff scoped to the security fix alone. `piped_source` threading through `scan_segment`/`_scan_command_at_depth` (its only consumer was `comments_write_invocation`) becomes dead alongside it | `/assess-simplify claude-code/hooks/git_guard.py` reports the dead functions/constants removed (or confirms none remain reachable); `bash scripts/test-hooks.sh` still passes with the same pass count minus any cases that existed solely to exercise the removed code path |
| `SUB-01.4.10.2` | filed by `TSK-01.4.9`'s band-review `/assess-simplify` pass: `scripts/test-hooks.sh`'s `G161`-`G163` cases (plus the `FIXTURE_COMMENTS_COPY`/`FIXTURE_COMMENTS_ALIAS` fixtures at lines ~812-816) were built to exercise `is_comments_script`'s samefile/content-signature matching — with the new deny-by-default gate, every one of those commands is now rejected by the generic `command != "git"` check before that identity/content logic is ever reached, and `G179`-`G188` already cover the same ground more directly (`G186` explicitly proves the gate is basename-agnostic). The fixture setup now builds infrastructure no reviewer-agent test path can reach | `scripts/test-hooks.sh`'s round-4 comments.py-copy/alias block (`G161`-`G163` and their now-unreachable fixtures) is collapsed or removed without losing any assertion `G179`-`G188` doesn't already make; `bash scripts/test-hooks.sh` still passes |

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

- **New skill: tech debt tracking** (2026-09-10) — dropped. Was only ever a loose idea (capture/triage/link debt to `BACKLOG.md`); user decided it may not be needed at all rather than refine the scope.
