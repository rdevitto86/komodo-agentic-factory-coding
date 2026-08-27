---
name: audit-sdd
description: Audit the Drive-hosted SDD (fetched via MCP) against its section map — missing/N/A sections, undefined jargon, orphaned decisions, drift from current code — filed as BACKLOG.md stories. Pass --report to skip the write.
argument-hint: [--report]
disable-model-invocation: true
---

# SDD audit

Findings only, never edits the SDD — it is a Google Doc in Drive, read-only from this toolkit. This locates structural gaps and drift between the frozen doc and the code it describes.

Load `standards-specs` first — every check below tests against the section map and fetch contract it owns, not rules restated here.

## Process

1. **Fetch the SDD via the Google Drive MCP tools.** No SDD found at all is itself a finding — the repo has no technical source of truth — but never draft one; that's out of scope here.
2. **Check strict section order** — §0 through §14 all present, in order. A section with nothing to say must still appear, marked `N/A` with one line saying why; silent omission is a finding. §3 is exempt from the `N/A` rule — its sub-headings are chosen by app type.
3. **Check §0 Glossary coverage** — grep the fetched body for a term used with real technical weight below §0 that no glossary row defines.
4. **Check `NEEDS DECISION` / `BLOCKED ON:` usage** — an unset target in §7 or §8 left blank or filled with a plausible-sounding number instead of one of these two markers is a finding either way.
5. **Check drift against current code** — spot-check §1's component table, §6's testing-tier table, and §5's threat/control table against what the repo actually contains now; a described component, tier, or control that no longer exists (or a real one missing from the table) is drift.
6. **Check §11 Decisions** — each entry still numbered sequentially, never renumbered; a Rejected or Superseded entry still present, not deleted.
7. **Check §14 References** — every file present in `docs/runbooks/` is linked; no link points at a file that no longer exists.
8. **Check §13 Open Items** — an item the code shows resolved but the doc still lists open (or vice versa) is a finding; gaps must read as gaps, not as resolved.

## Report

| Kind | Where | Finding | Evidence |
|---|---|---|---|
| Drift | §6 | testing-tier table names a suite no longer in the repo | `file:line` (absent) |
| Missing section | — | §9 Infrastructure & Delivery omitted, no `N/A` | doc structure |

**Sev**: Critical (Status: Frozen but content contradicts an approved decision, or a component/control described no longer exists and nothing replaced it) · High (missing section, undefined heavy-use term) · Medium (stale drift between a described tier/control and current code, an unset target left blank instead of `NEEDS DECISION`) · Low (runbook link drift, ordering nits).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <finding> · S → \`/audit-sdd\` reports it clear`. Append under the current target state's `Cross-Cutting` domain — full story-line rules live in `write-backlog`. `--report` prints the table only; nothing is written.
