---
name: audit-backlog
description: Verdict every open BACKLOG.md line against current repo state — valid, resolved, stale, duplicate, or ambiguous — and recheck any [BLOCKED] line's Recheck: condition. Findings only, never edits the file.
argument-hint: [optional: target state or domain to scope the pass]
---

# Backlog audit

Scoping: **$ARGUMENTS** (default: every open line in `BACKLOG.md`)

Judges backlog validity against current repo state. Never a fixer — `generate-backlog`'s own rule is "never dump audit or review findings straight in"; this reports, the user or a follow-up `/generate-backlog` normalize pass decides what changes.

**Lighter-weight than a full `/workflow-decompose` re-derivation.** `/workflow-loop`'s P1 runs this before the decompose fork, on the same scope — an already-valid backlog skips the fork's full repo+changelog re-derivation; only what this flags needs decompose's attention.

## Process

1. Read `BACKLOG.md` in full — every open line, every `[BLOCKED]` note, within scope.
2. For each line, check it against the current repo (code, `CHANGELOG.md`, the file's other lines) and verdict it:

| Verdict | Test |
|---|---|
| Valid | Still accurate, still open, `Done when` still executable |
| Resolved | Already true in the code — should have been deleted, not left open |
| Stale | Repo state moved past what the line describes; it no longer makes sense as written |
| Duplicate | Overlaps another open line's scope |
| Ambiguous | No `Done when` derivable, or the command it names no longer resolves to anything |

Absence of contradiction is not validity. A line naming no file, command, or artifact — a `Done when` like "once X is scoped" — can't be checked against current repo state at all; "nothing in the repo contradicts it" looks identical whether the line is a live placeholder or a dead fragment nothing ever backed. For any line in this shape, `git blame`/`git log -S '<line text>'` its introduction and check `CHANGELOG.md` for whether the thing it references (the suite, the tool, the flag) was ever real. No commit ever built it → **Stale**, not Valid.

3. For any `[BLOCKED]` line in scope, test its `Recheck:` condition against current state — flag if it now passes.

## Report

| Line | Verdict | Why |
|---|---|---|
| `<line text>` | Stale | `<file:line or command that proves it>` |

No findings (every line valid, no `[BLOCKED]` recheck cleared): state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## No `Findings → backlog` step

The other seven `audit-*` skills file each finding as a new `BACKLOG.md` story. This one's findings are verdicts *about existing `BACKLOG.md` lines* — filing "this line is stale" as a new line would be circular. Report only; the user or a `/generate-backlog` normalize pass acts on the verdict.
