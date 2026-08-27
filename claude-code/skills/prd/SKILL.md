---
name: prd
description: Create, edit, and audit docs/spec/PRD.md — business scope, requirement priorities, and the requirement-ID scheme. Authoring keeps the fixed section map and the append-only §7 ID scheme current as scope changes; auditing checks structure, requirement-ID formatting, and cross-reference against the SDD, filing findings to BACKLOG.md unless --report is passed.
argument-hint: [audit [--report]]
paths: "**/docs/spec/PRD.md"
---

# PRD — docs/spec/PRD.md

Load `standards-specs` first — it owns the section map and the read contract; this skill authors and audits the file against that map, it never restates the map beyond what's needed to apply it. `docs/spec/PRD.md` is a plain repo file — read, edited, and written with the ordinary Read/Edit/Write tools, no MCP tool and no Drive round-trip.

**Two modes, one file.** No `audit` token in `$ARGUMENTS` → Part 1, authoring. `audit` (optionally with `--report`) → Part 2, findings only.

---

# Part 1 — Authoring (create + edit)

## Starting from the template

If `docs/spec/PRD.md` doesn't exist yet, copy `templates/project/docs/spec/PRD.md` verbatim as the starting point — the same stub `write-repo`'s Create step already writes for a new repo. Never invent a different heading set. **The PRD is optional** — a repo with no PRD simply has no requirement IDs to cite anywhere; that is not a gap to fill on its own.

## Section map

Follow `standards-specs`' section map exactly: §4 Scope boundary, §7 Requirement IDs, §9 Business risk, §10 Phase split. The numbering skips on purpose — §4/§7/§9/§10 are the sections `standards-specs` names; never fill the gaps by inventing content for the numbers in between.

- **§4 Scope boundary** states what is in scope for the repo, and what is explicitly out.
- **§7 Requirement IDs** is a table — `ID | Requirement | Priority`, priority one of Must/Should/Could. **IDs are minted only here**, never invented by a skill or a reader elsewhere; a new requirement gets the next ID in sequence, never a reused or renumbered one.
- **§9 Business risk** states the risk tied to not shipping, or shipping late or wrong.
- **§10 Phase split** states what ships in V1 versus what is deferred to V2.

## Keeping it current

Update the file directly, alongside the change that makes a described section stale — scope narrowed or widened, a requirement added or reprioritized, a phase moved. Bump the `Status` / `Owner` / `Last updated` header at the top whenever content changes. This is a repo file edited like any other tracked doc; there is no separate approval gate beyond the normal review the diff already gets. **§7 and §10 also feed the SDD's §7 Observability and §10 Recovery sections** when a PRD backs the repo — keep the two in sync rather than letting the SDD restate a stale priority or phase.

---

# Part 2 — Audit

Findings only — this mode locates gaps and drift, it never fixes them. Run Part 1 to act on what it finds.

## Process

1. **Read `docs/spec/PRD.md` from the repo.** No PRD is not itself a finding — it's optional; stop here if none exists.
2. **Check the section map** — §4 Scope boundary, §7 Requirement IDs + Must/Should/Could priority, §9 Business risk, §10 Phase split all present.
3. **Check §7 requirement-ID formatting** — every ID matches the pattern the template's example row establishes, no duplicates, no gaps that suggest a deleted row left an orphaned reference elsewhere.
4. **Cross-reference against the SDD** — read `docs/spec/SDD.md`, then check every §7 ID appears at least once in SDD §1 or §11. An ID with zero hits there is orphaned.
5. **Check for undocumented scope** — an SDD §1/§11 capability citing no PRD ID at all may be scope the PRD never captured; flag it, don't delete it.

## Report

| Kind | Where | Finding | Evidence |
|---|---|---|---|
| Orphaned ID | PRD §7 | `CP3` cited nowhere in the SDD | SDD §1/§11 (absent) |
| Malformed ID | PRD §7 | `checkout-1` doesn't match the ID pattern | PRD §7 row |
| Missing section | — | §9 Business risk omitted | doc structure |

**Sev**: High (missing required section, malformed ID that breaks traceability) · Medium (orphaned ID, undocumented scope) · Low (ordering, wording nits).

No findings (or no PRD at all): state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <finding> · S → \`/prd audit\` reports it clear`. Append under the current target state's `Cross-Cutting` domain — full story-line rules live in `write-backlog`. `--report` prints the table only; nothing is written.
