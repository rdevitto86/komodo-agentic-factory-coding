---
name: backlog-audit
description: Verdict every open BACKLOG.md task against current repo state and apply the verdicts directly — resolved, stale, duplicate, ambiguous, or a cleared block.
argument-hint: [scope]
context: fork
agent: workflow-implementer
background: false
---

# Backlog audit

Judges backlog validity against current repo state and applies the verdict directly — same as the other `assess-*` skills write their findings straight to `BACKLOG.md`, this skill edits `BACKLOG.md` itself rather than filing a new story about it. `backlog-modify` stays reserved for the one-time planning run (a fresh target-state decomposition) and for normalizing an unstructured source file — not for routine maintenance of an already-shaped `BACKLOG.md`, which this skill (and the other assessments) handle directly.

Load `backlog-modify` first — every check below tests against the format and rules it owns, not rules restated here.

**Lighter-weight than a full `/workflow-decompose` re-derivation.** `/workflow-loop`'s P2.4 runs this once per band, over the whole file, after `/changelog write` — realigning the backlog against what the band just shipped rather than gatekeeping P1's pick. P1 itself runs no audit; it leans on P0's existence check and picks its scope straight off the file's own priority order.

## Process

Scoping: **$ARGUMENTS** (default: every open line in `BACKLOG.md`)

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
| Ambiguous | Leave the task as-is. This is a human call, not an edit this skill makes on its own — flag it in the report instead. |
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
