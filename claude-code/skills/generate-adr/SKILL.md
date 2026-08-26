---
name: generate-adr
description: Numbered Architecture Decision Records under docs/adrs/ — the fit test for when a decision earns its own file, the numbering rule, and the template. Load when creating or editing a file under docs/adrs/.
paths: "**/docs/adrs/**"
---

# Architecture Decision Records

An ADR is the deep record for one design decision. SDD §11 (see `generate-sdd`) already carries a table row for every decision — Chosen/Rejected/Why. An ADR exists only for the row that doesn't fit that row; it is never a replacement for it. A row that cites an ADR still states Chosen/Rejected/Why in the table itself.

## The fit test

Write one only when at least one is true:
- **Contentious** — reasonable engineers would land differently; the losing arguments need to be on record, not just the winner.
- **Expensive to reverse** — a schema, a wire contract, a security boundary, anything that costs a migration to undo.
- **Doesn't fit a table row** — the rationale needs sequencing, a runbook, or edge cases to be complete.

Otherwise the SDD §11 row alone is enough. Most decisions never earn an ADR — treat that as the table doing its job, not as a gap.

## File location and numbering

`docs/adrs/NNN-slug.md` — colocated with `docs/sdd.md`, never a top-level `/adrs`.

- **`NNN`** — 3-digit, zero-padded, sequential across the whole folder regardless of status. Read the existing files, take the highest number, add 1. **Never reused, never renumbered** — a Rejected or Superseded ADR keeps its number forever; the sequence is an audit trail, not a live index.
- **`slug`** — kebab-case, 2-5 words naming the decision, not the ticket (`rs256-signing-key-rotation`, not `auth-changes`).

Every ADR needs a citing row in SDD §11 and a link from SDD §14 References (newest first) — both owned by `generate-sdd`, not restated here.

## Template

```markdown
# ADR NNN — <Decision Title>

- **Status:** <Proposed | Accepted | Rejected | Deprecated | Superseded by ADR-NNN>
- **Date:** <YYYY-MM-DD>
- **Deciders:** <names>
- **Supersedes:** <ADR-NNN — omit the line if none>

## Context
<The forces that made this undecidable by default — the constraint, the cost of the status quo, what's non-negotiable. Enough for a reader with no memory of the discussion to see why a decision was necessary at all.>

## Decision
<The choice, stated as fact. A numbered list of concrete commitments when there's more than one — the record of what was agreed, not a transcript of the debate.>

## Alternatives Considered
<Table: Option | Rejected because>

## Consequences
<What gets easier, what gets harder. Both columns — a decision with no listed downside wasn't actually a decision.>

## References
<The SDD §11 row this backs, related ADRs, external RFCs/docs.>
```

**Optional, add only when it earns its place:** `## Implementation Status` — a table of item → state, for a decision that ships in phases and the decided/shipped gap needs tracking. A decision that ships in one piece omits this section entirely.

## Status lifecycle

- `Proposed` → `Accepted` or `Rejected` once decided.
- `Accepted` → `Deprecated` (no replacement) or `Superseded by ADR-NNN` (there is one). Fix up both files together: the old ADR's Status line points forward, the new one's Supersedes line points back.
- A Rejected or Deprecated ADR is never deleted — it's the reason a since-discarded option doesn't get re-litigated next quarter.
