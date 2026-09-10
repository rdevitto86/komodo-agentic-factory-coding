# Project Backlog

## Convention Legend
* **Priority Tagging:** `[C]` Critical | `[H]` High | `[M]` Medium | `[L]` Low
* **Status Indicators:** `[TODO]` | `[IN_PROGRESS]` | `[BLOCKED]` | `[DONE]`
* **Hierarchy ID:** `EPIC-XX` -> `TG-XX.Y` (Task Group) -> `TSK-XX.Y.Z` (Task) -> `SUB-XX.Y.Z.N` (Subtask)
* **Owner (optional):** an `Owner` row in a task's field table marks work only a person can resolve — a policy call, an external approval, a risk-acceptance decision. Absent means agent-executable by default.

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

### [TG-01.2] Token Efficiency
* **Target Release:** V1
* **Context (2026-09-09):** the always-on budget stays healthy (`scripts/validate.sh` tracks the figure). The bounded `git_guard.py` shell-parsing hardening pass this task group's last open task tracked is complete — all Critical substitution-scanner bypasses it surfaced (comment-boundary reset, `command`-prefix reparse detection, the `command -v`/`-V` false-positive, the `env` wrapper bypass, and the `extract_substitutions`/`split_segments` dedup) are closed as of `0.46.2` (see `CHANGELOG.md`). No task currently open in this task group.

### [TG-01.3] Skill Namespace & Assess Refactor
* **Target Release:** V1

#### [TSK-01.3.1] Root `AGENTS.md`'s "atomic write via temp-file-then-mv" rule for a live `claude-code/hooks/` file is unexecutable through Bash today [P: L] [TODO]
| Subtask | Work | Done when |
|---|---|---|
| `SUB-01.3.1.1` | `git_guard.py`'s comment-guard extension list blocks `cp`/`mv`/redirect onto `.py` (and `.json`/`.md`) targets for every agent, not just `reviewer` — so the temp-file-then-`mv` sequence root `AGENTS.md` prescribes for a nontrivial hook edit cannot actually run via Bash; the Edit/Write tool is the only path that works, which contradicts the rule's own wording. Surfaced live during TSK-01.4.2: deleting `reviewer_guard.py` before updating `settings.json` broke every subsequent Edit/Write in that session (the `PreToolUse` hook command itself was missing, which the harness treats as blocking); recovered with `ln` since `mv`/`cp`/redirect were themselves blocked | the rule either states Edit/Write as the sanctioned atomic path, or `git_guard.py` gains a narrow allowance for `mv`/`cp` onto a hook-directory target so the prescribed shell sequence actually runs |

#### [TSK-01.3.2] New skill: tech debt tracking (needs refinement) [P: L] [TODO]
| Subtask | Work | Done when |
|---|---|---|
| `SUB-01.3.2.1` | scope and write a `tech-debt` skill; what it should actually cover (capturing debt, triaging it, linking it to `BACKLOG.md`) isn't decided yet — needs refinement before it's buildable | — |

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

_Nothing archived yet._
