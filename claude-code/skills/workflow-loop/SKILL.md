---
name: workflow-loop
description: The end-to-end execution loop — spec, decompose, execute, consolidate, publish. Run it for any change bigger than a one-line fix, including when the user asks in plain language to build, ship, or implement something end to end rather than typing the slash command.
argument-hint: [task, "fast <task>" for the checked short path, or "open <topic>" for the unscripted one]
---

# Workflow Loop

**Five phases, in order.** Every phase names what ends it.

`$ARGUMENTS` names the task. **If it begins with `open`, skip the whole machine; if `fast`, skip the planning phases** — both blocks at the bottom.

**Load the matching way of working before P0**: `ways/sdlc.md` for code, `ways/debugging.md` when the task reads as diagnostic ("why is X broken", "debug", "investigate a failure") rather than build-something.

---

## Where each phase runs

**This session is the orchestrator** — rules, repo facts, the task list, and phase results, never the work that produced them.

| Phase | Runs as | Fork agent |
|---|---|---|
| P0 Spec | Here — dialogue cannot be forked; `/backlog-plan` when a backlog has to be built | — |
| P1 Decompose | Pick next task group here, then **`/workflow-decompose`**, then plan the run's PRs and branch here | `pm` |
| P2.0 Align | Here — the queue is the perpetual context | — |
| P2.1 Implement | **`/workflow-implement`, once per task** | `builder` |
| P2.2 Verify + commit | `verify_gate.py`, then commit here | — |
| P2.3 Band review | `/assess-bugs`, `/assess-simplify` (+ `/assess-security`, + `/assess-performance`), once per band | `reviewer` (`/assess-performance` runs inline, not forked) |
| P2.4 Closeout | `/changelog-write`, once per band | — |
| P3 Consolidate | **`/workflow-consolidate`**, then commit its own delta (labels decided) here | `builder` |
| P4 Publish | **`/workflow-complete`** — push + `/git-pr-create` + tag check | — |

**The forked phases carry their own instructions** (`context: fork`), so their rules and work never enter this window — only the returned result.

**One fork per task.** A fork returns a result; it never returns its reasoning.

**A phase ends only when its named skill actually ran** — state on disk that merely looks satisfied is not a substitute.

**A fork cannot see this conversation.** Everything it needs goes in `$ARGUMENTS` or is on disk — never "as discussed".

---

## P0 · Spec

**P0's one job is confirming `BACKLOG.md` exists before P1 decomposes it** — never decomposing (P1's job) or auditing itself. P0 either finds a backlog, builds one, or stops.

**`README.md` missing** → run `/readme-modify` to create it (cheap, never the blocker).

**`BACKLOG.md` exists** → `docs/spec/SDD.md`/`docs/spec/PRD.md` are optional context, never a blocker. Read if present, framing only (`standards-specs` owns their section maps) — full read on session's first run, `test -f` existence-only after. Go to P1.

**`BACKLOG.md` missing, `docs/spec/SDD.md` exists** → build the backlog now, from the SDD. Run `/backlog-plan <goal>`, goal drawn from the SDD (and PRD if present) plus `$ARGUMENTS` — its own Step 1 asks for refinement; never invent a second round of questions here. Write on the user's approval, then go to P1.

**`BACKLOG.md` missing and `docs/spec/SDD.md` missing** — nothing to build from. **This is the one condition allowed to exit the loop with nothing delivered.** Say so and stop.

**Ends when:** `BACKLOG.md` exists — either it already did, or `/backlog-plan` just wrote it with the user's approval.

---

## P1 · Decompose

**Requires `BACKLOG.md` to already exist.** If this phase starts without one on disk, exit the loop rather than templating or planning a backlog here.

**No audit runs here.** No phase runs a band-scoped audit right now — TSK-01.4.4 moves that pass into P3.

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

**After P2.3's band review appends findings, fold the severity-floor set into this list before re-entering P2.1.**

**Pick tasks that share no dependency edge and no file.** Two tasks touching one file are one task.

**A task `[BLOCKED]` on something outside this run is not a phase halt** — pick the next task with no dependency edge to it and continue; the blocker surfaces in P4's report. Only stop if every remaining task is transitively blocked.

**Ends when:** one task is named `[WIP]` and the rest are written down.

### P2.1 · Implement

**Run `/workflow-implement <task text and Done when commands>`, once per task.**

**Pass every command explicitly.** The fork cannot see the queue — a task with no `Done when` commands stops rather than guessing one.

**Run the fork even when the code already appears to exist on disk** — verifying inherited state is not implementing it.

**A fork returns a result, never its reasoning.**

**No retry counter here** — a P2.2 failure sending a task back to this phase is already bounded by P2.2's own pass/fail, not a repeat-prone loop. The retry tally lives at P2.3, the phase where a "same skill, same file, no forward progress" loop actually happens.

**When a task involves a genuine mechanism or design choice — not merely "write this function" — the brief states the chosen mechanism and why, rather than leaving it to the fork's judgment.** A fork picking wrong on an open design question costs a discovery-at-review round trip the brief could have closed for free. If the choice is genuinely undecided, that's a P0/P2.0 decision to make before forking, not something to hand off ambiguously.

**Ends when:** every one of the task's `Done when` commands exits zero.

### P2.2 · Verify + commit

**Run the task's `Done when` commands; paste what each returned** — "tests pass" without output is not a result.

**Fix the cause, never suppress the check** — a skipped test, a widened type, or a silenced lint is a failed phase reported as a passed one.

**Failure returns to P2.1**, with the failure output in the brief.

**A band exception (see `ways/`) also runs a per-task `/assess-bugs` here, before the commit** — that call's findings block the commit like any other P2.2 failure.

**Once every command is green (and the exception call, when it ran, is clean), commit** — `/git-commit-message` against this task's diff alone, then `git add` + `git commit`. One commit per task. No review runs here otherwise.

**Ends when:** every command exits zero, its output is in the transcript, and the task is committed. Loop back to P2.0 for the next task.

### P2.3 · Band review

**Runs once per band, after every task in the band is committed — never per task.** Dispatch `/assess-bugs`, `/assess-simplify`, and `/assess-security` (when the surface warrants it, per `ways/`) together in one parallel block, as plain forks — no isolation, they are read-only after TSK-01.4.2. Each runs as a `reviewer` fork. **Also run `/assess-performance`** (when a touched path is performance-sensitive, per `ways/`) in the same pass — it carries no `context: fork` frontmatter, so it runs inline in this session rather than as a `reviewer` fork, and self-files its own `BACKLOG.md` story at Med-High or above per its own contract, same as `/assess-code-quality` already invokes it elsewhere.

**Review against the band, not any one task** — did every task's acceptance condition land, and did anything outside the tasks change?

**Each of the three finder forks returns its findings; the orchestrator appends every row to `BACKLOG.md` under the matching domain in one Edit, before triage.** `/assess-performance` files its own story directly — nothing to append for that one.

**Severity floor.** Fix the findings `ways/sdlc.md`'s severity floor selects, via `/workflow-implement`; each fixed task re-enters P2.2. Everything else stays filed and open for a later pick.

**Retry counter — session-stated, not a file.** Before re-invoking the same `assess-*` skill against the same file a repeat time within this band, state "round N for `<file>`" in this session's own turn text — P2.0's queue is the tally, no `.claude/state/` file. Hitting `ways/sdlc.md`'s round cap is the stop signal: stop auto-continuing that skill against that file and escalate to the user.

**Ends when:** the severity-floor set is fixed or the round cap has surfaced a stop to the user, and everything else is filed. Continue to P2.4.

### P2.4 · Closeout

**Once per band, not per-task** — after P2.3's band review has resolved its severity floor, before P3.

**Only `/changelog-write` runs here**, covering every task the band shipped. No `assess-*` repeat — P2.3 already covered the whole band — and no band-scoped audit — no phase runs one right now (TSK-01.4.4 moves that pass into P3).

**The four standing closeout tasks live in their own `Quality Assurance & Epic Hardening` task group, gated by their own `Trigger` bullet — an ordinary band's changelog write never satisfies them.** See `ways/sdlc.md` for the gate's exact condition.

**Ends when:** `CHANGELOG.md`'s `[Unreleased]` section reflects the band.

---

## P3 · Consolidate

**Run `/workflow-consolidate <the band's `TSK-` IDs and task summaries>`** once the whole band is done and P2.4 has cleared, not after each task.

It releases P2.4's `[Unreleased]` entries at the bump they earn, syncs the manifest, clears the finished stories, and refreshes only the README parts the change invalidated. **It never touches the SDD** — frozen; a change it needs comes back to you as a finding.

**Right before this commit, decide the PR's labels** — category + authorship, per `git-pr-create`'s "Label it" order, read off `git diff <base>...HEAD --stat` for the band so far. Carry that decision into P4.

**Run `/git-commit-message` against consolidate's delta to draft the message, then commit** — `git add` + `git commit`, one commit.

**Ends when:** the changelog section is released, the backlog no longer lists finished work, PR labels are decided, and consolidate's delta is committed.

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
- **Timing is captured silently, session-stated, not a file** — the same pattern as P2.3's retry counter. Note each phase's (P0–P4) and each forked skill's start and end wall-clock in this session's own turn text as it happens. Never print elapsed time, and never fold it into an "Ends when" report — surface it only if the user asks.
- **A fork's result is the record — don't re-open a file it just wrote.** Work from the returned `## Findings`/`## Changed` block — a reviewer fork returns `## Findings` only, this session files the rows itself; only open the file directly for a task no fork result handed you (e.g. reading `BACKLOG.md` fresh at the start of P1 decompose).
- **Standards verification happens inside the review or implement fork, never in this window** — a P2.3 finding needing re-verifying is P2.1's job.
- **After any context compaction, re-read the active `ways/` file before the next phase gate** — it loads via `Read`, not invocation, so compaction skips it.

---

## Delegating outside the phases

| Need | Send to |
|---|---|
| Read-only research across many files | `researcher` |
| "Where is X" — a path list | `scout` |
| A design question with more than one answer | `architect` — weighed options, never a decision |
| Tests against an existing interface | `tester` — test paths only, and that is prose, not a lock |
| Grading work this session produced | Fresh subagent — **never `subagent_type: fork`.** A `context: fork` *skill* (`/assess-bugs`, `/assess-security`, `/assess-simplify`) runs `reviewer`, not this session — how P2.3 grades the diff. |

**Parallel writers need `isolation: worktree`** — two agents editing one checkout collide, and `builder` writes tests too, so `tester` beside it is two writers. Read-only fan-out needs none.

**Set `model`/`effort` in the delegate's own frontmatter.** `opus`/`high` for architecture and hard debugging, `sonnet`/`medium` for research and routine code, `haiku`/`low` for path lookup. **Brief with `Task` / `Files` / `Context` / `Done when` / `Out of scope`** — never "see above". Ask for the verdict, not the transcript.

---

## The open hatch

**`/workflow-loop open <topic>` skips every phase above.** For design, architecture, and exploration, where a script produces worse output than judgement. Use it when the task is to *decide* something, never to build something already decided.

---

## The fast path

**`/workflow-loop fast <task>` skips P1 decompose, P2.0 align, and P3 consolidate.** P2.1, P2.2, P2.3, and P2.4 run unchanged — fast means less planning, never less verification or review.

**Entry is checked, not judged.** Run both against the change's own diff — before P2.1 on the paths the task names, and again before P2.2's commit:

```bash
git diff --shortstat     # ≤ 1 file changed and ≤ 10 lines changed
git diff --name-only     # no path on the exclusion list below
```

**Exclusion list — any match refuses the fast path regardless of diff size:** the agent-config surface (`**/hooks/**`, `**/agents/**`, `settings.json`), the repo's verify gate (`.claude/verify.sh`, `Makefile`, `Taskfile.yml`, `justfile`), and anything the repo's `AGENTS.md` names as a security boundary. A change able to weaken its own verification is not a fast path.

**Either check failing sends the change back to P1** — the full machine, from the top.
