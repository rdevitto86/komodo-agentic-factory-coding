---
name: adr
description: Create, edit, and audit docs/adr/ — one Nygard-style file per architecture decision. Authoring mints the next sequential number and never edits or renumbers a file once accepted; a reversal is a new ADR that supersedes the old one. Auditing checks numbering, Status values, Superseded cross-references, and that every file is linked from the SDD's §14 References, filing findings to BACKLOG.md unless --report is passed.
argument-hint: [audit [--report]]
paths: "**/docs/adr/**"
---

# ADR — docs/adr/

`standards-specs` does not name an ADR contract — it owns the SDD/PRD section maps only. This skill owns the ADR shape itself: one file per decision under `docs/adr/`, `docs/adr/*.md` read and written with the ordinary Read/Edit/Write tools, no MCP tool and no Drive round-trip.

**Two modes, one directory.** No `audit` token in `$ARGUMENTS` → Part 1, authoring. `audit` (optionally with `--report`) → Part 2, findings only.

---

# Part 1 — Authoring (create + edit)

## One file per decision, numbered sequentially

Unlike the SDD/PRD's single file, an ADR is one file per decision: `docs/adr/NNNN-slug.md`, zero-padded four digits, starting at `0001`. A new ADR gets the next sequential number after the highest one present in `docs/adr/` — never a reused number, never a gap left on purpose.

## Starting from the template

Copy `templates/project/docs/adr/template.md` verbatim as the starting point for a new ADR, filling in the number, title, date, deciders, and the three body sections. Never invent a different heading set.

- **Status** is one of `Proposed`, `Accepted`, `Rejected`, `Superseded`.
- **Context** states the forces — technical, business, organizational — that make the decision necessary.
- **Decision** states the change as a single clear sentence.
- **Consequences** states what becomes easier or harder, and names what was rejected and why.

## Immutable once accepted — a reversal is a new file

**Never edit or renumber an ADR once it reaches `Accepted`.** A decision that gets reversed is recorded as a new ADR with the next sequential number, its own Context/Decision/Consequences, and a line in its Consequences naming the ADR it supersedes. The superseded file's `Status` is then changed to `Superseded` — the only edit an accepted ADR ever receives — with a line naming which ADR supersedes it. A `Proposed` or `Rejected` ADR that never shipped may still be edited freely; the immutability rule starts at `Accepted`.

## Cross-reference from the SDD

The SDD's §11 Decisions cites which calls earned a full ADR write-up (see `sdd/SKILL.md`'s §11 section) and its §14 References links out to `docs/adr/`. When a new ADR is created, add or update the link in `docs/spec/SDD.md`'s §14 so the file isn't orphaned — this skill edits `docs/adr/`, not the SDD itself; that link-back is a small courtesy edit, not a scope expansion into SDD authoring.

## Keeping it current

Update `docs/adr/` alongside the decision it records — a new ADR the moment the call is made, a `Superseded` status change the moment a later ADR reverses it. This is a repo file tree edited like any other tracked doc; there is no separate approval gate beyond the normal review the diff already gets.

---

# Part 2 — Audit

Findings only — this mode locates gaps and drift, it never fixes them. Run Part 1 to act on what it finds.

## Process

1. **List every file in `docs/adr/`**, excluding `template.md`. No ADRs at all is not itself a finding — a repo may have made no contentious, expensive-to-reverse calls yet.
2. **Check sequential numbering** — the numbers present form an unbroken run starting at `0001`, no gaps, no reused number, no duplicate.
3. **Check `Status` values** — each file's Status is one of `Proposed`, `Accepted`, `Rejected`, `Superseded`; anything else is a finding.
4. **Check `Superseded` cross-references** — a file with `Status: Superseded` names which ADR supersedes it; a file that claims to supersede another names that ADR's number and title correctly.
5. **Check §14 References** — read `docs/spec/SDD.md`, then check every file present in `docs/adr/` (excluding `template.md`) is linked from its §14 References; a file with zero hits there is orphaned.

## Report

| Kind | Where | Finding | Evidence |
|---|---|---|---|
| Numbering gap | `docs/adr/` | `0003` missing between `0002` and `0004` | directory listing |
| Bad Status | `docs/adr/0002-*.md` | Status reads `Done`, not one of the four allowed values | file header |
| Dangling Superseded | `docs/adr/0004-*.md` | Status `Superseded`, no ADR named as the successor | file body |
| Orphaned ADR | `docs/adr/0005-*.md` | not linked from SDD §14 References | SDD §14 (absent) |

**Sev**: High (numbering gap or reuse, invalid Status value) · Medium (dangling Superseded reference, orphaned ADR) · Low (ordering, wording nits).

No findings (or no ADRs at all): state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <finding> · S → \`/adr audit\` reports it clear`. Append under the current target state's `Cross-Cutting` domain — full story-line rules live in `backlog`. `--report` prints the table only; nothing is written.
