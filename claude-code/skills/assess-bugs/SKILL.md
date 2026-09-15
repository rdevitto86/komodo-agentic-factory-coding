---
name: assess-bugs
description: Read the diff for correctness bugs against the task it claims to satisfy — logic errors, edge cases, wrong assumptions — and return them as a findings table. Model-agnostic finder; never a fixer.
argument-hint: a reviewer brief — Task (band summary) | Files | Context | Round | Standards | Out of scope (all six required)
context: fork
agent: reviewer
background: false
---

# Bug assessment

Brief: **$ARGUMENTS** — a `reviewer` brief carrying `Task`, `Files`, `Context`, `Round`, `Standards`, and `Out of scope`. A missing or empty one of those is your standing stop, not something to infer from the diff.

Model-agnostic finder — Read/Grep/Glob/Bash only, no host-specific tooling, usable from any bridge-connected model. Findings only, never fixes — a correctness or requirement finding routes back to implementation; report the rest and stop.

## Process

1. **Load the `standards-*` skills the brief's `Standards` slot names** — read their Testing and Conventions sections before scoring anything a bug.
2. **Read `git diff` for the band**, not the whole tree. A file untouched by this band is out of scope.
3. **For each changed function/block, check**: does it do what the task said, including the edge case the task named? A wrong assumption, an unhandled input, a race, an off-by-one, a swallowed error.
4. **A finding needs a concrete trigger path**, not a hypothetical. "Could theoretically" is not a finding; "input X reaches the unguarded branch at Y, effect Z follows" is.
5. **Skip style, naming, reuse, and efficiency** — that is `assess-simplify`'s job, not this one's.

## Report

`reviewer.md`'s section structure governs — a near-miss goes to `## Considered and dismissed`, not here. This table's columns and `Sev` scale are this lens's own.

| Sev | Where | Claim | Scenario |
|---|---|---|---|
| H | `file.go:42` | swallows the write error | retry after crash → silent data loss |

**Diagram form**: a Mermaid `graph LR` — one node per cited `file:line`, one edge where a finding's effect is another finding's trigger. `reviewer.md` says when it is drawn and where it sits.

**Sev**: Critical (data loss/crash/security on a path the target state depends on) · High (wrong result reachable in normal use) · Medium (edge case, recoverable) · Low (cosmetic or needs an unlikely input).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

The caller files these rows to `BACKLOG.md`; you never write.
