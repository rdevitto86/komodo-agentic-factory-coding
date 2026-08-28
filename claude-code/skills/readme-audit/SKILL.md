---
name: readme-audit
description: Audit README.md against readme's fixed template — missing/extra sections, facts with no source, restated SDD depth, stale References — filed as BACKLOG.md stories. Pass --report to skip the write.
argument-hint: [--report]
disable-model-invocation: true
---

# README audit

Findings only, never edits `README.md` — `readme` owns the format and is the only skill that writes it. This locates structural drift, unsourced claims, and stale references, same division of labor `changelog`'s audit mode keeps with its own write mode.

Load `readme` first — every check below tests against rules it owns, not rules restated here.

## Process

1. **Read `README.md` in full.** No file at all is itself a finding — cite `readme`'s Scaffold branch, don't draft one.
2. **Check the fixed shape** — `# <repo-name>` H1, an optional table of contents, then exactly the six numbered sections in order (Overview, Features, Setup, Usage, Testing, References). A missing or extra section is a finding; a table of contents linking anything beyond the six headers is a finding too.
3. **Check every fact traces to a source** — a Setup command not in the actual build tooling, an env var/config key the repo doesn't read, a Usage route/export/job not found in the code, a Testing command not in the Makefile/scripts, a Features subsection naming a capability no code backs. An unsourced claim is a finding, not a style note.
4. **Check the Overview status line** — present and pointing at `BACKLOG.md` iff an open Blocker-tier item exists there; present on a clean backlog, or absent with one open, is a finding.
5. **Check §6 References** — lists only files actually present (`BACKLOG.md`, `CHANGELOG.md`, an API contract file); never lists the SDD or PRD — both are Drive docs, never repo files.
6. **Check for restated depth** — design rationale, infra diagrams, or endpoint-by-endpoint detail that belongs in the SDD and should stay there, not be inlined prose. §2 Features is the section most prone to this: a subsection that reads as a design writeup rather than a short paragraph is a finding.

## Report

| Kind | Where | Finding | Evidence |
|---|---|---|---|
| Unsourced claim | §3 Setup | env var `FOO_TIMEOUT` not read anywhere | grep, no hits |
| Stale reference | §6 References | links `docs/runbook/failover.md`, file deleted | `docs/runbook/` listing |
| Missing status | §1 Overview | open Blocker in `BACKLOG.md`, no status line | `BACKLOG.md` line |

**Sev**: Critical (a Setup/Usage command or route that doesn't exist — actively misleads a new reader) · High (missing required section, a References entry pointing at a deleted file, wrong Blocker status) · Medium (restated SDD depth, a drifted Testing command) · Low (wording, ordering nits).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <finding> · S → \`/readme-audit\` reports it clear`. Append under the current target state's `Cross-Cutting` domain — full story-line rules live in `backlog`. `--report` prints the table only; nothing is written.
