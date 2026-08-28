---
name: backlog
description: Create, edit, and audit BACKLOG.md — turn a goal into an epic-scoped breakdown, normalize an existing messy file into that shape, or verdict every open task against current repo state and apply the verdicts directly. Owns the BACKLOG.md format.
argument-hint: [plan <goal> | normalize <file> | audit [scope]]
---

# Backlog — BACKLOG.md

**Three modes, one file.** The first token in `$ARGUMENTS` picks the mode:

- **`plan <goal>`** → Part 1, the planning run. Turn a goal into an epic-scoped breakdown.
- **`normalize <file>`** → Part 2, normalizing an existing file. Turn a messy or unstructured file into the shape below.
- **`audit [scope]`** → Part 3, the audit. Verdict every open task against current repo state and apply the verdicts directly. Default scope is every open line in `BACKLOG.md`.

If `$ARGUMENTS` names a messy or unstructured file with no mode token, treat it as `normalize`. If `$ARGUMENTS` is empty or names no file, treat it as `audit` with no scope (its own default already covers that case).

Parts 1 and 2 are planning — write no implementation code during either. Part 3 judges and edits `BACKLOG.md` directly; it never writes implementation code either.

---

## The BACKLOG.md format

**A dashboard, not a log.** Open and just-finished work only — a `[DONE]` task is a pending sweep, not a permanent record; `CHANGELOG.md` and git carry history. `standards-worklog` states the read/write directive shared with `CHANGELOG.md`; this section is the format itself.

Hierarchy is fixed, four levels deep: **epic → task group → task → subtask.** This exact shape — headings, bracketed IDs, priority tags, bullet fields — is not a style choice; match it verbatim.

```markdown
# Project Backlog

## Convention Legend
* **Priority Tagging:** `[C]` Critical | `[H]` High | `[M]` Medium | `[L]` Low
* **Status Indicators:** `[TODO]` | `[IN_PROGRESS]` | `[BLOCKED]` | `[DONE]`
* **Hierarchy ID:** `EPIC-XX` -> `TG-XX.Y` (Task Group) -> `TSK-XX.Y.Z` (Task) -> `SUB-XX.Y.Z.N` (Subtask)

---

## [EPIC-01] Now, V1
*Goal: <the smallest thing that is genuinely usable — one line>*

### [TG-01.1] <task group>
* **Target Release:** V1

#### [TSK-01.1.1] <task text> [P: C] [TODO]
* **SUB-01.1.1.1** <what this piece of work is>
  * **Done when:** `<command>`
* **SUB-01.1.1.2** <what this piece of work is>
  * **Done when:** `<command>`

#### [TSK-01.1.2] <task text> [P: M] [TODO]
* **SUB-01.1.2.1** <what this piece of work is>
  * **Done when:** `<command>`

### [TG-01.2] <task group>
* **Target Release:** V1

#### [TSK-01.2.1] <task text> [P: H] [IN_PROGRESS]
* **SUB-01.2.1.1** <what this piece of work is>
  * **Done when:** `<command>`

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives — never for finished work, which is swept out on completion, not archived.*
```

Task heading shape: `#### [TSK-E.T.S] <text> [P: SEV] [STATUS]`. Subtask line shape: `* **SUB-E.T.S.N** <text>` — plain text, no checkbox — with a nested `* **Done when:**` bullet directly beneath it, holding one or more literal commands with an exit code.

**A subtask *is* the task's acceptance criterion, not a separate breakdown from it.** There is no JIRA-style AC list living apart from the subtask list — each `SUB-` line names one piece of work and carries the command that proves that piece is done, in one place. Where JIRA would write "AC-1, AC-2, AC-3" under a story, this file writes `SUB-E.T.S.1`, `SUB-E.T.S.2`, `SUB-E.T.S.3`, each with its own `Done when:`.

### Rules

- **No checkboxes, on any line.** A task's own state is its `[STATUS]` tag; a subtask carries no `[STATUS]` tag of its own — its `Done when:` command is what proves it, not a state written down separately. `CHANGELOG.md` is the completed-work record; `BACKLOG.md` tracking the same completion a second way (a checked box that then gets deleted anyway on sweep) is a distinction with no lasting value.
- **Every task carries at least one `SUB-` line, and every `SUB-` line carries its own `Done when:` bullet** with one or more literal commands, not a description — a command that exits zero, not "tests pass." `[DONE]` requires every one of a task's subtasks' commands to have exited zero; `workflow-implement` runs the whole set and reports each one's output. A subtask whose completion can only be judged by reading the code, not running something, is Ambiguous — see Part 3 — not plannable as-is. Subtasks are never invented just to fill the level out — one is enough for a task too small to need more.
- **`[P: SEV]` then `[STATUS]` sit at the end of the `TSK-` heading line, in that order, always present** — `#### [TSK-01.1.1] <text> [P: C] [TODO]`. `[TODO]` is the default for anything not started; move to `[IN_PROGRESS]` the moment work starts on it, `[BLOCKED]` per the shape below, `[DONE]` once every `SUB-` line's `Done when:` command is verified against the repo.
- **`[DONE]` is a pending sweep, not a resting state.** The task stays on the page — visible, but not counted as open — until `/backlog audit` moves it into `CHANGELOG.md` and deletes it. Never hand-delete a `[DONE]` task yourself; that's the sweep's job, and it's what confirms the entry lands in `CHANGELOG.md` first.
- **Epics** are `## [EPIC-01] Now, V1`, `## [EPIC-02] Next, V2` — every epic carries a one-line `*Goal: ...*` directly beneath its heading. Nothing is scheduled by date. A third, active epic is possible but rare — plan runs stick to V1/V2 (see Part 1).
- **Task groups** are `Cross-Cutting` or a feature/route/screen/stack/queue name **inside this one service** — never another service's name. Every task group carries a `* **Target Release:**` bullet directly beneath its heading, naming the version or milestone it ships with.
- **Tasks are flat under their task group** — no phase. Default is parallel.
- **Numbering is four levels deep, for reference, not for sequencing.** `EPIC-XX` (`01` for Now/V1, `02` for Next/V2, in file order). `TG-XX.Y`, `Y` numbered within its epic, in file order. `TSK-XX.Y.Z`, `Z` numbered within its task group, in file order — so V1's Cross-Cutting's first task is `TSK-01.1.1`. `SUB-XX.Y.Z.N`, `N` numbered within its task. Renumber whenever an epic, task group, task, or subtask is added, deleted, or reordered, so the numbers stay contiguous — this is a display convenience for saying "do TSK-01.1.1–TSK-01.1.4," never an ID stored anywhere else or referenced across files.
- **`(after: "<task text>")` is the only sequencing the file encodes**, appended to a task's own text, naming the task it must follow by its own text — never a number, and never an ID from another document. Numbers shift on renumbering; text doesn't.
- **Every task group with behavior tasks carries its own `Tests:` task.** That is the merge gate; integration, smoke, e2e, and perf get their own task. `standards-sdlc` defines the tiers.
- **In a repo `repo-init` scaffolds as an app/service/infra type** (`go-api`, `go-mcp`, `vue-ui`, `svelte-ui`, `cdk-infra` — see `repo-init`), **every epic's `Cross-Cutting` task group carries four standing closeout tasks** — `Security review`, `Bug sweep`, `Code smell`, `Performance`. They run last, after any Deploy tasks (see below): delete the epic's heading only once every other task is gone and these four are too (or swept, if left `[DONE]`). **A skill/config/doc-only repo — one `repo-init` never scaffolds as one of those types, this toolkit included — carries none of the four**; there is no runtime surface for a security scan, a perf suite, or a code-smell pass to cover, so `Cross-Cutting` in that kind of repo ends at whatever real tasks it holds.
- **Every task carries a `[P: SEV]` tag.** The task's own text names what the work is; each `SUB-` line's `Done when:` command is what states what "done" means for that piece — write it concretely enough that someone else can run it, not so vague it can only be judged by the person who wrote it.
- **An `assess-*` skill files its own findings straight in** — that is its own `Findings → backlog` step, not this skill's Parts 1/2. Those runs (a planning pass, a normalize pass) never invent a task from a finding it did not itself derive from the repo or the source file being normalized.

### Foundation and Deploy edges

`Cross-Cutting` has two fixed edges. Neither changes the rules above — same flat task list, same `(after:)` sequencing, same four closeout tasks last.

- **Foundation, first.** Repo skeleton, toolchain floor, container build, health endpoint — what `repo-init` Create already seeds. On Scaffold/Refresh of a pre-existing repo these surface as real open tasks instead of pre-satisfied ones.
- **Deploy, last — before the four closeout tasks.** CI deploy pipeline, STG rollout, PROD rollout. The task-group rule still applies: a service repo's Deploy tasks cover *becoming deployable* (build, push, wire the pipeline). The cloud infra itself is a task in the infra repo's own `Cross-Cutting`, never this one.
- **A Deploy task blocked on something outside this repo is still `[BLOCKED]`, same shape as any other** — the citation just points at the other repo's record instead of a code defect:

  ```markdown
  #### [TSK-01.4.1] <deploy task text> [P: H] [BLOCKED]
  * **Blocked By:** `external`
    * **Reason (YYYY-MM-DD):** <why the other repo's own open block stops this
      one, four sentences maximum>
    * **Citation:** <the other repo's record — e.g. its `BACKLOG.md`>
    * **Recheck:** <cheap, testable condition — e.g. the other repo's
      `BACKLOG.md` no longer lists that task as `[BLOCKED]`>
  ```

### A blocked task

**Stopping is a result, not a failure to report.** Keep the heading, set `[BLOCKED]`, and add a `* **Blocked By:**` bullet directly beneath the heading, holding three nested fields — name the in-file task it's blocked on if there is one (`TSK-01.1.2`), or `external` if the blocker sits outside this file, then a dated `Reason:` (`YYYY-MM-DD`, four sentences maximum), a `Citation:` (`file:line`, or the other repo's record for an external block), and a `Recheck:` naming a cheap, testable condition — which is what lets `/workflow-decompose` clear the block on a later pass automatically instead of it sitting blocked forever:

```markdown
#### [TSK-01.2.1] <task text> [P: H] [BLOCKED]
* **Blocked By:** `external`
  * **Reason (YYYY-MM-DD):** <what's missing and why it blocks this task, four
    sentences maximum>
  * **Citation:** `<file:line>`
  * **Recheck:** `<command that shows the blocker is gone>`, or a written
    risk-acceptance decision is recorded
```

**A blocked note without a citation is a guess.** If nothing testable exists for `Recheck:`, the block is a decision for the user, not a task state; say so instead of inventing a condition.

---

# Part 1 — The planning run (`plan <goal>`)

Target: **$ARGUMENTS**, minus the `plan` token.

You are planning, not building. Write no implementation code during this mode.

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

- **Read the SDD (a repo file under `docs/spec/SDD.md` — see `standards-specs`) if one exists** for architecture, data model, and interface context — the source of truth for a code repo. It informs the plan; it does not hand you a ready-made decomposition — that is this mode's own job.
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

The format above governs shape. Four constraints are the planning run's own:

- **Cap each run at 3 task groups and 6 tasks per task group.** If the work genuinely exceeds that, plan V1 only and say V2 needs its own pass. An unbounded dump is what made the old ledger useless.
- **In an app/service/infra repo (see the `Cross-Cutting` rule above), the four closeout tasks are structural, not planned content** — exempt from the cap, and never write them yourself during a planning run; `normalize` (Part 2) is what adds them if a `Cross-Cutting` group is missing any. A skill/config/doc-only repo carries none of the four — don't add them there either.
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

Each `Done when` cell collapses that task's `SUB-` lines' commands into one row for this preview; Step 6 writes them out as the actual nested `SUB-`/`Done when:` bullets, one subtask per command.

## ⚠️ Risks
- **<thing>** — why it could bite

## ❓ Open
- <anything still unresolved>
```

## Step 6 — Write on approval only

Merge into `BACKLOG.md` per the format above. Never write before approval.

- **Append under the right epic and task group.** Never create a second task group with the same name.
- **Renumber the file's `TG-`/`TSK-`/`SUB-` tags after merging** so they stay contiguous — a new task group, task, or subtask never leaves a gap or reuses a number already on the page.
- **Never remove a task you did not add** unless the user says so.
- **Never rewrite another task's `(after: ...)` tag** — that dependency was true when someone else wrote it; if it is stale, ask instead of silently dropping it.

---

# Part 2 — Normalizing an existing file (`normalize <file>`)

Same command family, different ask: **$ARGUMENTS**, minus the `normalize` token, names a messy or unstructured file — a backlog with no hierarchy, dates, or already-done items; scattered `// TODO` comments; a plain notes file.

## Step 1 — Read everything, invent nothing

Read the source file(s) in full. Every line becomes exactly one of:

- **A genuinely open task** → keep, rewritten to the shape above, `[TODO]` unless the source says work is active (`[IN_PROGRESS]`), broken into at least one `SUB-` line. If the source names a runnable check for a piece of it, carry that over as that subtask's `Done when:` bullet; if it only names a description, do not invent a command — see the vague case below.
- **Something already done** (phrased as done, or contradicted by the current code) → drop, and add it to `CHANGELOG.md` if it is not already recorded. Verify against the code before dropping — never drop on the comment's word alone.
- **A date, a name, a status log entry** → drop; git already carries that history.
- **Too vague to act on** (no runnable `Done when` command derivable) → keep as an `L`-priority, `[TODO]` task naming exactly what is unclear, with a single `SUB-` line and no `Done when:` bullet beneath it. Never invent a command the source did not state.

## Step 2 — Sort into the hierarchy

Assign each surviving task an epic (default `## [EPIC-01] Now, V1` unless the source clearly marks it future work) and a task group (the feature or route it belongs to — infer from the file's path or the task's own text, never a new taxonomy, never another service's name).

If the repo is an app/service/infra type (see the `Cross-Cutting` rule above) and an epic's `Cross-Cutting` task group is missing any of the four closeout tasks, add the missing ones. Skip this for a skill/config/doc-only repo — it carries none of the four.

## Step 3 — Present, then stop

Show the normalized file as a diff against the source and **wait for approval** before writing — this replaces the user's existing file, so it is never silent.

---

# Part 3 — Audit (`audit [scope]`)

Scoping: **$ARGUMENTS**, minus the `audit` token (default: every open line in `BACKLOG.md`)

Judges backlog validity against current repo state and applies the verdict directly — same as the other `assess-*` skills write their findings straight to `BACKLOG.md`, this mode edits `BACKLOG.md` itself rather than filing a new story about it. Parts 1 and 2 above stay reserved for the one-time planning run (a fresh target-state decomposition) and for normalizing an unstructured source file — not for routine maintenance of an already-shaped `BACKLOG.md`, which this mode (and the other assessments) handle directly.

**Lighter-weight than a full `/workflow-decompose` re-derivation.** `/workflow-loop`'s P1 runs this before the decompose fork, on the same scope — an already-valid backlog skips the fork's full repo+changelog re-derivation; only what this flags needs decompose's attention.

## Process

1. Read `BACKLOG.md` in full — every open task, every `[BLOCKED]` task's `Blocked By:` bullet, every `[DONE]` task, within scope.
2. For each task, check it against the current repo (code, `CHANGELOG.md`, the file's other tasks) and verdict it:

| Verdict | Test |
|---|---|
| Valid | Still accurate, still open, still concretely checkable |
| Resolved | Already true in the code — should have been deleted, not left open |
| Stale | Repo state moved past what the task describes; it no longer makes sense as written |
| Duplicate | Overlaps another open task's scope |
| Ambiguous | A `SUB-` line missing a `Done when:` bullet, or one whose bullets are prose rather than runnable commands |

Absence of contradiction is not validity. A task naming no file, command, or artifact — text like "once X is scoped" — can't be checked against current repo state at all; "nothing in the repo contradicts it" looks identical whether the task is a live placeholder or a dead fragment nothing ever backed. For any task in this shape, `git blame`/`git log -S '<task text>'` its introduction and check `CHANGELOG.md` for whether the thing it references (the suite, the tool, the flag) was ever real. No commit ever built it → **Stale**, not Valid.

3. For any `[BLOCKED]` task in scope, test its `Blocked By:` bullet's `Recheck:` clause against current state — flag if it now passes.
4. For any `[DONE]` task in scope, run every command in every `SUB-` line's `Done when:` bullet and confirm each still exits zero before sweeping it; a `[DONE]` tag someone set without that being true is Ambiguous, not swept.

## Findings → backlog

Verdicting *is* the edit — apply each one directly to `BACKLOG.md`, not by filing a new task:

| Verdict | Applied as |
|---|---|
| Valid | No edit |
| Resolved | Delete the task. If the change it describes isn't already recorded in `CHANGELOG.md`, add it there — `standards-worklog` covers the read/write directive. |
| Stale | Delete the task — it no longer describes anything the repo can act on. |
| Duplicate | Delete the weaker of the two tasks (less specific text, or the one added later per `git log`) and keep the other. |
| Ambiguous | Leave the task as-is. This is a human call, not an edit this mode makes on its own — flag it in the report instead. |
| `[BLOCKED]` whose `Recheck:` now passes | Set the task's status back to `[TODO]` or `[IN_PROGRESS]` and remove its `Blocked By:` bullet. |
| `[DONE]`, confirmed true in the repo | Delete the task, its `Blocked By:` bullet, and its `SUB-` lines (each with its nested `Done when:` bullet). If not already recorded, add it to `CHANGELOG.md` — `standards-worklog` covers the read/write directive. |

Never invent a new task from a verdict — verdicting "this task is stale" means deleting that task, never filing a fresh one describing the staleness. That circularity is exactly what stays out of scope here.

## Report

After applying edits, report what changed:

| Task | Verdict | Action | Why |
|---|---|---|---|
| `[TSK-E.T.S] <text>` | Stale | Deleted | `<file:line or command that proves it>` |
| `[TSK-E.T.S] <text>` | Ambiguous | Left open — needs a human call | `<why nothing here is checkable>` |

No findings (every task valid, no `[BLOCKED]` recheck cleared, nothing edited): state that plainly, one line, and stop. **Never invent a finding to have something to report or to edit.**
</content>
