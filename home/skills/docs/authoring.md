# Authoring a PRD or SDD

Read this when *writing* either spec. `SKILL.md` carries the section list and the slice contract, which is all a reader needs.

## The split

| | PRD | SDD |
|---|---|---|
| Answers | *What are we building and why?* | *How does it actually work?* |
| Written by | Business architects, PMs, non-technical leadership | Domain architects, tech leads |
| Read by | Business and technical readers | Both — never assume the reader is an engineer |
| Contains | Business tables + plain language; other systems named only by business role ("the login system"), never by repo name, AWS service, or code symbol | Full technical detail, but every term is either replaced with plain language or defined in §0 before its first heavy use |
| Length | ~1,800–2,500 words | ~2,500–3,500 words |

If a sentence in the PRD needs a repo name, a library, or an AWS service to make sense, it belongs in the SDD instead.

---

## Producing a PRD from nothing

**A PRD is elicited, never invented.** The whole value of the human gate is that nothing downstream rests on a guess, so a fabricated requirement is worse than an empty section.

Interview in this order. Each answer is the precondition for the next question being askable:

1. **What is missing today, and what does that cost?** → §1. Until this is concrete the rest has no anchor.
2. **What will this own, and what will it deliberately not own?** → §2 and §4. Ask for the non-goals explicitly; people volunteer goals and hide boundaries.
3. **Who calls it, and what do they need from it?** → §5.
4. **What must it do?** → §6, grouped into capabilities before IDs are assigned.
5. **How will we know it worked?** → §7. A target nobody has set is `NEEDS DECISION`, never a plausible-sounding number.
6. **What could go wrong?** → §8.
7. **What is V1 versus V2?** → §9.

**Ask in batches, not one at a time.** Then draft, then hand it back for approval — the doc is not the agent's to approve.

**Anything the user did not say is `NEEDS DECISION`.** An empty marker is a working document; a confident invention is a false one that gets built.

**The SDD follows the same rule**, sourced from the approved PRD plus the technical decisions the user makes. Never draft an SDD against an unapproved PRD.

---

## Cross-reference contract

Every requirement ID minted in PRD §6 must appear at least once in SDD §1 or §7. This is the mechanism that keeps the two docs from silently going stale relative to each other — a reader can trace *why* (PRD) to *how* (SDD) for any requirement without either doc drifting unnoticed.

**When creating or substantially editing either doc, run the check:**
1. Collect every ID in PRD §6 (pattern: 2–3 letters + digits, e.g. `CP1`).
2. Grep SDD §1 and §7 for each one.
3. Any PRD ID with zero hits in the SDD is an orphan — flag it to the user; don't silently invent an SDD reference to make the check pass.
4. Any capability described in SDD §1/§7 that cites no PRD ID at all is worth a second look — it may be undocumented scope. Flag it, don't delete it unasked.

This is a reporting check, not an auto-fix — a real orphan might mean the PRD needs a new row, or the SDD needs a citation, or the capability was cut and both docs need updating. That's a judgment call for the user, not something to resolve silently.

## Formatting rules that apply to the PRD and SDD

- **`NEEDS DECISION`** — the exact marker for any target, metric, or scope point that hasn't actually been decided. Never fabricate a plausible-sounding number (a "99.9% uptime target" nobody set) and never silently drop the row. Resolving one: replace the marker with the real value and append `(decided <YYYY-MM-DD>)` — that date is the only provenance the PRD carries, since it has no decisions table like SDD §7. Example — before: `**NEEDS DECISION**` in §7 Success Metrics for throughput; after: `10 RPS sustained (decided 2026-08-15)`.
- **`BLOCKED ON: <specific thing>`** — use instead of `NEEDS DECISION` when someone has already started triaging the gap but it can't be resolved yet, e.g. `BLOCKED ON: CI/CD pipeline review`. This distinguishes "nobody has looked at this" (`NEEDS DECISION`) from "actively being worked, waiting on X" (`BLOCKED ON`) — a reader of the doc alone should never have to ask which one it is. Once unblocked, resolve it the same way as `NEEDS DECISION` above.
- **Tables over prose** wherever the content has rows — requirements, risks, roles, threats, decisions. Prose is for the handful of sections that are inherently narrative (§2 The Solution, §1 Architecture's lead-in).
- **Diagrams are linked images**, not inline ASCII: `![System context](diagrams/system-context.png)`, files under `docs/diagrams/`.
- **Strict section order, every section present.** A section with nothing to say still appears, marked `N/A` with one line saying why — never silently omitted. This is what makes the "read one repo's pair, navigate any repo's pair" property hold.
- **File location is fixed:** `docs/prd.md` and `docs/sdd.md`, lowercase, at the repo root's `docs/` folder — not `PRD.md`, not nested under a subfolder.

## Encountering an existing doc that doesn't conform

Don't rewrite it unasked — that's scope expansion past whatever the user actually asked for. Flag the specific deviation (wrong section order, missing requirement IDs, jargon with no glossary entry, orphaned cross-reference) in one line to the user, same as any other out-of-scope finding, and let them decide whether it's worth fixing now.

**If the user does ask for a migration**, fix in this order — each step is the join key or precondition for the next:
1. Requirement IDs in PRD §6 — the SDD cross-reference contract hangs off this key, so nothing downstream can be checked until it exists.
2. Section order and missing/`N/A` sections in both docs.
3. Glossary entries for any undefined jargon.
4. Cross-reference contract — run the orphan check now that IDs and sections are in place.
5. Implementation slices in SDD §10 — last, because a slice cites requirement IDs and depends on the architecture being settled.
