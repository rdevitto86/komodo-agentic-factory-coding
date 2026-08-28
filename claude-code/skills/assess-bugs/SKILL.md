---
name: assess-bugs
description: Read the diff for correctness bugs against the task it claims to satisfy — logic errors, edge cases, wrong assumptions — and file them as BACKLOG.md stories. Model-agnostic finder; never a fixer. Pass --report to skip the write.
argument-hint: <task text or band summary> [standards-* skills that apply] [--report]
---

# Bug assessment

Reviewing: **$ARGUMENTS**

Model-agnostic finder — Read/Grep/Glob/Bash only, no host-specific tooling, usable from any bridge-connected model. Never a fork of the session that wrote the code: read the diff cold, without the reasoning that produced it. Findings only, never fixes — a correctness or requirement finding routes back to implementation; report the rest and stop.

## Process

1. **Load the `standards-*` skills named in `$ARGUMENTS`** (or infer from touched file extensions if none named) — read their Testing and Conventions sections before scoring anything a bug.
2. **Read `git diff` for the band**, not the whole tree. A file untouched by this band is out of scope.
3. **For each changed function/block, check**: does it do what the task said, including the edge case the task named? A wrong assumption, an unhandled input, a race, an off-by-one, a swallowed error.
4. **Skip style, naming, reuse, and efficiency** — that is `assess-simplify`'s job, not this one's.

## Report

| Sev | Where | Claim | Scenario |
|---|---|---|---|
| H | `file.go:42` | swallows the write error | retry after crash → silent data loss |

**Sev**: Critical (data loss/crash/security on a path the target state depends on) · High (wrong result reachable in normal use) · Medium (edge case, recoverable) · Low (cosmetic or needs an unlikely input).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <claim> · S → \`/assess-bugs <file>\` reports it clear`. Append under the current target state (the first `##` heading) and the domain matching the file's area, or `Cross-Cutting` if none fits — full story-line rules live in `backlog-modify`. `--report` prints the table only; nothing is written.
