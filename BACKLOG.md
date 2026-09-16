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
- [x] **AC-1:** Given `make verify`, when it runs, then `scripts/test_install.py` executes and a failure in it fails the target.
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

#### [TSK-01.1.11] `scripts/validate.py` opens and re-reads every `SKILL.md` five separate times per run, inside the gate that fires on every dirty `builder` Stop [P: M] [TODO]

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.11.1` | `[Impl]` | filed by this band's `/assess-simplify`. `check_frontmatter`, `check_document_names`, `check_reachability`, `check_section_order` and `check_budget` each walk `claude-code/skills/` and read every `SKILL.md` from scratch — roughly 67 skills x 5 opens — and `check_reachability` parses frontmatter a second time for a file whose full text it already holds. Four of the five reads predate this band, which added the fifth, so this is pre-existing architecture rather than a defect the band introduced — filed, not fixed in-band. One `load_skills()` pass returning `{name: (body, head, fm)}` collapses it. The `head = body.split("---")[1]` + `DMI.search(head)` pair is also written out verbatim twice | `python3 scripts/verify.py` |

#### [TSK-01.1.12] `scripts/validate.py`'s eight check functions hand-roll the same section-header and verdict idiom, and have already drifted into two incompatible return contracts [P: L] [TODO]

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.12.1` | `[Impl]` | filed by this band's `/assess-simplify`. `check_links` and `check_hooks` return a raw problem count while the other six return 0-or-1, and `main` sums both into one `%d problem(s)` line that therefore means two different things. This band added the eighth hand-written copy of the idiom. A `section(name)` helper plus a `verdict(failures, ok_message) -> int` forces one contract so the ninth check cannot invent a third | `python3 scripts/verify.py` |

#### [TSK-01.1.13] `scripts/evals.py` hand-rolls `shutil.which` and types out a skill list that is already derivable from disk [P: L] [TODO]

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.13.1` | `[Impl]` | filed by this band's `/assess-simplify`. `claude_on_path()` walks `os.environ["PATH"]` by hand — `shutil.which`, already imported in `scripts/install.py`, does it and handles `PATHEXT` on Windows. Separately, `COVERED_SKILLS` hard-codes five names whose `evals/` directories are on disk and can be scanned, so `--list` prints what someone remembered to add rather than what will actually run; the per-skill reason prose belongs only in `docs/design-decisions.md`, which already carries it | `python3 scripts/evals.py --list` |

#### [TSK-01.1.14] `scripts/validate.py`'s new orphan-skill check keys on `argument-hint:` as a proxy for slash reachability, never on `user-invocable` [P: L] [TODO]

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.14.1` | `[Impl]` | filed by this band's `/assess-bugs`. A skill that is `disable-model-invocation: true`, path-ungated, and takes no arguments — so legitimately carries no `argument-hint:` — would be reported unreachable and fail the gate, blocking a `builder` fork's Stop, even though the user reaches it by typing `/name`. The three skills the check must not flag (`assess-change-risk`, `assess-code-conventions`, `repo-assess`) pass only because they happen to carry `argument-hint`. The proxy was chosen deliberately and verified against the current tree, but `user-invocable` is the field that actually decides it | `python3 scripts/validate.py` |

#### [TSK-01.1.15] `scripts/evals.py` tests `--list` against raw argv, so it wins over the `--` passthrough its own header documents [P: L] [TODO]

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.15.1` | `[Impl]` | filed by this band's `/assess-bugs`. `python3 scripts/evals.py -- --list` — the forwarding form the file's own header documents — matches the `"--list" in argv` test before the `--` branch is ever reached, so it prints the covered set and exits 0 instead of forwarding `--list` to `claude plugin eval` | `python3 scripts/evals.py --list` |

#### [TSK-01.1.17] The repo ships no `LICENSE` and no `SECURITY.md`, while its installer symlinks the clone into `~/.claude` where every file executes as a hook [P: H] [TODO]
| Field | Value |
|---|---|
| Owner | `human` |

**User Story:**
> **As** someone evaluating whether this toolkit can be adopted at work,
> **I want** stated licensing terms and a disclosure path,
> **So that** installing it is a decision with known terms rather than an unreviewable one.

**Acceptance Criteria:**
- [ ] **AC-1 (Licensed):** Given the repo root, when it is read, then a `LICENSE` file states the terms under which it may be used and redistributed.
- [ ] **AC-2 (Disclosure path):** Given `SECURITY.md`, when it is read, then it names how to report a vulnerability and what the install's execution surface is.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.17.1` | `[Impl]` | the licence choice is a policy call, not an agent one — this subtask is blocked on the user naming it, then writing the file verbatim from that licence's canonical text. `README.md` also names no terms today | `test -f LICENSE` |
| `SUB-01.1.17.2` | `[Impl]` | write `SECURITY.md`: how to report, and what an install actually grants. `scripts/install.py` symlinks `claude-code/` into `~/.claude`, so every file under `hooks/` runs as a `PreToolUse`/`PostToolUse`/`SessionStart` command on every tool call, and an upstream sync moves every session on the next start unless `--ref` pinned it. State the `--ref` pin as the mitigation it is | `test -f SECURITY.md`; `python3 scripts/validate.py` |

#### [TSK-01.1.18] Every generated hook command names its interpreter by bare `PATH` name, so a directory ahead of `python3` on `PATH` displaces the `PreToolUse` guard on every tool call [P: H] [TODO]

**User Story:**
> **As** someone whose `PATH` includes a project-local or user-writable bin directory,
> **I want** the installed hook commands to name the interpreter they were resolved against,
> **So that** a shadowing binary cannot silently replace the guard that gates every Bash call.

**Acceptance Criteria:**
- [ ] **AC-1 (Resolved, not named):** Given a generated `settings.json`, when a hook command is read, then its interpreter is the absolute path `resolve_interpreter` found, not a bare name.
- [ ] **AC-2 (Decision recorded):** Given `check_python`'s existing comment that baking the bare name is intended, when this changes, then `docs/design-decisions.md` records why the trade flipped.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.18.1` | `[Impl]` | filed by `TSK-01.7.1`'s band-review `/assess-security` pass. `resolve_interpreter` (`scripts/install.py:62-68`) calls `shutil.which(candidate[0])` purely as an existence test and returns the bare-name list, discarding the absolute path it just resolved; that list reaches `hook_command:123` and lands in `~/.claude/settings.json` as `python3 /abs/.claude/hooks/git_guard.py`. Any directory an attacker or a careless install can write that precedes the real interpreter — a project `.venv/bin`, a global npm bin, `~/.local/bin` — then runs on every `PreToolUse` Bash call, every `PostToolUse` Edit/Write and every `SessionStart`, and exiting 0 turns `git_guard.py`'s decision into a blanket allow. `check_python:114-115` documents the bare name as deliberate, so this is a recorded trade to revisit rather than an oversight; weigh it against the portability reason the comment gives before changing it | `python3 scripts/test_install.py` covers a generated command naming an absolute interpreter; `python3 scripts/verify.py` |

#### [TSK-01.1.19] `build_settings` and its new test helper carry avoidable duplication, and a trailing-argument parameter whose annotation, default and body disagree [P: L] [TODO]

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.19.1` | `[Impl]` | filed by `TSK-01.7.1`'s band-review `/assess-simplify` pass. `scripts/install.py:134-145` traverses the token list twice to learn two facts about one token — a generator with a nested `any()` finds the index, then a second defaultless `next()` re-tests the same token against `HOOK_NAMES` — writing the `token.endswith(n + ".py")` predicate twice. One generator yielding the `(index, name)` pair collapses it to a single scan with the predicate written once | `python3 scripts/test_install.py`; `python3 scripts/verify.py` |
| `SUB-01.1.19.2` | `[Impl]` | `scripts/install.py:121-122`'s `trailing_args: list = ()` annotates a list, defaults to a tuple, and pays `list(trailing_args)` in the body to reconcile them; the default is never used, since `build_settings:148` is the only caller and always passes a list. Drop the default and the coercion | `python3 scripts/test_install.py` |
| `SUB-01.1.19.3` | `[Impl]` | `scripts/test_install.py`'s new `load_json` exists for one call and forces a `None`-sentinel branch at the call site, re-expanding a ladder the neighbouring cases express as one `problem` string through `record`. Give `first_bad_hook_command` the path instead of the parsed dict and let a parse failure become one more returned problem string | `python3 scripts/test_install.py` |
| `SUB-01.1.19.4` | `[Impl]` | `scripts/install.py:134`'s `shlex.split` raises `ValueError` on an unbalanced quote in a source hook command, which the replaced `str.endswith` test never could. It is reachable only from the repo's own tracked `claude-code/settings.json` and lands before `write_settings`, so the target keeps its previous file — an availability nuisance on a corrupt trusted file, not an attacker path. Decide whether to catch it and skip the entry, as the old test did silently, or let it fail loudly | `python3 scripts/test_install.py` |

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

#### [TSK-01.4.15] `comments.py hook` is now registered twice for a `builder` fork, and only one registration is needed [P: L] [TODO]
| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.4.15.1` | `[Impl]` | the global `PostToolUse` registration added in `claude-code/settings.json` closes the hole where a primary session editing code got no comment feedback at all — `auto_format.py` was global, `comments.py hook` was not. `claude-code/agents/builder.md`'s own frontmatter registration was kept alongside it rather than removed, because whether a `settings.json` `PostToolUse` hook fires for a subagent's tool calls was not verified in-session, and a silent loss of feedback on the primary author path is a worse failure than a duplicated advisory line. Verify which way it actually behaves, then drop the redundant one and correct `AGENTS.md`'s hook table, which currently documents both | a `builder` fork editing one file is observed emitting the comment findings once or twice, recorded either way; the redundant registration is removed; `python3 scripts/validate.py` |

#### [TSK-01.4.14] `install.sh`'s new `core.hooksPath` classification carries a fourth state, `different`, that no branch downstream ever reads [P: M] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1 (Collapse):** Given `scripts/hooks/git/install.sh`, when its `core.hooksPath` classification is read, then it carries only the distinctions some branch below actually tests.
- [ ] **AC-2 (No regression):** Given `scripts/test_install.py`, when it runs, then `G1`-`G4` pass unchanged.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.4.14.1` | `[Impl]` | filed by `TSK-01.4.13`'s band-review `/assess-simplify` pass: the classification added at `scripts/hooks/git/install.sh:42-58` sets `state="different"` for an existing-but-not-`$HOOK_DIR` path, but every branch below tests only `current` or `stale` — `different` and `unset` take the identical `else` in both `--status` and the install path. The `[ -d ]` stat and the absolute-vs-relative `case` are load-bearing only for the stale distinction. Two flags (`is_current`, `is_stale`) express the same behavior with no dead assignment | `python3 scripts/test_install.py` passes with `G1`-`G4` unchanged; `python3 scripts/validate.py` |

#### [TSK-01.4.16] Read-only git is prompt-only for five of seven agents, and each of their files says so outright [P: M] [TODO]

**User Story:**
> **As** whoever relies on a fork not touching history,
> **I want** the read-only git boundary enforced by the hook that already reads agent identity,
> **So that** the rule holds when a fork's own instructions are ignored, not only when they are followed.

**Acceptance Criteria:**
- [ ] **AC-1 (Enforced):** Given a `builder`, `tester`, `scout`, `researcher`, or `architect` fork, when it invokes a history-mutating git command, then `git_guard.py` denies it.
- [ ] **AC-2 (Read path intact):** Given the same agents, when they invoke `log`, `diff`, `show`, `status`, `blame`, `rev-parse`, or `ls-files`, then the command is permitted.
- [ ] **AC-3 (Orchestrator unaffected):** Given a primary session, when it commits or pushes, then nothing added here denies it.
- [ ] **AC-4 (Docs match):** Given those five agent files, when their git bullet is read, then it no longer states that nothing enforces the rule.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.4.16.1` | `[Impl]` | `builder.md`, `tester.md`, `scout.md`, `researcher.md` and `architect.md` each carry the line "Nothing enforces this — `git_guard.py` permits those globally, so it holds only because this file says so." `TSK-01.4.9` already shipped the mechanism: a deny-by-default gate keyed on agent identity through `lib/agents.py`, applied to `reviewer`. Extend it to a per-agent allowed-subcommand set rather than a second parallel gate, and keep the primary session ungated — P2.2 commits there | `python3 scripts/test_hooks.py` |
| `SUB-01.4.16.2` | `[UnitTest]` | cases per agent, both directions: a mutating subcommand denied and a read-only one permitted, for all five. Include the orchestrator case — no agent identity in the payload means permitted — so the gate cannot regress into blocking the loop's own commits | `python3 scripts/test_hooks.py` |
| `SUB-01.4.16.3` | `[Impl]` | correct the five agent files' git bullet to state the mechanism, and `AGENTS.md`'s hook table where it describes what `git_guard.py` denies | `python3 scripts/validate.py`; `python3 scripts/verify.py` |

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

#### [TSK-01.5.5] Two questions `docs/design-decisions.md` records as OPEN both need evidence no live session can produce [P: M] [TODO]

**User Story:**
> **As** whoever next edits brief validation or model tiering,
> **I want** both open questions answered from a source outside a live session,
> **So that** the current design is a chosen one rather than the only one that could be reached.

**Acceptance Criteria:**
- [ ] **AC-1 (Matcher settled):** Given the `PreToolUse`-on-`Task`/`Skill` question, when it is answered, then `docs/design-decisions.md` records the answer and its source.
- [ ] **AC-2 (Override settled):** Given the skill-level `model:` override question, when it is answered, then the same file records the answer and its source.
- [ ] **AC-3 (Consequence stated):** Given either answer, when it changes what the current design should be, then the follow-up is filed rather than left implied.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.5.5.1` | `[Audit]` | whether a `PreToolUse` matcher fires on `Task`/`Skill` decides where brief validation belongs. Today it lives in seven agent bodies, which means the orchestrator pays a full fork round trip to learn something decidable at dispatch. The session that tried to test it refused, correctly — registering the matcher means editing the live `settings.json` it is running under. Answer it from the Claude Code hook documentation or changelog, or from a throwaway repo opened as a fresh session; if the matcher fires, file the move and demote the agent-body rules to fallback | `grep -q 'PreToolUse' docs/design-decisions.md` and the section no longer reads `OPEN` |
| `SUB-01.5.5.2` | `[Audit]` | whether a skill-level `model:` on a `context: fork` skill overrides the fork agent's frontmatter decides whether per-skill tiering is available at all. The in-session test failed by design: `reviewer` classified the self-report request as prompt injection and returned that as its finding, which is the hardening wanted. Do not re-run that prompt; answer from SDK or platform documentation | the same file records the answer and its source, and the section no longer reads `OPEN` |

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

#### [TSK-01.7.6] `scripts/test_hooks.py` fails on Windows across two subsystems, so that leg cannot gate until it is ported [P: H] [TODO]

**User Story:**
> **As** someone relying on the Windows CI leg,
> **I want** the hook suite to run there,
> **So that** a leg that reports red for known reasons either goes green or stops pretending to gate.

**Acceptance Criteria:**
- [ ] **AC-1 (Green or skipped):** Given the `windows-latest` leg, when `scripts/test_hooks.py` runs, then every case either passes or reports a skip naming its missing precondition.
- [ ] **AC-2 (Leg gates):** Given the matrix, when the Windows leg finishes, then its result blocks the merge like every other leg.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.7.6.1` | `[Impl]` | surfaced by `TSK-01.7.1`'s first real CI run — the first time this suite has ever executed on anything but Linux. Three clusters fail on `windows-latest`: `G206`/`G207` (the `gh api` threaded-reply cases), `G137` (cwd outside any repo versus an absolute target landing inside one — a path-shape assumption), and `VG1`-`VG11`, the entire `verify_gate.py` block. Each is a POSIX assumption rather than a real defect in the code under test, but the count is large enough that porting them is its own task rather than a fix folded into the band that found them. Establish per cluster whether the right answer is a port or a documented skip; a skip must name its precondition the way `test_install.py`'s Windows guards now do | `python3 scripts/test_hooks.py` reports zero failures on `windows-latest` |
| `SUB-01.7.6.2` | `[Impl]` | until `SUB-01.7.6.1` lands, the Windows leg reports red on every pull request for known reasons. Decide deliberately between leaving it red and visible, or marking it `continue-on-error` with a comment naming this task — and if `continue-on-error` is chosen, flipping it back is this task's `AC-2` and must not be forgotten | the workflow states which legs gate, and `python3 scripts/validate.py` exits zero |

#### [TSK-01.7.7] `G210` reads ambient `gh` state, so the hook suite's result depends on whether the checkout has an open PR [P: M] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1:** Given any machine, when `scripts/test_hooks.py` runs, then `G210`'s result does not depend on the ambient `gh` binary's authentication or on whether the current branch has an open pull request.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.7.7.1` | `[Impl]` | surfaced while fixing `TSK-01.7.1`'s CI failures. `current_branch_pr_number()` (`claude-code/hooks/git_guard.py:944`) shells out to `gh pr view` with no `cwd` override, so it inherits the real process cwd — the actual checkout, not `test_hooks.py`'s fixture repo. `G210` (`scripts/test_hooks.py:1532`) uses the plain `bash_case` helper and never stubs `gh`, unlike `G206`-`G209` which use `gh_stub_env`. Verified this session: `G210` fails on a machine where `gh` is authenticated and the branch has an open PR, and passes under `GH_CONFIG_DIR=<empty>`, which is CI's state since the workflow sets no `GH_TOKEN`. The suite is therefore not hermetic, and it went red locally the moment this branch's own PR was opened. Likely fix is threading `cwd` through to that `gh` call, or giving `G210` its own `gh_stub_env` | `python3 scripts/test_hooks.py` passes with `gh` authenticated and the branch carrying an open PR |

#### [TSK-01.7.3] The `verify` workflow pulls its container and its actions by mutable tag, and persists the job token into a tree that then executes PR-authored code [P: M] [TODO]

**User Story:**
> **As** whoever relies on this repo's CI not being the weak link,
> **I want** the gate's own supply chain pinned and its token not left in the workspace,
> **So that** a re-pushed tag or a malicious pull request cannot reach the job that installs this toolkit.

**Acceptance Criteria:**
- [ ] **AC-1 (Pinned):** Given the workflow, when its container image and action references are read, then each names an immutable digest or commit SHA.
- [ ] **AC-2 (No persisted token):** Given a `pull_request` run, when PR-authored code executes, then no usable credential remains in the checkout.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.7.3.1` | `[Impl]` | filed by `TSK-01.7.1`'s band-review `/assess-security` pass. The floor leg's `container: "python:3.7"` is a mutable Docker Hub tag, and every step in that leg — the repo's own installer included — runs as root inside whatever image that tag currently serves. `python:3.7` is also past end-of-life, so its base layers are no longer rebuilt. Pin by digest; note this interacts with `TSK-01.7.2`, which may remove the leg entirely | `grep -q 'python:3.7@sha256:' .github/workflows/verify.yml`, or the leg is gone per `TSK-01.7.2` |
| `SUB-01.7.3.2` | `[Impl]` | `actions/checkout@v4` leaves `persist-credentials` at its default of `true`, writing an `x-access-token` extraheader into `$GITHUB_WORKSPACE/.git/config`; the steps that follow run `scripts/install.py` and `scripts/verify.py` out of the PR's own tree, and `verify.py` fans out to four more PR-controlled scripts. `permissions: contents: read` caps the blast radius, which is why this is Medium, but the band widened the window from one job to four. Set `persist-credentials: false` | `grep -q 'persist-credentials: false' .github/workflows/verify.yml` |
| `SUB-01.7.3.3` | `[Impl]` | `actions/checkout@v4` and `actions/setup-python@v5` both resolve at run time to whatever commit their major tag points at. Both are GitHub-owned, which is why this is the lowest of the three, but the tj-actions/changed-files incident is the precedent. Pin both to a commit SHA with the version in a trailing comment | both action references in `.github/workflows/verify.yml` name a 40-character SHA |

#### [TSK-01.7.4] The only merge gate runs no secret scan and no security static analysis [P: M] [TODO]

**Acceptance Criteria:**
- [ ] **AC-1:** Given a pull request adding a hardcoded credential, when the `verify` workflow runs, then the check fails rather than merging ungated.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.7.4.1` | `[Impl]` | filed by `TSK-01.7.1`'s band-review `/assess-security` pass, and pre-existing rather than introduced by it. `scripts/verify.py`'s four checks are the hook regression suite, the config validator, the comment lint and the install regression suite; none reads for a credential or a security rule. `standards-cicd` makes the secret and static-analysis scans blocking on the merge. Dependency scanning does not apply — the repo ships no manifest or lockfile. Decide whether the scan belongs in the workflow or in `scripts/verify.py`, remembering that `verify.py` also runs on every dirty `builder` Stop and is currently an 11-second gate | the `verify` workflow fails a branch carrying a planted test credential; `python3 scripts/verify.py` |

#### [TSK-01.7.5] The workflow's matrix carries a `python-version` key whose value is one constant, and the new `install` Makefile target collides with the GNU convention [P: L] [TODO]

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.7.5.1` | `[Impl]` | filed by `TSK-01.7.1`'s band-review `/assess-simplify` pass. `python-version` is the same literal on all three legs that read it and absent on the fourth, whose `setup-python` step is skipped anyway, so the matrix key buys an indirection over a constant. Dropping it leaves three bare `- os:` legs and the literal on the step. Weigh the lost self-documentation at the matrix before taking it | `python3 scripts/validate.py` |
| `SUB-01.7.5.2` | `[Impl]` | `Makefile`'s new `install` target runs the install *test suite*, not `scripts/install.py`, so `make install` exits zero having installed nothing — a collision with the GNU convention that the sibling `test`/`validate`/`comments` targets do not have. Rename it, and update the root `AGENTS.md` "Working on this repo" list, which does not mention it either way | `make -n install` runs the suite under a name that does not read as an installer; `python3 scripts/validate.py` |

#### [TSK-01.7.2] The declared Python 3.7 floor is three years past end-of-life and no hosted runner image can provide it, so CI tests it only inside a container [P: M] [TODO]
| Field | Value |
|---|---|
| Owner | `human` |

**User Story:**
> **As** whoever maintains this repo's CI,
> **I want** the declared interpreter floor to be one the platform can still supply,
> **So that** testing the floor does not depend on a container leg standing in for a runner that no longer exists.

**Acceptance Criteria:**
- [ ] **AC-1 (Decided):** Given the floor, when the decision is recorded, then `docs/design-decisions.md` states whether it stays at 3.7 or rises, and why.
- [ ] **AC-2 (Consistent):** Given whichever floor is chosen, when `AGENTS.md`, `scripts/install.py`'s floor check, and the CI matrix are read, then all three name the same version.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.7.2.1` | `[Audit]` | surfaced by `TSK-01.7.1`'s round-2 implement pass. Verified live this session: `actions/setup-python` publishes no 3.7 build for Ubuntu 24.04, which is what `ubuntu-latest` resolves to, and the `ubuntu-22.04` label that used to carry one entered deprecation on 2026-09-17 with job-failing brownouts and full removal in April 2027. `TSK-01.7.1` therefore tests the floor in a `python:3.7` container, which is correct but is a workaround for a floor the platform has stopped supporting — 3.7 has been end-of-life since June 2023. The floor exists only because `subprocess.run(capture_output=...)` needs 3.7; nothing in this repo requires that it stay there. Raising it drops the container leg and simplifies the matrix, but narrows who can install without a newer interpreter, which is why this is a human call rather than an agent one | `docs/design-decisions.md` records the decision, and `python3 scripts/validate.py` exits zero |

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

#### [TSK-01.10.5] Loop phase timing is session-stated only, so this task group's own latency claim has no recorded measurement behind it [P: L] [TODO]

**User Story:**
> **As** whoever decides which latency lever to pull next,
> **I want** phase and fork wall-clock to accumulate somewhere durable,
> **So that** a change to the loop is measured against a baseline instead of argued from impression.

**Acceptance Criteria:**
- [ ] **AC-1 (Durable):** Given a completed loop run, when its timing is read, then it survives the session that produced it.
- [ ] **AC-2 (Not in context):** Given the record, when a session starts, then reading it is opt-in and costs nothing in the always-on budget.
- [ ] **AC-3 (Baseline stated):** Given at least one recorded run, when this task group's context is read, then its latency claim cites a measurement.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.10.5.1` | `[Impl]` | `workflow-loop/SKILL.md`'s guardrail has the orchestrator note each phase's and each fork's start and end wall-clock "session-stated, not a file", which is the same pattern as the retry counter and correct for a counter that only has to survive one band. Timing is different: its whole value is comparison across runs, and nothing accumulates today. This task group's own context asserts "a simple edit currently costs 30-60 minutes" with no run behind it. Decide where the record lives — a gitignored path under the OS temp dir keyed like the band-gate marker is the cheapest shape that does not touch the repo — then record it and cite a real run in the task group's context | the timing record survives a session end and `TG-01.10`'s context cites a measured run; `python3 scripts/validate.py` |

### [TG-01.11] Outcome Evidence
* **Target Release:** V1
* **Context (2026-09-15):** every check this repo runs proves the harness is *well-formed* — `test_hooks.py` proves the hooks behave, `validate.py` proves the prompt graph resolves, the budget check proves it is cheap. Nothing proves it produces better work than a bare session. Every design decision rests on the reasoning in `docs/design-decisions.md` rather than a measured effect, which is honest but means a regression in skill quality is invisible until someone notices bad output. The `skill-creator` plugin is already enabled in `settings.json` and ships eval and variance tooling; no skill has an eval suite. This is the single largest gap between this toolkit and one that can be trusted to stay good as it changes.

#### [TSK-01.11.1] No skill has an eval suite, so skill quality is unmeasured and a regression is only visible as bad output someone happens to notice [P: H] [TODO]

**User Story:**
> **As** whoever edits a high-traffic skill next,
> **I want** a suite that tells me whether the edit made its output better or worse,
> **So that** a prompt change is a measured change rather than an argued one.

**Acceptance Criteria:**
- [x] **AC-1 (Traffic-ranked):** Given the skill library, when the eval targets are chosen, then they are the highest-traffic skills, named with the reason each was picked.
- [x] **AC-2 (Runnable cold):** Given a clean checkout, when the eval command runs, then it executes without manual setup beyond what `README.md` already requires.
- [ ] **AC-3 (Baseline recorded):** Given each covered skill, when its suite first runs, then its score and variance are recorded as the baseline to regress against. **Not done (2026-09-15):** the runner and all five suites shipped this band, but no eval has actually been run — no score or variance is recorded anywhere. Only the baseline run is outstanding; this task stays open until it lands.
- [x] **AC-4 (Gate decided):** Given the baseline, when the gate question is settled, then `docs/design-decisions.md` records whether evals block `verify` or run on demand, and why.

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.11.1.1` | `[Audit]` | pick the covered set before writing anything. Rank by how often a skill actually runs and how much damage a silent regression does — `workflow-loop`, `backlog-modify`, `git-pr-create`, `write-comments`, and the `reviewer`-forked `assess-*` trio are the obvious candidates, but confirm against real usage rather than intuition. Cap the first pass at five: an eval suite nobody maintains is worse than none. Record the set and the reason for each under a `## Skill eval coverage` heading | `grep -q '^## Skill eval coverage' docs/design-decisions.md` |
| `SUB-01.11.1.2` | `[Impl]` | build the suites against the `claude plugin eval` interface already on this machine, not an invented harness. Verified shape: cases are `<eval dir>/**/case.yaml`, or `prompt.md` plus `graders/*.md`, defaulting to `evals/`; the target resolves a path, a plugin name, or `plugin@marketplace`; `--trust-plugin` answers the first-run trust prompt so a non-interactive run does not hang; `--ablation with-without` adds the no-plugin baseline arm that makes a score meaningful. Add `scripts/evals.py` as this repo's own entry point so the invocation lives in one place the way `scripts/verify.py` already does, and confirm how a plain skills directory resolves as a target before committing to the path form | `python3 scripts/evals.py --list` |
| `SUB-01.11.1.3` | `[Impl]` | run each suite, record its baseline score and variance beneath the same heading, and record the gate decision. `verify` is 9.2 s today and runs on every dirty `builder` Stop, so folding model-calling evals into it would change that gate's cost and latency character completely — default to a separate on-demand command and state that plainly rather than leaving it implied. Keep `scripts/evals.py` out of `scripts/verify.py`'s call graph | `python3 scripts/verify.py`; `grep -q 'evals do not gate verify' docs/design-decisions.md` |

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives to keep them separate from active work without losing historical ideas.*

- **New skill: tech debt tracking** (2026-09-10) — dropped. Was only ever a loose idea (capture/triage/link debt to `BACKLOG.md`); user decided it may not be needed at all rather than refine the scope.
