---
name: sdd
description: Create, edit, and audit docs/spec/SDD.md — the technical source of truth. Authoring keeps the fixed section map, the N/A rule, and the append-only §11 Decisions log current as work lands; auditing checks structure, glossary coverage, NEEDS DECISION usage, and drift against current code, filing findings to BACKLOG.md unless --report is passed.
argument-hint: [audit [--report]]
paths: "**/docs/spec/SDD.md"
---

# SDD — docs/spec/SDD.md

Load `standards-specs` first — it owns the section map and the read contract; this skill authors and audits the file against that map, it never restates the map beyond what's needed to apply it. `docs/spec/SDD.md` is a plain repo file — read, edited, and written with the ordinary Read/Edit/Write tools, no MCP tool and no Drive round-trip.

**Two modes, one file.** No `audit` token in `$ARGUMENTS` → Part 1, authoring. `audit` (optionally with `--report`) → Part 2, findings only.

---

# Part 1 — Authoring (create + edit)

## Starting from the template

If `docs/spec/SDD.md` doesn't exist yet, copy `templates/project/docs/spec/SDD.md` verbatim as the starting point — the same stub `git-repo-init`'s Create step already writes for a new repo. Never invent a different heading set.

## Section map

Follow `standards-specs`' section map exactly: §0 Glossary, §1 Components, §3 app-type-specific detail, §5 Threats/Controls, §6 Testing tiers, §7 Observability, §9 Infrastructure & Delivery, §10 Recovery, §11 Decisions, §13 Open Items, §14 References. The numbering skips on purpose — §2/§4/§8/§12 are sections `standards-specs` has not yet named; never fill the gaps by inventing content for them.

- **§3's heading is renamed** to whatever the app type actually needs — API contract, UI flows, infra topology, firmware boundary — per the template's own placeholder comment.
- **A section with genuinely nothing to say stays present**, marked `N/A` with one line saying why — never deleted. §3 is exempt from the `N/A` rule since its sub-headings are chosen by app type.
- **§0 Glossary must define every term** used below it with real technical weight — add a row before using a new one, not after.
- **§7 and §10 also carry the PRD's requirement priorities / V1–V2 phase split** when a PRD backs the repo (`docs/spec/PRD.md` — see `standards-specs`).

## §11 Decisions — append-only

A new entry gets the next sequential number, written as a dated sub-section when the call is contentious, expensive to reverse, or has real sequencing worth recording. **Never edit or renumber a prior entry, including a superseded one** — a reversal is a new entry with its own number that says what it supersedes.

## NEEDS DECISION / BLOCKED ON

An unset target inside §7 or §8's content is marked `NEEDS DECISION` or `BLOCKED ON: <reason>` — never left blank, and never filled with a plausible-sounding placeholder instead of one of those two markers.

## Keeping it current

Update the file directly, alongside the code change that makes a described section stale — a component added or removed, a control retired, a decision made. Bump the `Status` / `Owner` / `Last updated` header at the top whenever content changes. This is a repo file edited like any other tracked doc; there is no separate approval gate beyond the normal review the diff already gets.

---

# Part 2 — Audit

Findings only — this mode locates gaps and drift, it never fixes them. Run Part 1 to act on what it finds.

## Process

1. **Read `docs/spec/SDD.md` from the repo.** No file at all is itself a finding — the repo has no technical source of truth — but never draft one here; that's Part 1's job, on request, not automatic.
2. **Check strict section order** — §0 through §14 all present, in order. A section with nothing to say must still appear, marked `N/A` with one line saying why; silent omission is a finding. §3 is exempt from the `N/A` rule — its sub-headings are chosen by app type.
3. **Check §0 Glossary coverage** — grep the file for a term used with real technical weight below §0 that no glossary row defines.
4. **Check `NEEDS DECISION` / `BLOCKED ON:` usage** — an unset target in §7 or §8 left blank or filled with a plausible-sounding value instead of one of these two markers is a finding either way.
5. **Check drift against current code** — spot-check §1's component table, §6's testing-tier table, and §5's threat/control table against what the repo actually contains now; a described component, tier, or control that no longer exists (or a real one missing from the table) is drift.
6. **Check §11 Decisions** — each entry still numbered sequentially, never renumbered; a Rejected or Superseded entry still present, not deleted.
7. **Check §14 References** — every file present in `docs/runbook/` is linked; no link points at a file that no longer exists.
8. **Check §13 Open Items** — an item the code shows resolved but the doc still lists open (or vice versa) is a finding; gaps must read as gaps, not as resolved.

## Report

| Kind | Where | Finding | Evidence |
|---|---|---|---|
| Drift | §6 | testing-tier table names a suite no longer in the repo | `file:line` (absent) |
| Missing section | — | §9 Infrastructure & Delivery omitted, no `N/A` | doc structure |

**Sev**: Critical (Status: Frozen but content contradicts an approved decision, or a component/control described no longer exists and nothing replaced it) · High (missing section, undefined heavy-use term) · Medium (stale drift between a described tier/control and current code, an unset target left blank instead of `NEEDS DECISION`) · Low (runbook link drift, ordering nits).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <finding> · S → \`/sdd audit\` reports it clear`. Append under the current target state's `Cross-Cutting` domain — full story-line rules live in `backlog-modify`. `--report` prints the table only; nothing is written.
