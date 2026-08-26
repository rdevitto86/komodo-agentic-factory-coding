# Authoring an SDD

Read this when *writing* the SDD. `SKILL.md` carries the section list, which is all a reader needs.

## Producing one, PRD optional

**The SDD is elicited directly — it is never blocked on an approved PRD existing.** The SDD is the source of truth for a code repo. A PRD, when the user has one, lives in Google Drive (`standards-prd`) and can inform this doc; when it does, never draft against an unapproved one — pull only its confirmed decisions, same no-invention rule as everywhere else. With no PRD, elicit the technical decisions directly from the user instead.

**Fetch §4, §7, and §10 when a PRD exists** — the scope boundary, requirement priorities, and phase split that this doc has nowhere to hold and would otherwise invent. `standards-prd` carries the fetch rule; that is the whole of the PRD's role here.

**Anything the user did not say is `NEEDS DECISION`.** An empty marker is a working document; a confident invention is a false one that gets built. Work section by section, in order — §1 Architecture and §2 Data Model first, since later sections (§5 Security, §6 Testing) cite decisions made there.

**Ask in batches, not one at a time.** Then draft, then hand it back for approval — the doc is not the agent's to approve.

## Cross-reference contract with the PRD (when one exists)

**Skip this whole section when no PRD backs this repo.** There is nothing to cross-reference against, and requirement IDs are never invented to fill the gap.

Every requirement ID minted in the PRD's §7 must appear at least once in §1 or §11 here. This is the mechanism that keeps the two docs from silently going stale relative to each other — a reader can trace *why* (PRD) to *how* (SDD) for any requirement without either doc drifting unnoticed.

**When creating or substantially editing the SDD against a known PRD, run the check:**
1. Collect every ID in the PRD's §7 (pattern: 2–3 letters + digits, e.g. `CP1`) — load `standards-prd` to read the Drive doc.
2. Grep this doc's §1 and §11 for each one.
3. Any PRD ID with zero hits here is an orphan — flag it to the user; don't silently invent a citation to make the check pass.
4. Any capability described in §1/§11 that cites no PRD ID at all is worth a second look — it may be undocumented scope. Flag it, don't delete it unasked.

This is a reporting check, not an auto-fix — a real orphan might mean the PRD needs a new row, or this doc needs a citation, or the capability was cut and both docs need updating. That's a judgment call for the user, not something to resolve silently.

## Formatting rules

- **`NEEDS DECISION`** — the exact marker for any target or scope point that hasn't actually been decided. Never fabricate a plausible-sounding number and never silently drop the row.
- **`BLOCKED ON: <specific thing>`** — use instead of `NEEDS DECISION` when someone has already started triaging the gap but it can't be resolved yet, e.g. `BLOCKED ON: CI/CD pipeline review`. This distinguishes "nobody has looked at this" from "actively being worked, waiting on X" — a reader of the doc alone should never have to ask which one it is.
- **Tables over prose** wherever the content has rows — components, threats, testing tiers, observability signals, recovery scenarios, decisions, risks. Prose is for the handful of sections that are inherently narrative (§1's lead-in, §3 UI Design).
- **Diagrams are linked images**, not inline ASCII: `![System context](diagrams/system-context.png)`, files under `docs/diagrams/`.
- **ADRs are separate files, not a §11 substitute.** Format, numbering, and the fit test for when a decision earns one live in `generate-adr`. A row that cites an ADR still states Chosen/Rejected/Why in the table — the ADR is the deeper record, not the only one.
- **Strict section order, every section present.** A section with nothing to say still appears, marked `N/A` with one line saying why — never silently omitted, except §3 Interface & Application Details, whose sub-headings are chosen by app type, not by missing information.
- **File location is fixed:** `docs/sdd.md`, lowercase, at the repo root's `docs/` folder — not `SDD.md`, not nested under a subfolder.

## Encountering an existing doc that doesn't conform

Don't rewrite it unasked — that's scope expansion past whatever the user actually asked for. Flag the specific deviation (wrong section order, orphaned cross-reference, jargon with no §0 entry) in one line to the user, same as any other out-of-scope finding, and let them decide whether it's worth fixing now.

**If the user does ask for a migration**, fix in this order — each step is the join key or precondition for the next:
1. When a PRD backs this repo, confirm its §7 has requirement IDs — the cross-reference contract hangs off this key. A PRD missing them is a finding for the people who own it, never something this session fixes. With no PRD, skip to step 2.
2. Section order and missing/`N/A` sections here.
3. §0 Glossary entries for any undefined jargon used with real weight below it.
4. Cross-reference contract — run the orphan check now that IDs and sections are in place.
