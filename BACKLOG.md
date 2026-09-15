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

#### [TSK-01.1.7] Nothing enforces that a task's Acceptance Criteria actually map to a subtask, so an unmapped AC deadlocks the task in `backlog-audit`'s Ambiguous bucket indefinitely [P: L] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1:** Given a task whose Acceptance Criteria include one with no subtask proving it, when it is planned or normalized, then the gap is surfaced at write time rather than only discovered later at sweep.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.7.1` | `[Impl]` | surfaced by `/assess-bugs` at TSK-01.1.3's band review: `backlog-modify` and `docs/design-decisions.md` both acknowledge the failure mode ("a backlog defect, not a sweep the gate should quietly let through") but nothing enforces the mapping at write time — `backlog-plan`'s decomposition constraints don't check it, and no lint exists. Recoverable only by a human manually ticking the box. Deliberately not fixed in the same band since it's an acknowledged, intentional trade-off rather than a defect | `bash scripts/validate.sh` |

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

#### [TSK-01.4.14] `install.sh`'s new `core.hooksPath` classification carries a fourth state, `different`, that no branch downstream ever reads [P: M] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1 (Collapse):** Given `scripts/hooks/git/install.sh`, when its `core.hooksPath` classification is read, then it carries only the distinctions some branch below actually tests.
- [ ] **AC-2 (No regression):** Given `scripts/test-install.sh`, when it runs, then `G1`-`G4` pass unchanged.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.4.14.1` | `[Impl]` | filed by `TSK-01.4.13`'s band-review `/assess-simplify` pass: the classification added at `scripts/hooks/git/install.sh:42-58` sets `state="different"` for an existing-but-not-`$HOOK_DIR` path, but every branch below tests only `current` or `stale` — `different` and `unset` take the identical `else` in both `--status` and the install path. The `[ -d ]` stat and the absolute-vs-relative `case` are load-bearing only for the stale distinction. Two flags (`is_current`, `is_stale`) express the same behavior with no dead assignment | `bash scripts/test-install.sh` passes with `G1`-`G4` unchanged; `bash scripts/validate.sh` |

### [TG-01.5] Review Precision & Model Tiering
* **Target Release:** V1
* **Context (2026-09-15):** the `Never invent a finding` prohibition is already in all nine `assess-*` skills and in `reviewer.md`, so the gap is not the missing rule — it is that only `assess-security` states a *positive* evidence bar. A negative rule cannot be complied with; a positive one can. Paired with the model tier, since no prompt change substitutes for the reviewer running on the weaker model.

#### [TSK-01.5.3] The three `assess-*` skills that do not fork as `reviewer` now state an evidence bar with nowhere to put what it excludes [P: M] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1 (Disposal):** Given `assess-testing`, `assess-vulnerabilities`, and `assess-performance`, when each is read, then a near-miss that fails the evidence bar has a named place to go.
- [ ] **AC-2 (No agent coupling):** Given the same three skills, when they run, then the disposal path works without them forking as `reviewer`.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.5.3.1` | `[Impl]` | filed by `TSK-01.5.1`'s band-review `/assess-bugs` pass: `reviewer.md` routes a near-miss to `## Considered and dismissed`, but that instruction reaches only the three skills carrying `context: fork`/`agent: reviewer`. `assess-testing`, `assess-vulnerabilities`, and `assess-performance` gained the bar this round and run inline, so a near-miss they catch is silently dropped rather than preserved. Give each the same disposal section in its own `## Report` contract | `bash scripts/validate.sh`; `make verify` |

#### [TSK-01.5.4] The evidence bar is now written five independent times, with no canonical statement to propagate an edit from [P: L] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1 (Single source):** Given the evidence bar, when it is edited in one place, then no other file needs a matching hand-edit to stay consistent.
- [ ] **AC-2 (AC-4 tension resolved):** Given `TSK-01.5.1`'s AC-4, when this is done, then the decision to keep or drop `assess-bugs`'s own copy is recorded rather than left implicit.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.5.4.1` | `[Impl]` | filed by `TSK-01.5.1`'s band-review `/assess-bugs` and `/assess-simplify` passes, which disagreed: `reviewer.md` and four skills each hand-write their own trigger/path/effect sentence, so an edit to the agent's wording has nothing to propagate from. `/assess-simplify` argued `assess-bugs`'s copy is pure duplication since it forks as `reviewer` and already inherits the agent's text — `assess-simplify`, the third fork skill, correctly got no copy. But `TSK-01.5.1`'s AC-4 explicitly required `assess-bugs` to state one, so removing it contradicts a shipped acceptance criterion. Resolve the tension deliberately: either amend the AC's intent in `docs/design-decisions.md` and drop the inherited copies, or keep them and record why the duplication is accepted | `bash scripts/validate.sh`; `make verify` |

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
| `SUB-01.6.2.1` | `[Impl]` | filed during `TSK-01.6.1`'s rename: `scripts/validate.sh` validates agent and skill frontmatter against the loader's key schema but never cross-checks an `agent:` VALUE against the roster, so the six skills repointed by that rename would have failed silently had any name been mistyped. `git_guard.py`'s reviewer gate has the same shape — it keys on `REVIEWER_AGENT` and falls through to allow if the string stops matching, which is a security surface failing open, not just a broken fork | `bash scripts/validate.sh` fails on a deliberately mistyped `agent:` value and passes on the real tree; `make verify` |

#### [TSK-01.6.3] `docs/design-decisions.md`'s per-role contract rationale covers `architect` but not `tester` [P: L] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1:** Given the paragraph recording which existing output contract each role was weighed against, when it is read, then `tester` is covered like every other agent in the roster.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.6.3.1` | `[Impl]` | filed by `TSK-01.6.1`'s band-review `/assess-simplify` pass: the paragraph exists to record, per role, which existing contract was asked-and-rejected before a new agent was added. It states that for `builder`, `pm`, `reviewer`, and `architect`, but not for `tester` — whose Output template visibly reuses `builder`'s `Result`/`Verified`/`Notes` shape, so the answer is "it did fit, and was reused". One clause | `bash scripts/validate.sh` |

#### [TSK-01.6.4] `AGENTS.md` states a hook regression-case count that the suite has outgrown [P: L] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1:** Given `AGENTS.md`'s line naming the regression-case count, when `scripts/test-hooks.sh` runs, then the two agree.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.6.4.1` | `[Impl]` | pre-existing drift, spotted by a fork during `TSK-01.6.1` and deliberately not fixed there to keep that diff to the lines the rename required: `AGENTS.md` says `259 hook + comments regression cases` while the suite reports 287. Confirm which number is authoritative before editing — the 259 may have been scoped to a subset — and consider whether a hand-maintained count belongs in the file at all | `bash scripts/test-hooks.sh`'s reported count matches `AGENTS.md`; `make verify` |

### [TG-01.7] Cross-Platform Portability
* **Target Release:** V1
* **Context (2026-09-15):** every hook is already Python and `scripts/install.py` already ships, so the remaining non-portable surface is six shell files plus the `Makefile`. The sharp end is the gate itself — `verify_gate.py` resolves `.claude/verify.sh` → `make verify` → `task verify` → `just verify`, and three of those four do not exist on a stock Windows box. No preloaded binaries: `python3` 3.7+ is already the hard floor and a binary would *add* a setup step.

#### [TSK-01.7.1] The toolkit's own gate and test surface are shell and `make`, so neither runs on Windows without Git Bash — the one platform `docs/windows-install.md` explicitly supports [P: M] [TODO]

**User Story:**
> **As an** engineer installing this toolkit on Windows,
> **I want** the repo's own verify and test targets to run natively,
> **So that** the guardrails I just installed actually execute on my machine.

**Acceptance Criteria:**
- [ ] **AC-1 (Gate):** Given `verify_gate.py`, when it resolves a repo's gate, then a Python entry point is among the discovery targets and is tried before the shell and `make` ones.
- [ ] **AC-2 (Suites):** Given the repo's regression suites, when they are run on a machine with only `python3` and `git`, then every one of them executes.
- [ ] **AC-3 (Parity):** Given `make verify` and the Python entry point, when both run on macOS, then they execute the same set of checks.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.7.1.1` | `[Impl]` | port `scripts/validate.sh`, `scripts/test-hooks.sh`, `scripts/test-install.sh`, and `scripts/release.sh` to Python, stdlib only, preserving every existing assertion and the pass counts | `python3 scripts/validate.py`; `python3 scripts/test_hooks.py`; `make verify` |
| `SUB-01.7.1.2` | `[Impl]` | port `setup.sh` to Python, folding it into or alongside the existing `scripts/install.py` rather than maintaining two installers | `python3 scripts/install.py --dry-run` |
| `SUB-01.7.1.3` | `[Impl]` | add the Python entry point to `verify_gate.py`'s discovery order ahead of the shell and `make` targets, and keep `make verify` as a thin wrapper so the macOS path is unchanged | `make verify`; `bash scripts/test-hooks.sh` |
| `SUB-01.7.1.4` | `[Impl]` | decide and record whether `scripts/hooks/git/`'s dispatchers stay shell. They are the one genuinely portable case — `core.hooksPath` hooks run through Git Bash, which ships with Git for Windows — so this is a uniformity call, not a defect fix. Record the decision in `docs/design-decisions.md` either way | `grep -c "hooksPath" docs/design-decisions.md` returns non-zero; `make verify` |

#### [TSK-01.7.2] No skill tells a target repo to keep its own build and verify surface cross-platform, even though this toolkit solved that problem for itself [P: M] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1:** Given `standards-shell`, when it is read, then it states when a script must be Python rather than shell, and that a repo's verify gate must be invocable on every platform the repo claims to support.
- [ ] **AC-2:** Given `standards-cicd`, when it is read, then it covers runner-matrix portability for a repo targeting more than one OS.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.7.2.1` | `[Impl]` | add the portability rules to `standards-shell` and `standards-cicd`. Both are already `paths:`-gated to exactly the files where the rule applies, so this costs nothing in the always-on budget — a new skill would cost a description forever to say something only relevant when a `.sh` or a pipeline config is open | `bash scripts/validate.sh` |

### [TG-01.8] UI & Language Standards
* **Target Release:** V1
* **Context (2026-09-15):** `standards-ui-design` and `standards-ui-security` are web-only skills wearing generic names — the security one covers `postMessage`, iframes, and `frame-ancestors`, all browser. Merging design and security per platform follows the precedent the language skills already set (`assess-security` reads a Security section out of `standards-go`, and does not load a separate skill for it), and native-only mobile removes the one unsolved activation problem: with React Native out of scope, `**/*.tsx` is unambiguously web again. `standards-swift` and `standards-kotlin` landed 2026-09-15, defaulted off.

### [TG-01.9] Accessibility & Visual Output
* **Target Release:** V1
* **Context (2026-09-15):** `config-accessibility` governs formatting — heading depth, density caps, emoji placement — while every skill's Report section still contracts for a markdown table. The rules say show-don't-tell and the output contracts say emit-a-table. Since a terminal cannot render a diagram, visual output means a hosted artifact, which is a real shift in where this toolkit's output lives and is why this is a task group rather than a single skill.

#### [TSK-01.9.1] Every skill's output contract is a markdown table, so `config-accessibility`'s show-don't-tell rule is contradicted by the contracts it is supposed to govern [P: M] [TODO]

**User Story:**
> **As a** maintainer who reads structure faster than prose,
> **I want** the toolkit's own outputs to default to diagrams where a diagram is clearer,
> **So that** I am not re-deriving a graph from a table every time I plan or review.

**Acceptance Criteria:**
- [ ] **AC-1 (Rule):** Given `config-accessibility`, when it is read, then it states a visual-first rule — an output carrying a dependency relation or more than a handful of entities ships a diagram, not only prose.
- [ ] **AC-2 (Contracts):** Given `workflow-decompose` and the `assess-*` skills, when their Report sections are read, then each names a diagram form alongside its table.
- [ ] **AC-3 (Surface):** Given the visual-first rule, when it names where a diagram is rendered, then it distinguishes inline Mermaid from a published artifact and says which applies when.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.9.1.1` | `[Impl]` | add the visual-first rule to `config-accessibility`, including the inline-vs-artifact distinction — a terminal cannot render a diagram, so the rule has to say where the visual actually goes rather than assuming markdown is enough | `grep -c "visual-first\|diagram" claude-code/skills/config-accessibility/SKILL.md` returns non-zero; `bash scripts/validate.sh` |
| `SUB-01.9.1.2` | `[Impl]` | add a diagram form to `workflow-decompose`'s queue output and to the `assess-*` report contracts | `bash scripts/validate.sh`; `make verify` |

#### [TSK-01.9.2] There is no single view of a repo's whole work state — what is planned, what shipped, what is left lives split across `BACKLOG.md` and `CHANGELOG.md` with no way to see it at once (after: "Every skill's output contract is a markdown table") [P: M] [TODO]

**User Story:**
> **As a** maintainer planning a release,
> **I want** one high-level map of everything planned, done, and remaining,
> **So that** I can see the shape of the work without reading two files end to end.

**Acceptance Criteria:**
- [ ] **AC-1 (Sources):** Given the map, when it is generated, then every node derives from `BACKLOG.md` or `CHANGELOG.md` and nothing is hand-maintained.
- [ ] **AC-2 (Level):** Given the map, when it is read, then nodes are epics and task groups, not individual subtasks.
- [ ] **AC-3 (State):** Given the map, when it is read, then shipped, open, and blocked work are visually distinct, and the released version is shown.
- [ ] **AC-4 (Zero setup):** Given a machine with only `python3`, when the map is generated, then it renders with no install step.
- [ ] **AC-5 (Not a record):** Given the map, when it is described in its own skill, then it is stated to be a view over the two record files and never a source of truth.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.9.2.1` | `[Impl]` | write the parser — stdlib Python, reads `BACKLOG.md` and `CHANGELOG.md`, emits the graph as JSON. This is the load-bearing half; the renderer is swappable and the data model should carry position-independent nodes so a 2D or 3D renderer can consume the same output | `python3` parser run against this repo's own two files exits zero and emits nodes for every epic and task group |
| `SUB-01.9.2.2` | `[Impl]` | write the skill that renders the JSON as a published artifact, 2D first with the data model 3D-ready. Force-graph or three.js from the CDN the artifact CSP already allows — no build step, no install. State in the skill that the artifact is a view, never a record | `bash scripts/validate.sh` |

### [TG-01.10] Parallelism & Loop Latency
* **Target Release:** V1
* **Context (2026-09-15):** a simple edit currently costs 30–60 minutes, and `TG-01.4`'s own context names why — a one-task band runs 7 serial forks, and `verify_gate.py` fires on every builder Stop, so an N-task band runs the full gate N times. Parallelism does not fix that; it fixes the multi-task band. The fast path and the band-level gate are the levers that return time on the case that actually hurts. Parallelism is opportunistic by decision: the planner proves disjointness or the band runs serial.

#### [TSK-01.10.1] A one-line change pays the full five-phase machine, so trivial edits cost 7 serial forks and 30–60 minutes of wall clock [P: H] [TODO]

**User Story:**
> **As an** engineer making a small, obvious change,
> **I want** the loop to skip the phases that exist for large bands,
> **So that** the process cost is proportional to the change.

**Acceptance Criteria:**
- [ ] **AC-1 (Predicate):** Given `workflow-loop`, when the fast path is described, then its entry condition is stated as a checkable property of the diff, not a judgment call.
- [ ] **AC-2 (Phases):** Given a change meeting that condition, when the loop runs, then P1 decompose, P2.0 align, and P3 consolidate are skipped and the remaining phases are unchanged.
- [ ] **AC-3 (Escape):** Given a change that touches a security boundary, when the fast path is evaluated, then it is refused regardless of diff size.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.10.1.1` | `[Impl]` | add the fast path to `workflow-loop/SKILL.md`, modelled on the existing `open` hatch — that hatch already establishes that not every change deserves the full machine. The file sits near the 5000-token compaction cap, so this addition must net-shrink or hold | `bash scripts/validate.sh`; `make verify` |

#### [TSK-01.10.2] `verify_gate.py` fires on every builder Stop, so an N-task band runs the repo's full gate N times for one merged result [P: M] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1:** Given a multi-task band, when it completes, then the repo's full gate has run once against the band's merged state rather than once per task.
- [ ] **AC-2:** Given a single task's `Done when` commands, when its fork finishes, then those still run per-task — the band gate replaces the full-suite run, not the task's own proof.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.10.2.1` | `[Impl]` | move the full-gate run from per-fork Stop to a band-level check at P2.2. Keep the per-task `Done when` commands where they are; the task's own proof is not the thing being deduplicated | `bash scripts/test-hooks.sh`; `make verify` |

#### [TSK-01.10.3] Parallel execution is unavailable even where two tasks provably cannot collide, because nothing computes which files a task will touch (after: "The agent roster is named after workflow phases rather than roles") [P: M] [TODO]

**User Story:**
> **As an** orchestrator running a multi-task band,
> **I want** provably disjoint tasks to run at the same time,
> **So that** a band's wall clock reflects its widest dependency chain rather than its task count.

**Acceptance Criteria:**
- [ ] **AC-1 (Manifest):** Given a decomposed queue, when `pm` returns it, then each task carries the set of files it is expected to touch.
- [ ] **AC-2 (Default):** Given two tasks whose manifests intersect, or a queue where the manifest is unavailable, when the band runs, then it runs serial — parallelism is opt-in on proof, never the default.
- [ ] **AC-3 (Isolation):** Given tasks selected to run in parallel, when they execute, then each runs in its own worktree and the orchestrator merges before the band gate.
- [ ] **AC-4 (Scope):** Given the parallel design, when it is documented, then it is scoped to one branch — never across PRs or branches.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.10.3.1` | `[Impl]` | teach `pm` to return a files-touched manifest per task, and `workflow-loop` P2.0 to use manifest disjointness as the parallel predicate rather than today's weaker no-shared-file rule | `bash scripts/validate.sh` |
| `SUB-01.10.3.2` | `[Impl]` | add the worktree-per-group execution and the orchestrator merge step, gated on the manifest proof, defaulting to serial. Read-only fan-out (`assess-*`, `researcher`, `scout`) needs no worktree and should be documented as always-parallel — that half is free today and under-used | `make verify` |

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
| `SUB-01.11.1.1` | `[Impl]` | write one `templates/briefs/<role>.md.tmpl` per role. These live outside any loaded path, so they cost nothing in the always-on budget | `ls templates/briefs/`; `bash scripts/validate.sh` |
| `SUB-01.11.1.2` | `[Impl]` | add the stop-on-missing-slot rule to each agent body. The fork is the only place with the information to validate its own brief, and `workflow-implement` already half-states this for `Done when` — generalize that rather than inventing a mechanism | `bash scripts/validate.sh` |
| `SUB-01.11.1.3` | `[Impl]` | delete the three prose brief descriptions now superseded by the templates. `workflow-loop/SKILL.md` is near the compaction cap, so this should buy tokens back rather than cost them | `grep -rc "Out of scope" claude-code/skills/workflow-loop/SKILL.md` reflects the removal; `make verify` |
| `SUB-01.11.1.4` | `[Impl]` | spike whether a `PreToolUse` matcher on `Task`/`Skill` fires the way the `Edit|Write` matcher does. If it does, brief validation can move from the fork to the harness, which is strictly better; if it does not, the fork-side rule above stands alone. Record the result either way | `grep -c "PreToolUse" docs/design-decisions.md` returns non-zero; `make verify` |

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

- **New skill: tech debt tracking** (2026-09-10) — dropped. Was only ever a loose idea (capture/triage/link debt to `BACKLOG.md`); user decided it may not be needed at all rather than refine the scope.
