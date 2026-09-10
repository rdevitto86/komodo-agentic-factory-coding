---
name: backlog-modify
description: Reshape BACKLOG.md — normalize an existing messy file into the format this skill owns.
argument-hint: normalize <file>
---

# Backlog — BACKLOG.md

**One mode: `normalize <file>`.** Turn a messy or unstructured file into the shape below. Turning a goal into a new epic-scoped breakdown is `backlog-plan`'s job now, not this skill's — see that skill.

If `$ARGUMENTS` names a messy or unstructured file with no mode token, treat it as `normalize`.

This mode is planning — write no implementation code. Verdicting existing tasks against current repo state (audit) is `backlog-audit`'s job, not this skill's — see that skill for edits to already-shaped content.

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
* **Owner (optional):** an `Owner` row in a task's field table marks work only a person can resolve — a policy call, a risk-acceptance decision. Absent means agent-executable by default.

---

## [EPIC-01] Now, V1
*Goal: <the smallest thing that is genuinely usable — one line>*

### [TG-01.1] <task group>
* **Target Release:** V1

#### [TSK-01.1.1] <task text> [P: C] [TODO]
| Subtask | Work | Done when |
|---|---|---|
| `SUB-01.1.1.1` | <what this piece of work is> | `<command>` |
| `SUB-01.1.1.2` | <what this piece of work is> | `<command>` |

#### [TSK-01.1.2] <task text> [P: M] [TODO]
| Subtask | Work | Done when |
|---|---|---|
| `SUB-01.1.2.1` | <what this piece of work is> | `<command>` |

### [TG-01.2] <task group>
* **Target Release:** V1

#### [TSK-01.2.1] <task text> [P: H] [IN_PROGRESS]
| Subtask | Work | Done when |
|---|---|---|
| `SUB-01.2.1.1` | <what this piece of work is> | `<command>` |

---

## Archive
*Note: use this section strictly for abandoned, shelved, or deprecated initiatives — never for finished work, which is swept out on completion, not archived.*
```

Task heading shape: `#### [TSK-E.T.S] <text> [P: SEV] [STATUS]` — this line alone is regex-read by `context_injector.py` at every session start, so its shape is fixed. Everything beneath it is free-form and uses tables, not bullets.

Subtask shape: one row per subtask, in a table directly beneath the task heading — `| SUB-E.T.S.N | <text> | <command> |`. A subtask carries no `[STATUS]` tag of its own; the `Done when` cell holding one or more literal commands with an exit code is what proves it, not a checkbox.

**A subtask *is* the task's acceptance criterion, not a separate breakdown from it.** There is no JIRA-style AC list living apart from the subtask table — each row names one piece of work and carries the command that proves that piece is done, in one place. Where JIRA would write "AC-1, AC-2, AC-3" under a story, this file writes `SUB-E.T.S.1`, `SUB-E.T.S.2`, `SUB-E.T.S.3` as rows, each with its own `Done when` cell.

### Rules

- **No checkboxes, anywhere, and no bullet-nested fields — a subtask and a blocked/owner field are table rows**, not a bulleted list. A task's own state is its `[STATUS]` tag; a subtask's row carries no status of its own. `CHANGELOG.md` is the completed-work record; `BACKLOG.md` tracking the same completion a second way (a checked box that then gets deleted anyway on sweep) is a distinction with no lasting value.
- **Every task carries a Subtask table with at least one row, and every row carries a `Done when` cell** holding one or more literal commands, not a description — a command that exits zero, not "tests pass." `[DONE]` requires every one of a task's subtask commands to have exited zero; `workflow-implement` runs the whole set and reports each one's output. A subtask whose completion can only be judged by reading the code, not running something, is Ambiguous — see `backlog-audit` — not plannable as-is. Subtasks are never invented just to fill the table out — one row is enough for a task too small to need more.
- **`[P: SEV]` then `[STATUS]` sit at the end of the `TSK-` heading line, in that order, always present** — `#### [TSK-01.1.1] <text> [P: C] [TODO]`. `[TODO]` is the default for anything not started; move to `[IN_PROGRESS]` the moment work starts on it, `[BLOCKED]` per the shape below, `[DONE]` once every `SUB-` line's `Done when:` command is verified against the repo.
- **`[DONE]` is a pending sweep, not a resting state.** The task stays on the page — visible, but not counted as open — until `/backlog-audit` moves it into `CHANGELOG.md` and deletes it. Never hand-delete a `[DONE]` task yourself; that's the sweep's job, and it's what confirms the entry lands in `CHANGELOG.md` first.
- **Epics** are `## [EPIC-01] Now, V1`, `## [EPIC-02] Next, V2` — every epic carries a one-line `*Goal: ...*` directly beneath its heading. Nothing is scheduled by date. A third, active epic is possible but rare — plan runs stick to V1/V2 (see `backlog-plan`).
- **Task groups** are `Cross-Cutting` or a feature/route/screen/stack/queue name **inside this one service** — never another service's name. Every task group carries a `* **Target Release:**` bullet directly beneath its heading, naming the version or milestone it ships with.
- **Tasks are flat under their task group** — no phase. Default is parallel.
- **Numbering is four levels deep, for reference, not for sequencing.** `EPIC-XX` (`01` for Now/V1, `02` for Next/V2, in file order). `TG-XX.Y`, `Y` numbered within its epic, in file order. `TSK-XX.Y.Z`, `Z` numbered within its task group, in file order — so V1's Cross-Cutting's first task is `TSK-01.1.1`. `SUB-XX.Y.Z.N`, `N` numbered within its task. Renumber whenever an epic, task group, task, or subtask is added, deleted, or reordered, so the numbers stay contiguous — this is a display convenience for saying "do TSK-01.1.1–TSK-01.1.4," never an ID stored anywhere else or referenced across files.
- **`(after: "<task text>")` is the only sequencing the file encodes**, appended to a task's own text, naming the task it must follow by its own text — never a number, and never an ID from another document. Numbers shift on renumbering; text doesn't.
- **Every task group with behavior tasks carries its own `Tests:` task.** That is the merge gate; integration, smoke, e2e, and perf get their own task. `standards-sdlc` defines the tiers.
- **In a repo `git-repo-init` scaffolds as an app/service/infra type** (`go-api`, `go-mcp`, `vue-ui`, `svelte-ui`, `cdk-infra` — see `git-repo-init`), **every epic's `Cross-Cutting` task group carries four standing closeout tasks** — `Security review`, `Bug sweep`, `Code smell`, `Performance`. They run last, after any Deploy tasks (see below): delete the epic's heading only once every other task is gone and these four are too (or swept, if left `[DONE]`). **A skill/config/doc-only repo — one `git-repo-init` never scaffolds as one of those types, this toolkit included — carries none of the four**; there is no runtime surface for a security scan, a perf suite, or a code-smell pass to cover, so `Cross-Cutting` in that kind of repo ends at whatever real tasks it holds.
- **Every task carries a `[P: SEV]` tag.** The task's own text names what the work is; each `SUB-` line's `Done when:` command is what states what "done" means for that piece — write it concretely enough that someone else can run it, not so vague it can only be judged by the person who wrote it.
- **An `assess-*` skill files its own findings straight in** — that is its own `Findings → backlog` step, not this skill's `normalize`, nor `backlog-plan`'s planning run. Those runs (a planning pass, a normalize pass) never invent a task from a finding it did not itself derive from the repo or the source file being normalized.

### Foundation and Deploy edges

`Cross-Cutting` has two fixed edges. Neither changes the rules above — same flat task list, same `(after:)` sequencing, same four closeout tasks last.

- **Foundation, first.** Repo skeleton, toolchain floor, container build, health endpoint — what `git-repo-init` Create already seeds. On Scaffold/Refresh of a pre-existing repo these surface as real open tasks instead of pre-satisfied ones.
- **Deploy, last — before the four closeout tasks.** CI deploy pipeline, STG rollout, PROD rollout. The task-group rule still applies: a service repo's Deploy tasks cover *becoming deployable* (build, push, wire the pipeline). The cloud infra itself is a task in the infra repo's own `Cross-Cutting`, never this one.
- **A Deploy task blocked on something outside this repo is still `[BLOCKED]`, same shape as any other** — the citation just points at the other repo's record instead of a code defect:

  ```markdown
  #### [TSK-01.4.1] <deploy task text> [P: H] [BLOCKED]
  | Field | Value |
  |---|---|
  | Blocked by | `external` |
  | Reason (YYYY-MM-DD) | <why the other repo's own open block stops this one, four sentences maximum> |
  | Citation | <the other repo's record — e.g. its `BACKLOG.md`> |
  | Recheck | <cheap, testable condition — e.g. the other repo's `BACKLOG.md` no longer lists that task as `[BLOCKED]`> |
  ```

### A blocked task

**Stopping is a result, not a failure to report.** Keep the heading, set `[BLOCKED]`, and add a field table directly beneath the heading, holding four rows — name the in-file task it's blocked on if there is one (`TSK-01.1.2`), or `external` if the blocker sits outside this file, then a dated `Reason` (`YYYY-MM-DD`, four sentences maximum), a `Citation` (`file:line`, or the other repo's record for an external block), and a `Recheck` naming a cheap, testable condition — which is what lets `/workflow-decompose` clear the block on a later pass automatically instead of it sitting blocked forever:

```markdown
#### [TSK-01.2.1] <task text> [P: H] [BLOCKED]
| Field | Value |
|---|---|
| Blocked by | `external` |
| Reason (YYYY-MM-DD) | <what's missing and why it blocks this task, four sentences maximum> |
| Citation | `<file:line>` |
| Recheck | `<command that shows the blocker is gone>`, or a written risk-acceptance decision is recorded |
```

**A blocked note without a citation is a guess.** If nothing testable exists for `Recheck:`, the block is a decision for the user, not a task state; say so instead of inventing a condition.

**A task done here but gated on a follow-up landing elsewhere stays `[BLOCKED]`, never `[DONE]` early.** The same field table covers it — `Blocked by: external`, `Reason` states the work here is complete and what it's waiting on, `Citation` names the other PR/repo record, `Recheck` names the merge condition (e.g. "`other-repo#218` shows `MERGED`"). Add an `Owner: human` row above it when only a person can confirm that merge, not an agent. This is the one sanctioned way to record "implementation done, acceptance pending" — never a third status invented for it.

**Turning a goal into a new epic-scoped breakdown lives in `backlog-plan` now** — a separate skill, extracted from this one's former planning-run mode. This file keeps only `normalize`.

---

# Normalizing an existing file (`normalize <file>`)

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
