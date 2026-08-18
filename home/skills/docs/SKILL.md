---
name: docs
description: Fixed section templates for docs/prd.md and docs/sdd.md — requirement-ID cross-reference, jargon rule, length budget. Load before creating/editing either file, or writing a PRD, SDD, or design doc.
paths: "**/docs/**, **/prd.md, **/sdd.md"
---

# PRD / SDD standard

Every repo's `docs/prd.md` and `docs/sdd.md` follow the same fixed sections, in the same order, so a reader who has read one repo's pair can navigate any other repo's pair cold. This skill exists because three repos already drifted into three different formats — mirroring an existing file is not a substitute for this template, even a good-looking one.

**Load this before touching either file at all** — editing an already-open doc mid-session counts the same as creating one. Loading it partway through means anything read or written before that point wasn't checked against the contract below.

**Both docs must be readable start to finish in 15 minutes.** That's the design constraint behind every rule below: fixed section order (skim without hunting), a length budget (no open-ended elaboration), tables over prose (scannable), and zero unexplained jargon (no re-reading a sentence three times to parse one term).

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

## `docs/prd.md` — fixed sections, in order

```markdown
# PRD — <repo-name>

**Status:** <Draft|Active|Frozen> · **Owner:** <team> · **Scope:** <V1/V2 boundary> · **Used by:** <who calls this>

---

## 1. The Problem
<Table: what's missing today → what it costs. 3-6 rows.>

## 2. The Solution
<One paragraph. The design position — what this project owns vs. what it deliberately doesn't.>

## 3. Goals
- **G1 · <name>.** <one line>
- **G2 · <name>.** <one line>

## 4. Not in Scope
- <Explicit non-goal, one line each. As load-bearing as the goals — say why it's someone else's job.>

## 5. Who This Serves
<Table: role → what they need. One row per distinct stakeholder.>

## 6. Requirements
### 6.1 <Capability group name>
<Table: ID | What it delivers | Priority (Must/Should/Could)>
### 6.2 <Next capability group>
...

## 7. Success Metrics
<Numeric targets where they exist. Anything unset is **NEEDS DECISION** — never a fabricated number.>

## 8. Risks
<Table: risk | plain-language impact | mitigation, or **NEEDS DECISION**.>

## 9. Timeline / Phases
<V1/V2 scope split. No calendar dates unless actually committed — otherwise **NEEDS DECISION**.>

## 10. Glossary
<Any business or legal term a non-technical reader might not already know — GDPR, MoSCoW, whatever this doc used above.>
```

**Requirement IDs (§6):** a 2–3 letter prefix per capability group + a number — `CP1`, `CP2`, `DC1`. Assign the prefix once per group and never reuse it for a different group in the same doc. These IDs are the join key to the SDD — see Cross-reference contract below.

---

## `docs/sdd.md` — fixed sections, in order

```markdown
# SDD — <repo-name>

**Status:** <Draft|Active|Frozen> · **Owner:** <team> · **Stack:** <language/framework/infra>
**Companion:** [`prd.md`](prd.md) — business scope and requirement IDs live there, not duplicated here.

---

## 0. Glossary
<Grouped term tables — "Core building blocks," "Security terms," etc. Term + plain meaning. Every term used below with any real weight must be defined here first.>

## 1. Architecture
<System diagram: `![System context](diagrams/system-context.png)`. Component responsibility table. Cite the PRD requirement ID a component exists to satisfy, e.g. "Control Plane stack (CP1, CP2) — lock table + queue.">

## 2. Data Model
<Entities, storage choice, retention. Plain language — jargon-gated through §0, not assumed.>

## 3. Domain Events / Integrations
<What this system emits or consumes, and who else touches it. Event/message table if applicable.>

## 4. Security & Governance
<Table: threat → control.>

## 5. Operational Requirements
<Performance, scaling, availability targets. Unset targets are **NEEDS DECISION**, matching the PRD convention.>

## 6. Infrastructure & Delivery
<Hosting model, deploy strategy, rollout plan.>

## 7. Design Decisions
<Table: # | Decision | Chosen | Rejected | Why. Embedded here — not split into a separate adr/ directory. One file holds the full 15-minute read.>

## 8. Risks
<Technical risk table — implementation/operational risk, distinct from the PRD's business-risk table in its §8.>

## 9. Open Items
<Known gaps, stated as gaps — never presented as resolved when they aren't.>

## 10. References
<Links: prd.md, API contract, runbooks, diagrams/.>
```

---

## Cross-reference contract

Every requirement ID minted in PRD §6 must appear at least once in SDD §1 or §7. This is the mechanism that keeps the two docs from silently going stale relative to each other — a reader can trace *why* (PRD) to *how* (SDD) for any requirement without either doc drifting unnoticed.

**When creating or substantially editing either doc, run the check:**
1. Collect every ID in PRD §6 (pattern: 2–3 letters + digits, e.g. `CP1`).
2. Grep SDD §1 and §7 for each one.
3. Any PRD ID with zero hits in the SDD is an orphan — flag it to the user; don't silently invent an SDD reference to make the check pass.
4. Any capability described in SDD §1/§7 that cites no PRD ID at all is worth a second look — it may be undocumented scope. Flag it, don't delete it unasked.

This is a reporting check, not an auto-fix — a real orphan might mean the PRD needs a new row, or the SDD needs a citation, or the capability was cut and both docs need updating. That's a judgment call for the user, not something to resolve silently.

## Formatting rules that apply to both docs

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
