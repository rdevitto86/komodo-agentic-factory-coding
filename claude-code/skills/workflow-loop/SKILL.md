---
name: workflow-loop
description: The end-to-end execution loop — spec, decompose, execute, consolidate, publish. Run it for any change bigger than a one-line fix, including when the user asks in plain language to build, ship, or implement something end to end rather than typing the slash command.
argument-hint: [task, or "open <topic>" for the unscripted path]
---

# Workflow Loop

**Five phases, in order.** Every phase names what ends it.

`$ARGUMENTS` names the task. **If it begins with `open`, skip the whole machine** — see The open hatch at the bottom.

**Load the matching way of working before P0**: `ways/sdlc.md` for code, `ways/debugging.md` when the task reads as diagnostic ("why is X broken", "debug", "investigate a failure") rather than build-something.

---

## Where each phase runs

**This session is the orchestrator** — rules, repo facts, the task list, and phase results, never the work that produced them.

| Phase | Runs as | Fork agent |
|---|---|---|
| P0 Spec | Here — dialogue cannot be forked; `/backlog-plan` when a backlog has to be built | — |
| P1 Decompose | Pick next task group here, then **`/workflow-decompose`**, then plan the run's PRs and branch here | `workflow-planner` |
| P2.0 Align | Here — the queue is the perpetual context | — |
| P2.1 Implement | **`/workflow-implement`, once per task** | `workflow-implementer` |
| P2.2 Verify | `verify_gate.py` — zero tokens | — |
| P2.3 Review | `/assess-bugs` (+ `/assess-security`), then commit here | `reviewer` |
| P2.4 Closeout | `/assess-bugs`, `/assess-security`, `/assess-simplify`, `/changelog-write`, `/backlog-audit`, once per band | `reviewer` (assess-*), `workflow-implementer` (backlog-audit) |
| P3 Consolidate | **`/workflow-consolidate`**, then commit its own delta (labels decided) here | `workflow-implementer` |
| P4 Publish | **`/workflow-complete`** — push + `/git-pr-create` + tag check | — |

**The forked phases carry their own instructions** (`context: fork`), so their rules and work never enter this window — only the returned result.

**One fork per task.** A fork returns a result; it never returns its reasoning.

**A phase ends only when its named skill actually ran** — state on disk that merely looks satisfied is not a substitute.

**A fork cannot see this conversation.** Everything it needs goes in `$ARGUMENTS` or is on disk — never "as discussed".

---

## P0 · Spec

**P0's one job is confirming `BACKLOG.md` exists before P1 decomposes it** — never decomposing (P1's job) or auditing (P1 runs `/backlog-audit`) itself. P0 either finds a backlog, builds one, or stops.

**`README.md` missing** → run `/readme-modify` to create it (cheap, never the blocker).

**`BACKLOG.md` exists** → `docs/spec/SDD.md`/`docs/spec/PRD.md` are optional context, never a blocker. Read if present, framing only (`standards-specs` owns their section maps) — full read on session's first run, `test -f` existence-only after. Go to P1.

**`BACKLOG.md` missing, `docs/spec/SDD.md` exists** → build the backlog now, from the SDD. Run `/backlog-plan <goal>`, goal drawn from the SDD (and PRD if present) plus `$ARGUMENTS` — its own Step 1 asks for refinement; never invent a second round of questions here. Write on the user's approval, then go to P1.

**`BACKLOG.md` missing and `docs/spec/SDD.md` missing** — nothing to build from. **This is the one condition allowed to exit the loop with nothing delivered.** Say so and stop.

**Ends when:** `BACKLOG.md` exists — either it already did, or `/backlog-plan` just wrote it with the user's approval.

---

## P1 · Decompose

**Requires `BACKLOG.md` to already exist.** If this phase starts without one on disk, exit the loop rather than templating or planning a backlog here.

**No audit runs here.** `/backlog-audit` runs at P2.4 instead, once per band.

**Pick the next task group before invoking anything.** `BACKLOG.md`'s file order is its priority order (`backlog-modify` sorts by `[P: SEV]` and `(after:)` edges; `backlog-prioritize` keeps it that way). Walk the target state top to bottom — `## Now, V1` by default, or whichever epic `$ARGUMENTS` names — and take the first task group holding at least one task that is neither `[DONE]` nor `[BLOCKED]`. `$ARGUMENTS` overrides this pick when it names a domain, task group, or story substring instead.

**No task group left with open, unblocked work** → say so and stop.

**Run `/workflow-decompose [target state] [scope]`**, `scope` being the task group just picked, or `$ARGUMENTS`'s override. It reads the repo facts, the backlog, and the changelog in a fork, and returns a queue naming the exact `TSK-` IDs in scope for this run.

**Read its `## Gaps` before doing anything else.** A missing `Done when`, a missing test story, or a chained decomposition is a spec problem — go to P0, not P2.

**A `BACKLOG.md` holding only `git-repo-init`'s seed stories is not a decomposed queue** — run the fork rather than treating it as if P1 already happened.

**A language manifest already on disk (`go.mod`, `package.json`, `cdk.json`) means `git-repo-init`'s Create already ran — trust the tree.** Re-invoke `/git-repo-init` only for an open Foundation-edge story (`backlog-modify` owns that edge), or when the task explicitly asks for Scaffold/Refresh.

**Plan the run's PRs before branching — no branch, no commit, no `gh pr create` here.** A task group is one domain, so the queue is usually already one PR-sized band per `git-pr-create`'s sizing rule of thumb. Split further only if the group partitions cleanly within itself (no shared file, no dependency edge crossing the split); an entangled group stays one band regardless of size. Present the grouping as a short table (PR # · `TSK-` IDs · why). **Only this run's first planned PR gets built out through P2–P4** — a group needing a second band stays queued, ordered by dependency.

**Once the plan's first PR is confirmed, branch here** — `git switch -c <type>/<short-kebab-description>` per `git-pr-create`'s naming rule. **Resuming a `[WIP]` story reuses its existing branch** (`git switch <existing-branch>`) — check the story text before assuming none exists.

**Ends when:** every task has `Done when` commands and a `Depends on` edge, and the branch is checked out.

---

## P2 · Execute

### P2.0 · Align

**Confirm and order P1's list; do not regenerate it** — P1 already paid for those reads.

**After a P2.3/P2.4 audit call files new `BACKLOG.md` stories, fold them into this list before re-entering P2.1.**

**Pick tasks that share no dependency edge and no file.** Two tasks touching one file are one task.

**A task `[BLOCKED]` on something outside this run is not a phase halt** — pick the next task with no dependency edge to it and continue; the blocker surfaces in P4's report. Only stop if every remaining task is transitively blocked.

**Ends when:** one task is named `[WIP]` and the rest are written down — or, when dispatching a P1-confirmed parallel set (see P2.1), every task in that set is named `[WIP]` together.

### P2.1 · Implement

**Run `/workflow-implement <task text and Done when commands>`, once per task.**

**Pass every command explicitly.** The fork cannot see the queue — a task with no `Done when` commands stops rather than guessing one.

**Run the fork even when the code already appears to exist on disk** — verifying inherited state is not implementing it.

**When P1's `## Parallel` output already names a set of tasks with no shared file and no dependency edge among them (confirmed at P2.0), dispatch that set together in one block, `isolation: worktree`, rather than one task at a time.** "Once per task" bounds a fork's scope, never the order tasks start in — serialize only across a real dependency edge. **Each task in the set still goes through P2.2/P2.3 individually** — verify, review, and commit each on its own, merging its worktree branch back before moving to the next task's commit; dispatching in parallel changes only when implementation starts, not how each result is checked in.

**A fork returns a result, never its reasoning — except `## Comment Candidates`.** Retain each task's non-empty `## Comment Candidates` entries verbatim across the band — P3's `/write-comments` call needs the live WHY-context captured at implementation time.

**Ends when:** every one of the task's `Done when` commands exits zero.

### P2.2 · Verify

**Run the command; paste what it returned** — "tests pass" without output is not a result.

**Fix the cause, never suppress the check** — a skipped test, a widened type, or a silenced lint is a failed phase reported as a passed one.

**Failure returns to P2.1**, with the failure output in the brief.

**Ends when:** the command exits zero and its output is in the transcript.

### P2.3 · Review

**`/assess-bugs`, plus `/assess-security` when the surface warrants it** (`ways/` names the trigger). Each runs as a `reviewer` fork.

**Review against the task, not the diff** — did every acceptance condition land, and did anything outside the task change?

**Each call files its findings straight to `BACKLOG.md`, no `--report`.** Findings affecting correctness, security, or a stated requirement are folded into P2.0's pick and become the next P2.1 task, not deferred. Everything else is optional, filed for a later pass.

**Commit here once the task's findings are clean.** Run `/git-commit-message` against this task's diff alone (not the whole band), then `git add` + `git commit` — one commit per task, including any P2.3 fix-commits.

**Ends when:** every story the calls filed for this task is fixed or confirmed genuinely optional, and the task is committed. Green ends the task; loop back to P2.0 for the next one.

### P2.4 · Closeout

**`/assess-bugs`, `/assess-security`, `/assess-simplify` against the whole band, once, plus the perf suite.** Runs after every task is green, before P3 — never per-task, each as a `reviewer` fork.

**Each call files its findings straight to `BACKLOG.md`, no `--report`.** Every story filed is folded into P2.0's pick and resolved in this pass — fixed via `/workflow-implement`, or declined and removed with the reason noted for P4's report.

**For a single-task band, skip the `/assess-bugs` repeat when that task was already cleared at P2.3 with no diff change since.** Skip the `/assess-security` repeat under the same condition only if P2.3 actually ran it — if P2.3 skipped `/assess-security` because the surface didn't warrant it, that same judgment applies here too, so it stays skipped for the same reason, not because it's being treated as already cleared. Either way, still run `/assess-simplify` (never covered at P2.3) plus the perf suite.

**Clears the target state's four standing closeout stories.** Never picked as ordinary P2.1 tasks — `assess-*` calls stay this session's job, not the fork that wrote the code.

**Once every closeout finding is fixed or declined, run `/changelog-write` for the whole band** — one pass covering every task shipped. P3 then only releases what this step already wrote.

**Once the changelog is written, run `/backlog-audit` over the whole file, no scope** — sweeps stale tasks, duplicates, and satisfied `[BLOCKED]` `Recheck:` entries.

**Ends when:** every closeout finding is fixed or declined, `/backlog-audit` has applied its verdicts, and `CHANGELOG.md`'s `[Unreleased]` section reflects the band.

---

## P3 · Consolidate

**Run `/workflow-consolidate <the task summaries that went green>`** once the whole band is done and P2.4 has cleared, not after each task.

It releases P2.4's `[Unreleased]` entries at the bump they earn, syncs the manifest, clears the finished stories, and refreshes only the README parts the change invalidated. **It never touches the SDD** — frozen; a change it needs comes back to you as a finding.

**Right before this commit, decide the PR's labels** — category + authorship, per `git-pr-create`'s "Label it" order, read off `git diff <base>...HEAD --stat` for the band so far. Carry that decision into P4.

**Run `/git-commit-message` against consolidate's delta to draft the message, then run `/write-comments` before committing anything — never after** (new commits only, never amends). Feed it the band's diff, the `BACKLOG.md` task stories, the drafted message, every task's accumulated Comment Candidates from P2.1, and `.claude/state/removed-comments.jsonl`; truncate that file once it returns.

**Commit here once both have run, one commit** — consolidate's version/manifest/backlog-cleanup output plus whatever `write-comments` spliced plus the now-truncated removed-comments log, under the drafted message. `git add` + `git commit`.

**Ends when:** the changelog section is released, the backlog no longer lists finished work, PR labels are decided, `/write-comments` has run with `.claude/state/removed-comments.jsonl` truncated after it, and consolidate's delta with any spliced comments is committed.

---

## P4 · Publish

**`/workflow-complete`.** Pushes the branch, runs `/git-pr-create` (handing it P3's labels) to open or update the pull request, then checks the tag once the push has landed.

**Ends when:** the branch is pushed and `/git-pr-create` has returned the PR URL.

---

## Guardrails

- **Stopping is judgement, not a counter** — the same check failing twice with the same error ends the attempt. Mark the story `[BLOCKED]`, indent the reason beneath it, four sentences max, with a `file:line` — full shape in `backlog-modify`.
- **Backing out is a rewrite** — capture `git diff` before a risky write; `git-pr-create` owns the recovery command.
- **The bridge is optional, never blocking** — an unreachable MCP server is a skipped step; never branch a phase on whether it is up.
- **Never poll a delegated phase** — it re-invokes this session the moment it finishes.
- **A fork's result is the record — don't re-open a file it just wrote.** Work from the returned `## Filed`/`## Changed` block; only open the file directly for a task no fork result handed you (e.g. reading `BACKLOG.md` fresh at the start of P1 decompose).
- **Standards verification happens inside the review or implement fork, never in this window** — a P2.3/P2.4 finding needing re-verifying is P2.1's job.
- **After any context compaction, re-read the active `ways/` file before the next phase gate** — it loads via `Read`, not invocation, so compaction skips it.

---

## Delegating outside the phases

| Need | Send to |
|---|---|
| Read-only research across many files | `engineering` |
| "Where is X" — a path list | `scout` |
| Grading work this session produced | Fresh subagent — **never `subagent_type: fork`.** A `context: fork` *skill* (`/assess-bugs`, `/assess-security`, `/assess-simplify`) runs `reviewer`, not this session — how P2.3/P2.4 grade the diff. |

**Parallel writers need `isolation: worktree`** — two agents editing one checkout collide; read-only fan-out needs none.

**Set `model`/`effort` in the delegate's own frontmatter.** `opus`/`high` for architecture and hard debugging, `sonnet`/`medium` for research and routine code, `haiku`/`low` for path lookup.

**Brief with `Task` / `Files` / `Context` / `Done when` / `Out of scope`** — never "see above". Ask for the verdict, not the transcript.

---

## The open hatch

**`/workflow-loop open <topic>` skips every phase above.** For design, architecture, and exploration, where a script produces worse output than judgement. Use it when the task is to *decide* something, never to build something already decided.
