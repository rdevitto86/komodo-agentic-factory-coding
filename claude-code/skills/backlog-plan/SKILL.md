---
name: backlog-plan
description: Turn a goal into an epic-scoped BACKLOG.md breakdown — the planning run, split out from backlog-modify's plan mode.
argument-hint: <goal>
---

# Backlog plan — the planning run

Load `backlog-modify` first — every shape rule below (the BACKLOG.md format, its Rules, the Foundation/Deploy edges, a blocked task) is owned there, not restated here.

Target: **$ARGUMENTS**.

You are planning, not building. Write no implementation code during this run. Verdicting existing tasks against current repo state (audit) is `backlog-audit`'s job, not this skill's.

## Step 1 — Ask before assuming

**Ask your questions first, in one batch, before producing any plan.** A plan built on a guess wastes more time than a question costs.

Ask only what changes the plan. Skip anything you can determine by reading the repo — read it instead.

Typical unknowns worth asking:

- **Scope boundary** — what is explicitly *not* in V1?
- **Existing surface** — is this extending something already built, or greenfield?
- **Hard constraints** — a deadline, a dependency that must land first, a decision already made.
- **Done condition** — what has to be true for this to ship?


If the repo answers a question, do not ask it. If nothing is genuinely unclear, say so and move on.

## Step 2 — Read the ground truth

Before proposing anything:

- **Read the SDD (a repo file under `docs/spec/SDD.md` — see `standards-specs`) if one exists** for architecture, data model, and interface context — the source of truth for a code repo. It informs the plan; it does not hand you a ready-made decomposition — that is this run's own job.
- **Read the existing `BACKLOG.md`.** Never duplicate a task already in it.
- **Read the code that this work touches.** The current state beats any ledger.
- **Read the project `AGENTS.md`** for stack and conventions.

State plainly if the repo contradicts what the user described. That contradiction is usually the most valuable output of the whole exercise.

## Step 3 — Name the epics

Two epics, no more:

- **V1** — the smallest thing that is genuinely usable. Not a prototype, not feature-complete. Lands under `## [EPIC-01] Now, V1`.
- **V2** — the next coherent increment. Lands under `## [EPIC-02] Next, V2`.

Anything beyond V2 is speculation and does not belong in a plan. **A planning run never writes a third active epic** — that's parked work the user put there themselves.

## Step 4 — Decompose

`backlog-modify`'s format governs shape. Four constraints are the planning run's own:

- **Cap each run at 3 task groups and 6 tasks per task group.** If the work genuinely exceeds that, plan V1 only and say V2 needs its own pass. An unbounded dump is what made the old ledger useless.
- **In an app/service/infra repo (see `backlog-modify`'s closeout-task rule), the four closeout tasks and their `Quality Assurance & Epic Hardening` task group are structural, not planned content** — exempt from the cap, and never write them yourself during a planning run; `backlog-modify`'s `normalize` mode is what adds the task group (and any missing task) when it's absent or incomplete. A skill/config/doc-only repo carries neither — don't add them there either.
- **Default every task to parallel** — mark `(after: "<task text>")` only where one task genuinely cannot start before another lands, never to impose an arbitrary order.
- **Every task carries at least one `SUB-` line, and every `SUB-` line carries a `Done when:` command.** A task nobody else can run is not planned, it is hoped for. If nothing runnable can be named, that is the finding — say so.

## Step 5 — Present, then stop

Show the plan and **wait for approval**. Do not write to `BACKLOG.md` yet.

```markdown
## 🎯 Epics
- **V1** — <one line>
- **V2** — <one line>

## 📋 Plan
### [TG-E.T] <task group>
| Task `[TSK-E.T.S] [P: sev]` | Done when |
|---|---|
| <what> `[TSK-E.T.S] [P: H]` | `<command 1>`; `<command 2>` |

Each `Done when` cell collapses that task's `SUB-` rows' commands into one row for this preview; Step 6 writes them out as the actual per-task Subtask table, one row per command.

## ⚠️ Risks
- **<thing>** — why it could bite

## ❓ Open
- <anything still unresolved>
```

## Step 6 — Write on approval only

Merge into `BACKLOG.md` per `backlog-modify`'s format. Never write before approval.

- **Append under the right epic and task group.** Never create a second task group with the same name.
- **Renumber the file's `TG-`/`TSK-`/`SUB-` tags after merging** so they stay contiguous — a new task group, task, or subtask never leaves a gap or reuses a number already on the page.
- **Never remove a task you did not add** unless the user says so.
- **Never rewrite another task's `(after: ...)` tag** — that dependency was true when someone else wrote it; if it is stale, ask instead of silently dropping it.
