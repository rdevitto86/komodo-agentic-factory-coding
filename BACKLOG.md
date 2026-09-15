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

#### [TSK-01.1.4] `git-repo-init`'s `BACKLOG.md` seed-story splicing describes a flat bullet line shape that matches neither the current table format nor `TSK-01.1.3`'s target shape [P: L] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1:** Given `git-repo-init/SKILL.md:107`, when read, then its seed-story shape matches whatever `backlog-modify` currently mandates.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.4.1` | `[Impl]` | surfaced by `/workflow-decompose` while gap-checking `TSK-01.1.3`: line 107 still documents `- T.D.S \| SEV \| [WIP] <text> · <size> → \`<done when>\`` for spliced seed stories. This is pre-existing drift — wrong against today's table format independently of `TSK-01.1.3`, so it is filed separately rather than widening that task to a tenth file | `python3 scripts/validate.py` |

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
| `SUB-01.1.5.1` | `[Impl]` | hit live twice while publishing this band: `git commit -F -` with a heredoc body reading "any `git pull` in the clone changed every engineer's next session" was denied with "git pull is allowed only with --ff-only", and `gh pr create --body "$(cat <<'PRBODY' ...)"` was denied for "git checkout changes repository state" because the PR prose quoted a checkout invocation. The guard scans the whole Bash command string, and a heredoc body is part of it, so prose describing a command is indistinguishable from the command. Teach the segment scanner to skip heredoc bodies — the delimiter is known from the `<<` operator, so the span is decidable without parsing the shell fully | `python3 scripts/test_hooks.py` passes with new cases covering both acceptance criteria |
| `SUB-01.1.5.2` | `[UnitTest]` | add the two regression cases to `scripts/test_hooks.py` | `python3 scripts/test_hooks.py` |

#### [TSK-01.1.6] `scripts/test_install.py` is not wired into `make verify`, so neither CI nor the Stop gate ever runs it [P: M] [TODO]

**User Story:**
> **As a** maintainer relying on the repo's own gate,
> **I want** every regression suite in `scripts/` to run in CI,
> **So that** a suite cannot pass locally and rot unnoticed on the branch.

**Acceptance Criteria:**
- [ ] **AC-1:** Given `make verify`, when it runs, then `scripts/test_install.py` executes and a failure in it fails the target.
- [ ] **AC-2:** Given the `verify` GitHub Actions workflow, when it runs on a PR, then that suite's result is visible in the run log.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.6.1` | `[Impl]` | surfaced while publishing the `--ref` band: `Makefile`'s `verify` target is `test validate comments`, where `test` is `scripts/test_hooks.py` only. `scripts/test_install.py` — 12 cases, six of them added by that band to cover `setup.sh --ref` — runs nowhere automatic, so the flag's entire guard surface is unprotected against regression in CI. Add it as its own target and fold it into `verify`. Note it is slower than the other three (it clones the repo and runs full installs into temp dirs), so measure the added wall-clock against `KOMODO_VERIFY_TIMEOUT`'s 300s default before wiring it in | `make verify` runs `scripts/test_install.py`; `time make verify` stays under 300s |

#### [TSK-01.1.7] Nothing enforces that a task's Acceptance Criteria actually map to a subtask, so an unmapped AC deadlocks the task in `backlog-audit`'s Ambiguous bucket indefinitely [P: L] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1:** Given a task whose Acceptance Criteria include one with no subtask proving it, when it is planned or normalized, then the gap is surfaced at write time rather than only discovered later at sweep.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.7.1` | `[Impl]` | surfaced by `/assess-bugs` at TSK-01.1.3's band review: `backlog-modify` and `docs/design-decisions.md` both acknowledge the failure mode ("a backlog defect, not a sweep the gate should quietly let through") but nothing enforces the mapping at write time — `backlog-plan`'s decomposition constraints don't check it, and no lint exists. Recoverable only by a human manually ticking the box. Deliberately not fixed in the same band since it's an acknowledged, intentional trade-off rather than a defect | `python3 scripts/validate.py` |

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
| `SUB-01.3.4.1` | write a `standards-zig` skill covering Zig language/build/toolchain conventions, structured like the existing `standards-aws` and `standards-specs` skills | `claude-code/skills/standards-zig/SKILL.md` exists and `python3 scripts/validate.py` passes |

### [TG-01.4] Workflow Loop & Hook Reliability
* **Target Release:** V1
* **Context (2026-09-09):** one band, one PR, by decision — the six tasks below chain on purpose (`(after:)` edges) because they all touch `workflow-loop/SKILL.md`, `scripts/test_hooks.py`, or `docs/design-decisions.md`; the PR will exceed `git-pr-create`'s 12-file soft max and says so in its body. Target: a one-task band drops from 7 serial forks to 4, review findings below Critical/High stop triggering fix-and-re-review loops, comments get written by the implementer with live context and reviewed at band review, and every phase instruction is executable by the Skill tool (which has no `isolation` parameter). Full rationale: the "Workflow Loop Tune-Up" plan; verify each task with `make verify` — `workflow-loop/SKILL.md` sits near `validate.sh`'s 5000-token compaction cap, so every rewrite of it must net-shrink or hold.

#### [TSK-01.4.10] `comments_write_invocation`/`is_comments_script`/`comments_script_signature`/`cat_source_path` and their supporting constants are now unreachable dead code in `git_guard.py`, superseded by `TSK-01.4.9`'s deny-by-default reviewer gate [P: M] [TODO]
| Subtask | Work | Done when |
|---|---|---|
| `SUB-01.4.10.1` | `TSK-01.4.9`'s new gate (`agent_type == REVIEWER_AGENT and command != "git"`) denies every non-`git` reviewer command before `scan_segment` ever reaches the old comments.py-bypass detection call site, so `comments_write_invocation` (and everything it alone calls: `is_comments_script`, `comments_script_signature`, `cat_source_path`, plus `COMMENTS_SCRIPT_BASENAME`/`COMMENTS_SCRIPT_REALPATH`/`COMMENTS_WRITE_SUBCOMMANDS`) is defined but never called from anywhere. Flagged by `TSK-01.4.9`'s own implementer and by its band-review `/assess-bugs` pass as a follow-up simplify candidate, deliberately not removed in that task to keep its diff scoped to the security fix alone. `piped_source` threading through `scan_segment`/`_scan_command_at_depth` (its only consumer was `comments_write_invocation`) becomes dead alongside it | `/assess-simplify claude-code/hooks/git_guard.py` reports the dead functions/constants removed (or confirms none remain reachable); `python3 scripts/test_hooks.py` still passes with the same pass count minus any cases that existed solely to exercise the removed code path |
| `SUB-01.4.10.2` | filed by `TSK-01.4.9`'s band-review `/assess-simplify` pass: `scripts/test_hooks.py`'s `G161`-`G163` cases (plus the `FIXTURE_COMMENTS_COPY`/`FIXTURE_COMMENTS_ALIAS` fixtures at lines ~812-816) were built to exercise `is_comments_script`'s samefile/content-signature matching — with the new deny-by-default gate, every one of those commands is now rejected by the generic `command != "git"` check before that identity/content logic is ever reached, and `G179`-`G188` already cover the same ground more directly (`G186` explicitly proves the gate is basename-agnostic). The fixture setup now builds infrastructure no reviewer-agent test path can reach | `scripts/test_hooks.py`'s round-4 comments.py-copy/alias block (`G161`-`G163` and their now-unreachable fixtures) is collapsed or removed without losing any assertion `G179`-`G188` doesn't already make; `python3 scripts/test_hooks.py` still passes |

#### [TSK-01.4.14] `install.sh`'s new `core.hooksPath` classification carries a fourth state, `different`, that no branch downstream ever reads [P: M] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1 (Collapse):** Given `scripts/hooks/git/install.sh`, when its `core.hooksPath` classification is read, then it carries only the distinctions some branch below actually tests.
- [ ] **AC-2 (No regression):** Given `scripts/test_install.py`, when it runs, then `G1`-`G4` pass unchanged.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.4.14.1` | `[Impl]` | filed by `TSK-01.4.13`'s band-review `/assess-simplify` pass: the classification added at `scripts/hooks/git/install.sh:42-58` sets `state="different"` for an existing-but-not-`$HOOK_DIR` path, but every branch below tests only `current` or `stale` — `different` and `unset` take the identical `else` in both `--status` and the install path. The `[ -d ]` stat and the absolute-vs-relative `case` are load-bearing only for the stale distinction. Two flags (`is_current`, `is_stale`) express the same behavior with no dead assignment | `python3 scripts/test_install.py` passes with `G1`-`G4` unchanged; `python3 scripts/validate.py` |

### [TG-01.5] Review Precision & Model Tiering
* **Target Release:** V1
* **Context (2026-09-15):** the `Never invent a finding` prohibition is already in all nine `assess-*` skills and in `reviewer.md`, so the gap is not the missing rule — it is that only `assess-security` states a *positive* evidence bar. A negative rule cannot be complied with; a positive one can. Paired with the model tier, since no prompt change substitutes for the reviewer running on the weaker model.

#### [TSK-01.5.3] The three `assess-*` skills that do not fork as `reviewer` now state an evidence bar with nowhere to put what it excludes [P: M] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1 (Disposal):** Given `assess-testing`, `assess-vulnerabilities`, and `assess-performance`, when each is read, then a near-miss that fails the evidence bar has a named place to go.
- [ ] **AC-2 (No agent coupling):** Given the same three skills, when they run, then the disposal path works without them forking as `reviewer`.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.5.3.1` | `[Impl]` | filed by `TSK-01.5.1`'s band-review `/assess-bugs` pass: `reviewer.md` routes a near-miss to `## Considered and dismissed`, but that instruction reaches only the three skills carrying `context: fork`/`agent: reviewer`. `assess-testing`, `assess-vulnerabilities`, and `assess-performance` gained the bar this round and run inline, so a near-miss they catch is silently dropped rather than preserved. Give each the same disposal section in its own `## Report` contract | `python3 scripts/validate.py`; `make verify` |

#### [TSK-01.5.4] The evidence bar is now written five independent times, with no canonical statement to propagate an edit from [P: L] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1 (Single source):** Given the evidence bar, when it is edited in one place, then no other file needs a matching hand-edit to stay consistent.
- [ ] **AC-2 (AC-4 tension resolved):** Given `TSK-01.5.1`'s AC-4, when this is done, then the decision to keep or drop `assess-bugs`'s own copy is recorded rather than left implicit.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.5.4.1` | `[Impl]` | filed by `TSK-01.5.1`'s band-review `/assess-bugs` and `/assess-simplify` passes, which disagreed: `reviewer.md` and four skills each hand-write their own trigger/path/effect sentence, so an edit to the agent's wording has nothing to propagate from. `/assess-simplify` argued `assess-bugs`'s copy is pure duplication since it forks as `reviewer` and already inherits the agent's text — `assess-simplify`, the third fork skill, correctly got no copy. But `TSK-01.5.1`'s AC-4 explicitly required `assess-bugs` to state one, so removing it contradicts a shipped acceptance criterion. Resolve the tension deliberately: either amend the AC's intent in `docs/design-decisions.md` and drop the inherited copies, or keep them and record why the duplication is accepted | `python3 scripts/validate.py`; `make verify` |

### [TG-01.6] Agent Roster
* **Target Release:** V1
* **Context (2026-09-15):** the roster is phase-named (`workflow-implementer`, `workflow-planner`), which is exactly why it is narrow — the name lies the moment the agent is invoked outside the loop. Renaming to role names decouples agent from phase and makes `workflow-loop` a mapping table instead of the owner. Sequenced before parallelism and prompt templates, both of which depend on stable role names. Five agent descriptions cost ~246 always-on tokens today; eight cost ~400 against 871 free.

#### [TSK-01.6.2] Nothing checks that a skill's `agent:` frontmatter names an agent that exists, so a roster change breaks every fork of that skill silently [P: H] [TODO]

**User Story:**
> **As** whoever changes the agent roster next,
> **I want** a stale `agent:` key to fail the repo's own gate,
> **So that** the failure surfaces at `make verify` rather than at the next fork that tries to run.

**Acceptance Criteria:**
- [ ] **AC-1 (Cross-check):** Given a skill whose `agent:` key names no file in `claude-code/agents/`, when `scripts/validate.sh` runs, then it fails and names the skill and the missing agent.
- [ ] **AC-2 (Name, not filename):** Given the check, when it resolves an agent, then it matches the agent's `name:` frontmatter, not its filename — the loader keys on `name:`.
- [ ] **AC-3 (Constant):** Given `REVIEWER_AGENT` in `claude-code/hooks/lib/agents.py`, when the check runs, then that constant is verified against a real agent's `name:` too.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.6.2.1` | `[Impl]` | filed during `TSK-01.6.1`'s rename: `scripts/validate.sh` validates agent and skill frontmatter against the loader's key schema but never cross-checks an `agent:` VALUE against the roster, so the six skills repointed by that rename would have failed silently had any name been mistyped. `git_guard.py`'s reviewer gate has the same shape — it keys on `REVIEWER_AGENT` and falls through to allow if the string stops matching, which is a security surface failing open, not just a broken fork | `python3 scripts/validate.py` fails on a deliberately mistyped `agent:` value and passes on the real tree; `make verify` |

#### [TSK-01.6.3] `docs/design-decisions.md`'s per-role contract rationale covers `architect` but not `tester` [P: L] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1:** Given the paragraph recording which existing output contract each role was weighed against, when it is read, then `tester` is covered like every other agent in the roster.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.6.3.1` | `[Impl]` | filed by `TSK-01.6.1`'s band-review `/assess-simplify` pass: the paragraph exists to record, per role, which existing contract was asked-and-rejected before a new agent was added. It states that for `builder`, `pm`, `reviewer`, and `architect`, but not for `tester` — whose Output template visibly reuses `builder`'s `Result`/`Verified`/`Notes` shape, so the answer is "it did fit, and was reused". One clause | `python3 scripts/validate.py` |

#### [TSK-01.6.4] `AGENTS.md` states a hook regression-case count that the suite has outgrown [P: L] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1:** Given `AGENTS.md`'s line naming the regression-case count, when `scripts/test_hooks.py` runs, then the two agree.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.6.4.1` | `[Impl]` | pre-existing drift, spotted by a fork during `TSK-01.6.1` and deliberately not fixed there to keep that diff to the lines the rename required: `AGENTS.md` says `259 hook + comments regression cases` and `README.md` says `286 regression cases`, while the suite reports 293 (it moved twice during `TG-01.10` alone). Confirm which number is authoritative before editing — the 259 may have been scoped to a subset — and consider whether a hand-maintained count belongs in the file at all | `python3 scripts/test_hooks.py`'s reported count matches `AGENTS.md`; `make verify` |

### [TG-01.7] Cross-Platform Portability
* **Target Release:** V1
* **Context (2026-09-15):** every hook is already Python and `scripts/install.py` already ships, so the remaining non-portable surface is six shell files plus the `Makefile`. The sharp end is the gate itself — `verify_gate.py` resolves `.claude/verify.sh` → `make verify` → `task verify` → `just verify`, and three of those four do not exist on a stock Windows box. No preloaded binaries: `python3` 3.7+ is already the hard floor and a binary would *add* a setup step.

### [TG-01.8] UI & Language Standards
* **Target Release:** V1
* **Context (2026-09-15):** `standards-ui-design` and `standards-ui-security` are web-only skills wearing generic names — the security one covers `postMessage`, iframes, and `frame-ancestors`, all browser. Merging design and security per platform follows the precedent the language skills already set (`assess-security` reads a Security section out of `standards-go`, and does not load a separate skill for it), and native-only mobile removes the one unsolved activation problem: with React Native out of scope, `**/*.tsx` is unambiguously web again. `standards-swift` and `standards-kotlin` landed 2026-09-15, defaulted off.

### [TG-01.9] Accessibility & Visual Output
* **Target Release:** V1
* **Context (2026-09-15):** `config-accessibility` governs formatting — heading depth, density caps, emoji placement — while every skill's Report section still contracts for a markdown table. The rules say show-don't-tell and the output contracts say emit-a-table. Since a terminal cannot render a diagram, visual output means a hosted artifact, which is a real shift in where this toolkit's output lives and is why this is a task group rather than a single skill.

### [TG-01.10] Parallelism & Loop Latency
* **Target Release:** V1
* **Context (2026-09-15):** a simple edit currently costs 30–60 minutes, and `TG-01.4`'s own context names why — a one-task band runs 7 serial forks, and `verify_gate.py` fires on every builder Stop, so an N-task band runs the full gate N times. Parallelism does not fix that; it fixes the multi-task band. The fast path and the band-level gate are the levers that return time on the case that actually hurts. Parallelism is opportunistic by decision: the planner proves disjointness or the band runs serial.

#### [TSK-01.10.4] The band-gate deferral is scoped to a checkout, not a session, so two concurrent sessions on one repo can cross [P: M] [TODO]

**User Story:**
> **As** someone running two sessions against one checkout,
> **I want** a band gate deferred by one of them not to suppress the other's,
> **So that** a change is never committed with the repo's own gate silently skipped on its behalf.

**Acceptance Criteria:**
- [ ] **AC-1 (Binding):** Given a band-gate marker, when a `builder` fork's Stop reads it, then the deferral applies only to the run that wrote it.
- [ ] **AC-2 (Unbindable is closed):** Given no identifier available to bind, when the marker is read, then the gate runs.
- [ ] **AC-3 (Honest doc):** Given `docs/design-decisions.md`, when the band gate is described, then it no longer records this as an open limitation.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.10.4.1` | `[Impl]` | filed by `TSK-01.10.2`'s band-review `/assess-security` pass, and deliberately mitigated rather than fixed there. The marker keys off the git common dir, so it is shared by every worktree of a checkout — which is what makes parallel forks work, and also what lets an unrelated concurrent session inherit the deferral, with no code path in its own flow that ever runs the full suite to compensate. It could not be fixed in that band because the Stop payload `verify_gate.py` receives carries only `stop_hook_active` and `cwd` — no session identifier exists to bind to. The window was cut from four hours to thirty minutes to bound the blast radius instead. Resolving this needs a session or band identifier that survives into the fork's Stop payload; establish whether one is available before designing, and if none is, record that and close this as won't-fix rather than inventing a token the orchestrator has to hand-manage | `python3 scripts/test_hooks.py` covers a marker written by one identity not deferring another's gate; `make verify` |

### [TG-01.11] Prompt Templates
* **Target Release:** V1
* **Context (2026-09-15):** the deciding rule is the one this repo already adopted for comments — enforce only what is decidable. A brief is generated per invocation and fails by omission, which is decidable; a standards skill is read per session and fails by misjudgment, which is not. So briefs and report contracts become templates, and `AGENTS.md`, `standards-*`, agent bodies, and `ways/` stay static prose. Sequenced last by decision, and after the roster rename, since each template is owned by the role it addresses.

#### [TSK-01.11.1] A fork brief is described in prose in three separate places and validated nowhere, so an omitted slot is only discovered by the fork guessing or stopping (after: "The agent roster is named after workflow phases rather than roles") [P: M] [TODO]

**User Story:**
> **As an** orchestrator briefing a fork that cannot see this conversation,
> **I want** one template per role with required slots,
> **So that** an incomplete brief is caught before the fork burns a turn on it.

**Acceptance Criteria:**
- [ ] **AC-1 (Templates):** Given `templates/briefs/`, when it is listed, then it holds one template per role in the roster.
- [ ] **AC-2 (Slots):** Given each template, when it is read, then it carries `Task`, `Files`, `Context`, `Done when`, and `Out of scope`, plus `Round` and `Standards` for reviewer briefs.
- [ ] **AC-3 (Enforcement):** Given a brief arriving with a required slot empty, when the receiving agent reads it, then it stops and names the missing slot rather than guessing.
- [ ] **AC-4 (Deduplication):** Given `workflow-loop`, `ways/sdlc.md`, and `workflow-implement`, when each is read, then none carries its own prose description of brief contents.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.11.1.1` | `[Impl]` | write one `templates/briefs/<role>.md.tmpl` per role. These live outside any loaded path, so they cost nothing in the always-on budget | `ls templates/briefs/`; `python3 scripts/validate.py` |
| `SUB-01.11.1.2` | `[Impl]` | add the stop-on-missing-slot rule to each agent body. The fork is the only place with the information to validate its own brief, and `workflow-implement` already half-states this for `Done when` — generalize that rather than inventing a mechanism | `python3 scripts/validate.py` |
| `SUB-01.11.1.3` | `[Impl]` | delete the three prose brief descriptions now superseded by the templates. `workflow-loop/SKILL.md` is near the compaction cap, so this should buy tokens back rather than cost them | `grep -rc "Out of scope" claude-code/skills/workflow-loop/SKILL.md` reflects the removal; `make verify` |
| `SUB-01.11.1.4` | `[Impl]` | spike whether a `PreToolUse` matcher on `Task`/`Skill` fires the way the `Edit|Write` matcher does. If it does, brief validation can move from the fork to the harness, which is strictly better; if it does not, the fork-side rule above stands alone. Record the result either way | `grep -c "PreToolUse" docs/design-decisions.md` returns non-zero; `make verify` |

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

- **New skill: tech debt tracking** (2026-09-10) — dropped. Was only ever a loose idea (capture/triage/link debt to `BACKLOG.md`); user decided it may not be needed at all rather than refine the scope.
