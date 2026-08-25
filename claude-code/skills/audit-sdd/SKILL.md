---
name: audit-sdd
description: Audit docs/sdd.md against generate-sdd's template and its PRD cross-reference contract — missing/N/A sections, undefined jargon, orphaned requirement IDs, drift from current code — filed as BACKLOG.md stories. Pass --report to skip the write.
argument-hint: [--report]
disable-model-invocation: true
---

# SDD audit

Findings only, never edits `docs/sdd.md` — `generate-sdd` owns the format and is the only skill that writes it. This locates structural gaps, cross-reference breaks, and drift between the frozen doc and the code it describes, same division of labor `audit-changelog` keeps with `generate-changelog`.

Load `generate-sdd` and its `authoring.md` first — every check below tests against rules they own, not rules restated here.

## Process

1. **Read `docs/sdd.md` in full.** No file at all is itself a finding — cite `generate-sdd`, don't draft one.
2. **Check strict section order** — §0 through §14 all present, in order. A section with nothing to say must still appear, marked `N/A` with one line saying why; silent omission is a finding. §3 is exempt from the `N/A` rule — its sub-headings are chosen by app type.
3. **Check §0 Glossary coverage** — grep the body for a term used with real technical weight below §0 that no glossary row defines.
4. **Run the cross-reference contract** (`authoring.md`'s own check, applied here as an audit rather than a write-time gate):
   - Collect every requirement ID in `docs/prd.md` §7.
   - Grep this doc's §1 and §11 for each one; any ID with zero hits is an orphan.
   - Any capability in §1/§11 citing no PRD ID at all is undocumented scope — flag it, don't delete it.
5. **Check `NEEDS DECISION` / `BLOCKED ON:` usage** — an unset target in §7 or §8 left blank or filled with a plausible-sounding number instead of one of these two markers is a finding either way.
6. **Check drift against current code** — spot-check §1's component table, §6's testing-tier table, and §5's threat/control table against what the repo actually contains now; a described component, tier, or control that no longer exists (or a real one missing from the table) is drift.
7. **Check §14 References** — every file present in `docs/adrs/` and `docs/runbooks/` is linked; no link points at a file that no longer exists.
8. **Check §13 Open Items** — an item the code shows resolved but the doc still lists open (or vice versa) is a finding; gaps must read as gaps, not as resolved.

## Report

| Kind | Where | Finding | Evidence |
|---|---|---|---|
| Orphaned ID | §1/§11 | PRD requirement `CP3` cited nowhere | `docs/prd.md` §7 row |
| Drift | §6 | testing-tier table names a suite no longer in the repo | `file:line` (absent) |
| Missing section | — | §9 Infrastructure & Delivery omitted, no `N/A` | doc structure |

**Sev**: Critical (Status: Frozen but content contradicts an approved decision, or a component/control described no longer exists and nothing replaced it) · High (missing section, orphaned requirement ID, undefined heavy-use term) · Medium (stale drift between a described tier/control and current code, an unset target left blank instead of `NEEDS DECISION`) · Low (ADR/runbook link drift, ordering nits).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <finding> · S → \`/audit-sdd\` reports it clear`. Append under the current target state's `Cross-Cutting` domain — full story-line rules live in `generate-backlog`. `--report` prints the table only; nothing is written.
