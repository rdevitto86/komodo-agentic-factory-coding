---
name: lifecycle
description: The end-to-end execution loop — spec, decompose, execute, consolidate, complete. Run it for any change bigger than a one-line fix.
argument-hint: [task, or "open <topic>" for the unscripted path]
disable-model-invocation: true
---

# Lifecycle

**Five phases, in order.** Every phase names what ends it, so there is never a judgement call about whether to move on.

`$ARGUMENTS` names the task. **If it begins with `open`, skip the whole machine** — see The open hatch at the bottom.

**Load the matching way of working before P0**: `ways/sdlc.md` for code. It defines what the gates mean in that domain; this file defines the machine.

---

## Where each phase runs

**This session is the orchestrator.** It holds rules, repo facts, the task list, and phase results — never the work that produced them.

| Phase | Runs as | Fork agent |
|---|---|---|
| P0 Spec | Here — dialogue cannot be forked | — |
| P1 Decompose | **`/decompose`** | `planner` |
| P2.0 Align | Here — the queue is the perpetual context | — |
| P2.1 Implement | **`/implement`, once per task** | `implementer` |
| P2.2 Verify | `verify_gate.py` — zero tokens | — |
| P2.3 Review | `/code-review` — isolated by construction | — |
| P3 Consolidate | **`/consolidate`** | `implementer` |
| P4 Complete | Here — short output | — |

**The forked phases carry their own instructions.** Each declares `context: fork` in its frontmatter, so neither the phase's rules nor the work it does ever enters this window — only its returned result. That is why this file is short: the detail lives where it is paid for.

**One fork per task is what stops context drift.** A fork returns a result; it never returns its reasoning.

**A fork cannot see this conversation.** Everything it needs goes in `$ARGUMENTS` or is on disk. Never write "as discussed".

---

## P0 · Spec — the human gate

**Requires `docs/prd.md` and `docs/sdd.md`.** Load `docs` for their shape.

**On any run after the first, the check is existence only.** One `test -f` each, then go to P1. Never re-read or re-validate them — they are frozen, and re-reading them every loop is exactly the token burn this design removes.

**If either is missing or still carries `NEEDS DECISION` in a section this work depends on**, draft what you can and stop. **This is the one phase allowed to block with nothing delivered** — building on an unapproved spec is the guessing the whole machine exists to prevent.

**Ends when:** both files exist and the user has approved them.

---

## P1 · Decompose

**Run `/decompose [target state]`.** It reads the repo facts, the backlog, and the changelog in a fork, and returns a queue.

**If it reports `BACKLOG.md` missing**, create it here from `templates/project/BACKLOG.md.tmpl` and run it again. The fork is read-only by design.

**Read its `## Gaps` before doing anything else.** A missing `Done when`, a missing test story, or a chained decomposition is a spec problem — fixing it means going back to P0 with the user, not improvising in P2.

**Ends when:** every task has a `Done when` command and a `Depends on` edge.

---

## P2 · Execute

### P2.0 · Align

**Confirm and order P1's list.** Do not regenerate it — P1 already paid for those reads.

**Pick tasks that share no dependency edge and no file.** Two tasks touching one file are one task.

**Ends when:** one task is named `[WIP]` and the rest are written down.

### P2.1 · Implement

**Run `/implement <task text and Done when command>`**, once per task.

**Pass the command explicitly.** The fork cannot see the queue — a task arriving without its `Done when` stops rather than guessing one.

**Ends when:** the task's `Done when` command exits zero.

### P2.2 · Verify

**Run the command. Paste what it returned.** "Tests pass" without output is not a result.

**Fix the cause, never suppress the check.** A skipped test, a widened type, or a silenced lint is a failed phase reported as a passed one.

**Failure returns to P2.1**, with the failure output in the brief.

**Ends when:** the command exits zero and its output is in the transcript.

### P2.3 · Review

**`/code-review`.** Never a fork of this session — a fork saw the reasoning that produced the code and will agree with it.

**Review against the task, not the diff.** Did every acceptance condition land, and did anything outside the task change?

**Findings that affect correctness, security, or a stated requirement return to P2.1.** Everything else is optional — a reviewer asked to find gaps will always find some, and chasing all of them produces defensive code and tests for cases that cannot happen.

**Ends when:** findings are fixed or explicitly declined. Green ends the task; loop back to P2.0 for the next one.

---

## P3 · Consolidate

**Run `/consolidate <the task summaries that went green>`** once the whole band is done, not after each task.

It writes the changelog entry, bumps the version, syncs the manifest, clears the finished stories, and refreshes only the README parts the change invalidated. **It never touches the PRD or SDD** — those are frozen, and a change either needs comes back to you as a finding.

**Ends when:** the changelog entry exists and the backlog no longer lists finished work.

---

## P4 · Complete

**`/complete`.** Outputs a copy-pastable conventional commit for the user to paste.

**The user commits.** Never stage, commit, push, or offer to.

---

## Guardrails

- **Stopping is judgement, not a counter.** The same check failing twice with the same error ends the attempt. Mark the story `[BLOCKED]` per `worklog` — four sentences, with a `file:line`.
- **Backing out is a rewrite.** Git is read-only, so capture `git diff` before a risky write and hand the user the `git checkout` command.
- **The bridge is optional, never blocking.** An unreachable MCP server is a skipped step. Never branch a phase on whether it is up.

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

**`/lifecycle open <topic>` skips every phase above.** For design, architecture, and exploration, where a script produces worse output than judgement.

It is named explicitly so that scripted stays the default. Use it when the task is to *decide* something, never to build something already decided.
