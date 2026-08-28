---
name: assess-simplify
description: Read the changed code for reuse, simplification, and efficiency cleanups, and file them as BACKLOG.md stories. Model-agnostic; quality only — never hunts for bugs or security defects. Pass --report to skip the write.
argument-hint: <task text or band summary> [standards-* skills that apply] [--report]
---

# Simplify assessment

Reviewing: **$ARGUMENTS**

Model-agnostic quality pass — Read/Grep/Glob/Bash only, no host-specific tooling. Findings only, never fixes — same contract as `assess-bugs`/`assess-security`, just a different lens.

## Process

1. **Load the `standards-*` skills named in `$ARGUMENTS`** for the reuse order (SDK first, vetted library second, custom last) and idiom conventions.
2. **Read `git diff` for the band.**
3. **For each changed block, check for**: duplicated logic already covered by an SDK/library call, a custom implementation of something the standards skill says exists, dead code, a function carrying more than one responsibility, an O(n²) shape where O(n) is available at no added complexity.
4. **Skip correctness and security** — that is `assess-bugs`/`assess-security`'s job, not this one's.

## Report

| Sev | Where | What | Win |
|---|---|---|---|
| M | `client.go:120` | hand-rolled retry loop | Forge SDK's `retry.Do` covers this, −18 lines |

**Sev**: High (a pattern the next change will copy) · Medium (contained duplication or a missed abstraction) · Low (naming/shape drift, no functional cost).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report** — a fix needs a concrete win (fewer lines, one fewer allocation, one fewer custom implementation), not just a different shape.

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <what> · S → \`/assess-simplify <file>\` reports it clear`. Append under the current target state (the first `##` heading) and the domain matching the file's area, or `Cross-Cutting` if none fits — full story-line rules live in `backlog`. `--report` prints the table only; nothing is written.
