---
name: backlog-prioritize
description: Reorder and re-prioritize BACKLOG.md's existing tasks — priority tag, file order within a task group, and epic placement — without inventing, rewriting, or deleting any task's own text.
argument-hint: [optional: what's driving the reprioritization, or a scope to limit it to]
disable-model-invocation: true
---

# Backlog prioritization

Driver: **$ARGUMENTS** (default: re-sort strictly by the file's own `[P: SEV]` tags and existing `(after: ...)` edges, no external driver)

Reorders what's already in `BACKLOG.md`. Never writes a new task, never edits a task's own text — that's `backlog`'s planning/normalize role (or its own audit mode's verdict role for validity, not priority). This skill only moves tasks: which epic a task sits under, where it falls within its task group's file order, and its `[P: SEV]` tag when the driver in `$ARGUMENTS` justifies a change.

## Process

1. **Read `BACKLOG.md` in full.** Every epic, every task group, every task, every `[BLOCKED]`/`[DONE]` status, every `SUB-` line and `(after: ...)` edge in scope.
2. **Read `$ARGUMENTS`.** If it names a driver (a deadline, an incident, a stakeholder ask, "ship the auth work before anything else"), that's what re-ranks priority and epic placement. If it names a scope instead (an epic, a task group), limit the pass to it. If empty, the only driver is the file's own `[P: SEV]` tags — this becomes a pure priority/dependency sort, no re-ranking judgment call.
3. **Resolve every `(after: "<task text>")` edge first.** A task can never sort ahead of a task it names — treat this as a hard constraint, not a preference, same as `workflow-decompose` does when it marks transitive blocks.
4. **Re-sort within each task group** by priority (`C` → `H` → `M` → `L`), honoring step 3's ordering constraints. Two tasks of equal priority with no dependency between them keep their relative file order — never invent a tiebreak the driver didn't supply.
5. **Re-rank `[P: SEV]` only when `$ARGUMENTS` gives a concrete reason to** (a task now blocks a named deadline, an incident elevated it, the user named it directly) — cite the reason next to the change in the report. Never bump priority on a hunch.
6. **Move a task to a different epic only when the driver justifies it** — pulling V2 work into V1 because it's now urgent, or pushing a V1 task to V2 because something outranked it. `[BLOCKED]` tasks move only if the block itself is what's being deprioritized, never silently.
7. **Never touch `[DONE]` tasks** — those are pending a `/backlog audit` sweep, not live priority.
8. **Never touch the four standing closeout tasks' position** — `Security review`, `Bug sweep`, `Code smell`, `Performance` stay last in their epic's `Cross-Cutting` task group, per `backlog`'s fixed edges.

## Applying the reorder

- **Move tasks, don't rewrite them.** A relocated task keeps its exact heading text, `[P: SEV]` (unless step 5 applies), `[STATUS]` tag, and any `Blocked By:`/`SUB-` lines beneath it.
- **Renumber `EPIC-`/`TG-`/`TSK-`/`SUB-` IDs after the move** so they stay contiguous in the new file order — `backlog`'s numbering rule applies here exactly as it does after an edit.
- **Never create a new task group** to hold a moved task — if no existing task group fits, that's a finding for the user, not a decision this skill makes on its own.
- **Never merge or split task groups.**

## Report

After applying, report what moved and why:

| Task | From | To | Why |
|---|---|---|---|
| `[TSK-...] <task text>` | `EPIC-01 / TG-01.2` pos 3 | `EPIC-01 / TG-01.1` pos 1 | `<driver from $ARGUMENTS, or "priority sort">` |

No reordering needed (file already matches priority + dependency order, nothing in `$ARGUMENTS` justifies a move): state that plainly, one line, and stop. **Never reorder for the sake of having something to report.**
