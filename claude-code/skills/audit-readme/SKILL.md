---
name: audit-readme
description: Audit README.md against generate-readme's fixed template — missing/extra sections, facts with no source, restated SDD depth, stale References — filed as BACKLOG.md stories. Pass --report to skip the write.
argument-hint: [--report]
disable-model-invocation: true
---

# README audit

Findings only, never edits `README.md` — `generate-readme` owns the format and is the only skill that writes it. This locates structural drift, unsourced claims, and stale references, same division of labor `audit-changelog` keeps with `generate-changelog`.

Load `generate-readme` first — every check below tests against rules it owns, not rules restated here.

## Process

1. **Read `README.md` in full.** No file at all is itself a finding — cite `generate-readme`'s Scaffold branch, don't draft one.
2. **Check the fixed shape** — `# <repo-name>` H1, then exactly the five numbered sections in order (Overview, Setup, Usage, Testing, References). A missing or extra section is a finding.
3. **Check every fact traces to a source** — a Setup command not in the actual build tooling, an env var/config key the repo doesn't read, a Usage route/export/job not found in the code, a Testing command not in the Makefile/scripts. An unsourced claim is a finding, not a style note.
4. **Check the Overview status line** — present and pointing at `BACKLOG.md` iff an open Blocker-tier item exists there; present on a clean backlog, or absent with one open, is a finding.
5. **Check §5 References** — lists only files actually present (`docs/sdd.md`, `BACKLOG.md`, `CHANGELOG.md`, an API contract file); never lists `docs/prd.md` — the PRD, when one exists, is a Google Doc in Drive, not a repo file; never lists an individual `docs/adrs/` file — the SDD §14 already links them.
6. **Check for restated depth** — design rationale, infra diagrams, or endpoint-by-endpoint detail that belongs in `docs/sdd.md` and should be a link, not inlined prose.

## Report

| Kind | Where | Finding | Evidence |
|---|---|---|---|
| Unsourced claim | §2 Setup | env var `FOO_TIMEOUT` not read anywhere | grep, no hits |
| Stale reference | §5 References | links `docs/adrs/003-x.md`, file deleted | `docs/adrs/` listing |
| Missing status | §1 Overview | open Blocker in `BACKLOG.md`, no status line | `BACKLOG.md` line |

**Sev**: Critical (a Setup/Usage command or route that doesn't exist — actively misleads a new reader) · High (missing required section, a References entry pointing at a deleted file, wrong Blocker status) · Medium (restated SDD depth, a drifted Testing command) · Low (wording, ordering nits).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <finding> · S → \`/audit-readme\` reports it clear`. Append under the current target state's `Cross-Cutting` domain — full story-line rules live in `generate-backlog`. `--report` prints the table only; nothing is written.
