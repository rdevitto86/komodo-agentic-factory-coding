---
name: workflow-loop
description: The end-to-end execution loop — spec, decompose, execute, consolidate, publish. Run it for any change bigger than a one-line fix, including when the user asks in plain language to build, ship, or implement something end to end rather than typing the slash command.
argument-hint: [task, or "open <topic>" for the unscripted path]
---

# Workflow Loop

**Five phases, in order.** Every phase names what ends it, so there is never a judgement call about whether to move on.

`$ARGUMENTS` names the task. **If it begins with `open`, skip the whole machine** — see The open hatch at the bottom.

**Load the matching way of working before P0**: `ways/sdlc.md` for code, `ways/debugging.md` when the task reads as diagnostic ("why is X broken", "debug", "investigate a failure") rather than build-something. It defines what the gates mean in that domain; this file defines the machine.

---

## Where each phase runs

**This session is the orchestrator.** It holds rules, repo facts, the task list, and phase results — never the work that produced them.

| Phase | Runs as | Fork agent |
|---|---|---|
| P0 Spec | Here — dialogue cannot be forked | — |
| P1 Decompose | **`/audit-backlog` then `/workflow-decompose`**, then branch here | `workflow-planner` |
| P2.0 Align | Here — the queue is the perpetual context | — |
| P2.1 Implement | **`/workflow-implement`, once per task** | `workflow-implementer` |
| P2.2 Verify | `verify_gate.py` — zero tokens | — |
| P2.3 Review | `/audit-bugs` (+ `/audit-security`), then commit here | — |
| P2.4 Closeout | `/audit-bugs`, `/audit-security`, `/audit-simplify`, once per band | — |
| P3 Consolidate | **`/workflow-consolidate`**, then commit its own delta here | `workflow-implementer` |
| P4 Publish | **`/workflow-complete`** — push + `/git-pr` | — |

**The forked phases carry their own instructions.** Each declares `context: fork` in its frontmatter, so neither the phase's rules nor the work it does ever enters this window — only its returned result. That is why this file is short: the detail lives where it is paid for.

**One fork per task is what stops context drift.** A fork returns a result; it never returns its reasoning.

**A phase ends only when its named skill actually ran.** State on disk that merely looks satisfied — files that exist, a build that passes — is not a substitute for running the phase; inheriting it and reporting the phase complete is the exact drift this machine exists to prevent.

**A fork cannot see this conversation.** Everything it needs goes in `$ARGUMENTS` or is on disk. Never write "as discussed".

---

## P0 · Spec — the human gate

**Requires the SDD (a repo file under `docs/spec/SDD.md`) and `README.md`.** Load `standards-specs` for the SDD's section map, and `write-readme` for README's shape. This phase never drafts the SDD — authoring happens with stakeholders directly in the repo, not by this toolkit.

**On any run after the first, the check is existence only.** One `test -f` for `README.md`, one for `docs/spec/SDD.md` — then go to P1. Never re-read the full SDD body every loop; that's the token burn this design removes.

**On the first run, read `README.md` before reading the SDD, not after.** It is the fastest source of the high-level framing (what the repo is, who it's for) that would otherwise have to be reconstructed from the task description alone — pull from it, never duplicate its wording verbatim.

**If `README.md` is missing, `docs/spec/SDD.md` is missing, or it still carries `NEEDS DECISION` in a section this work depends on**, draft what you can and stop. **This is the one phase allowed to block with nothing delivered** — building on an unapproved spec is the guessing the whole machine exists to prevent.

**Ends when:** `README.md` exists, `docs/spec/SDD.md` exists, and it has the user's approval.

---

## P1 · Decompose

**Run `/audit-backlog [scope]` first, on the same scope `$ARGUMENTS` names.** It's the cheap pass — stale, resolved, duplicate, or now-cleared `[BLOCKED]` lines surface here without the fork's full repo+changelog re-derivation. Carry its findings into `/workflow-decompose`'s brief; an unflagged backlog still runs the fork, but arrives with nothing left to recheck.

**Run `/workflow-decompose [target state] [scope]`, forwarding `$ARGUMENTS` as the scope if it names one** — a domain or a story substring. Default with no scope: every story in the current target state that isn't `[BLOCKED]` after the fork's own recheck pass. It reads the repo facts, the backlog, and the changelog in a fork, and returns a queue.

**If it reports `BACKLOG.md` missing**, create it here from `templates/project/BACKLOG.md.tmpl` and run it again. The fork is read-only by design.

**Read its `## Gaps` before doing anything else.** A missing `Done when`, a missing test story, or a chained decomposition is a spec problem — fixing it means going back to P0 with the user, not improvising in P2.

**A `BACKLOG.md` holding only `write-repo`'s seed stories is not a decomposed queue.** Those seed stories are scaffolding, not work derived from the SDD — run the fork rather than treating an unread backlog as if P1 already happened.

**A language manifest already on disk (`go.mod`, `package.json`, `cdk.json`) means `write-repo`'s Create already ran for this repo — trust the tree.** Re-invoke `/write-repo` only when a Foundation-edge story is still open in `BACKLOG.md` (`write-backlog` owns that edge), or when Scaffold/Refresh is what the task explicitly asks for. Checking the manifest's presence is the zero-token signal; re-running generation to confirm it worked is not.

**Once the queue is confirmed, branch here** — `git switch -c <type>/<short-kebab-description>` per `rules-source-control`'s naming rule, `type` and description drawn from the band's dominant concern. **Resuming a `[WIP]` story reuses its existing branch** (`git switch <existing-branch>`) instead of creating a second one — check the story text for a branch name before assuming none exists.

**Ends when:** every task has a `Done when` command and a `Depends on` edge, and the band's branch is checked out.

---

## P2 · Execute

### P2.0 · Align

**Confirm and order P1's list.** Do not regenerate it — P1 already paid for those reads.

**After a P2.3 or P2.4 audit call files new `BACKLOG.md` stories, fold them into this list before re-entering P2.1** — same domain, same pass. That is what keeps an audit finding from becoming stale backlog debt: it is picked up before this run ends, not left for the next `/workflow-loop` invocation to discover.

**Pick tasks that share no dependency edge and no file.** Two tasks touching one file are one task.

**A task `[BLOCKED]` on something outside this run is not a phase halt.** Pick the next task with no dependency edge to the blocked one and continue — the blocker still surfaces, in P4's report, not as a stopped loop. Only stop here if every remaining task is transitively blocked.

**Ends when:** one task is named `[WIP]` and the rest are written down.

### P2.1 · Implement

**Run `/workflow-implement <task text and Done when command>`**, once per task.

**Pass the command explicitly.** The fork cannot see the queue — a task arriving without its `Done when` stops rather than guessing one.

**Run the fork even when the code already appears to exist on disk.** Verifying inherited state is not implementing it — the fork is what actually re-derives whether that state is correct, tested, and matches the task, rather than this session taking a build's exit code on faith.

**Ends when:** the task's `Done when` command exits zero.

### P2.2 · Verify

**Run the command. Paste what it returned.** "Tests pass" without output is not a result.

**Fix the cause, never suppress the check.** A skipped test, a widened type, or a silenced lint is a failed phase reported as a passed one.

**Failure returns to P2.1**, with the failure output in the brief.

**Ends when:** the command exits zero and its output is in the transcript.

### P2.3 · Review

**`/audit-bugs`, plus `/audit-security` when the touched surface warrants it** (the `ways/` file names the trigger). Never a fork of this session — a fork saw the reasoning that produced the code and will agree with it.

**Review against the task, not the diff.** Did every acceptance condition land, and did anything outside the task change?

**Each call files its findings straight to `BACKLOG.md`, no `--report`.** Findings that affect correctness, security, or a stated requirement are folded into P2.0's pick and become the next P2.1 task in this same pass — not deferred. Everything else is optional and stays filed for a later pass — a reviewer asked to find gaps will always find some, and chasing all of them produces defensive code and tests for cases that cannot happen.

**Commit here once the task's findings are clean.** Run `/git-commit-message` against this task's diff alone (not the whole band), then `git add` + `git commit` with its result — one commit per task, including any P2.3 fix-commits the task itself spawned. This is what keeps a dropped session from losing more than the one task in flight; the previous tasks are already durable.

**Ends when:** every story the calls filed for this task is fixed or confirmed genuinely optional, and the task is committed. Green ends the task; loop back to P2.0 for the next one.

### P2.4 · Closeout

**`/audit-bugs`, `/audit-security`, `/audit-simplify` against the whole band, once, plus the perf suite.** Runs after every task in the pick is green, before P3 — never per-task, never a fork, same reasoning as P2.3.

**Each call files its findings straight to `BACKLOG.md`, no `--report`.** Every story a call just filed for this band is folded into P2.0's pick and resolved in this same pass — fixed via `/workflow-implement`, or explicitly declined and removed from `BACKLOG.md` with the reason noted for P4's report. A closeout finding left open past this phase is exactly the pile-up this step exists to prevent.

**Clears the target state's four standing closeout stories.** This is what stops them sitting open forever: they are never picked as ordinary P2.1 tasks — a `workflow-implementer` fork wrote the code and can't also review it cold, `audit-*` calls stay this session's job — they are cleared here instead.

**Ends when:** every story the closeout calls filed is fixed or explicitly declined — that satisfies the four stories' `Done when`, so P3 deletes them like any other finished story.

---

## P3 · Consolidate

**Run `/workflow-consolidate <the task summaries that went green>`** once the whole band is done and P2.4 has cleared, not after each task.

It writes the changelog entry, bumps the version, syncs the manifest, clears the finished stories, and refreshes only the README parts the change invalidated. **It never touches the SDD** — that's frozen, and a change it needs comes back to you as a finding.

**Commit here once it returns — its own delta only.** `workflow-consolidate` runs as `workflow-implementer`, which never commits by design (a fork's diff has to clear this session's judgment before it lands, not before). Every task's own diff already has its own commit from P2.3; this commit is just consolidate's changelog/version/backlog-cleanup output. Run `/git-commit-message` against that delta, then `git add` + `git commit`.

**Ends when:** the changelog entry exists, the backlog no longer lists finished work, and consolidate's delta is committed.

---

## P4 · Publish

**`/workflow-complete`.** Pushes the branch and runs `/git-pr` to open or update the pull request for this band.

**Ends when:** the branch is pushed and `/git-pr` has returned the PR URL.

---

## Guardrails

- **Stopping is judgement, not a counter.** The same check failing twice with the same error ends the attempt. Mark the story `[BLOCKED]`, indent the reason beneath it, four sentences maximum, with a `file:line` — full shape in `write-backlog`.
- **Backing out is a rewrite.** Capture `git diff` before a risky write; `rules-source-control` owns handing the user the recovery command.
- **The bridge is optional, never blocking.** An unreachable MCP server is a skipped step. Never branch a phase on whether it is up.
- **Never poll a delegated phase.** A fork and a backgrounded review both re-invoke this session the moment they finish. A scheduled check burns a full turn even when it lands on time, and can fire *stale* — after the work already completed — re-running dead instructions against state that already moved on.
- **A P2.3 or P2.4 finding that needs standards verification goes back to a fork, never re-loaded into this window.** Loading `standards-*` skills here to re-check a finding the reviewer already grounded is the exact context drift forking exists to prevent — if it genuinely needs re-verifying, that is P2.1's job, not this session's.

---

## Delegating outside the phases

| Need | Send to |
|---|---|
| Read-only research across many files | `engineering` |
| "Where is X" — a path list | `scout` |
| Grading work this session produced | Fresh subagent — **never a fork** |

**Parallel writers need `isolation: worktree`.** Two agents editing one checkout collide; read-only fan-out needs none.

**Set `model` and `effort` in the delegate's own frontmatter** — they beat the session setting. `opus`/`high` for architecture and hard debugging, `sonnet`/`medium` for research and routine code, `haiku`/`low` for path lookup. A lower effort does not remove reasoning; it lets the model skip thinking where none is needed.

**Brief with `Task` / `Files` / `Context` / `Done when` / `Out of scope`.** Never write "see above" — the delegate cannot see above. Ask for the verdict, not the transcript; every report lands in this window.

---

## The open hatch

**`/workflow-loop open <topic>` skips every phase above.** For design, architecture, and exploration, where a script produces worse output than judgement.

It is named explicitly so that scripted stays the default. Use it when the task is to *decide* something, never to build something already decided.
