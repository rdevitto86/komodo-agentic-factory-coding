---
name: audit-prd
description: Audit the Drive-hosted PRD (fetched via MCP) against its section map — missing sections, malformed requirement IDs, orphaned IDs not cited in the SDD — filed as BACKLOG.md stories. Pass --report to skip the write.
argument-hint: [--report]
disable-model-invocation: true
---

# PRD audit

Findings only, never edits the PRD — it is a Google Doc in Drive, read-only from this toolkit, authored by stakeholders outside it.

Load `standards-specs` first — every check below tests against the section map and requirement-ID scheme it owns, not rules restated here.

## Process

1. **Fetch the PRD via the Google Drive MCP tools.** No PRD is not itself a finding — it's optional; stop here if none exists.
2. **Check the section map** — §4 scope boundary, §7 requirement IDs + Must/Should/Could priority, §9 business risk, §10 V1/V2 phase split all present.
3. **Check §7 requirement-ID formatting** — every ID matches the 2-3-letter-plus-digits pattern (e.g. `CP1`), no duplicates, no gaps that suggest a deleted row left an orphaned reference elsewhere.
4. **Cross-reference against the SDD** — fetch it too, then check every §7 ID appears at least once in SDD §1 or §11. An ID with zero hits there is orphaned.
5. **Check for undocumented scope** — an SDD §1/§11 capability citing no PRD ID at all may be scope the PRD never captured; flag it, don't delete it.

## Report

| Kind | Where | Finding | Evidence |
|---|---|---|---|
| Orphaned ID | PRD §7 | `CP3` cited nowhere in the SDD | SDD §1/§11 (absent) |
| Malformed ID | PRD §7 | `checkout-1` doesn't match the ID pattern | PRD §7 row |
| Missing section | — | §9 business risk omitted | doc structure |

**Sev**: High (missing required section, malformed ID that breaks traceability) · Medium (orphaned ID, undocumented scope) · Low (ordering, wording nits).

No findings (or no PRD at all): state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <finding> · S → \`/audit-prd\` reports it clear`. Append under the current target state's `Cross-Cutting` domain — full story-line rules live in `write-backlog`. `--report` prints the table only; nothing is written.
