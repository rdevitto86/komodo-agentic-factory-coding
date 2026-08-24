---
name: docs
description: Fixed section templates for the frozen specs — docs/prd.md and docs/sdd.md. Requirement-ID cross-reference, jargon rule, length budget. Load before creating or editing either.
paths: "**/docs/**, **/prd.md, **/sdd.md"
---

# The frozen specs

Every repo's `docs/prd.md` and `docs/sdd.md` follow the same fixed sections in the same order, so a reader who knows one repo can navigate any other cold. This skill exists because three repos already drifted into three different formats — mirroring an existing file is not a substitute for this template, even a good-looking one.

**Both are source of truth, not living docs.** Once approved they are frozen; changing either is a human decision, never a step inside a build. **The SDD is purely technical writing** — it describes how the system works. Breaking that description into buildable work happens in `BACKLOG.md`, whose format lives in `worklog`; the SDD never carries a work queue.

**Load this before touching either file** — editing an already-open doc mid-session counts the same as creating one.

**Each must be readable start to finish in 15 minutes.** Fixed section order, a length budget, tables over prose, zero unexplained jargon.

**[authoring.md](authoring.md)** carries the writing procedure — the PRD/SDD split, the interview order for producing one from nothing, the cross-reference check, formatting rules, and what to do with a non-conforming doc. **Load it only when writing or migrating a spec.** Reading either doc for reference needs nothing beyond this file.

## `docs/prd.md` — fixed sections, in order

```markdown
# PRD — <repo-name>

**Status:** <Draft|Active|Frozen> · **Owner:** <team> · **Scope:** <V1/V2 boundary> · **Used by:** <who calls this>

---

**Executive Summary**
<2-3 sentences. The whole doc's BLUF — what's being built, why, and for whom. Written last, after §1-2 are settled.>

## 1. The Problem
<Table: what's missing today → what it costs. 3-6 rows.>

## 2. The Solution
<One paragraph. The design position — what this project owns vs. what it deliberately doesn't.>

## 3. Goals
- **G1 · <name>.** <one line>
- **G2 · <name>.** <one line>

## 4. Not in Scope
- <Explicit non-goal, one line each. As load-bearing as the goals — say why it's someone else's job.>

## 5. Assumptions & Dependencies
<Table: assumption or dependency → whose work it relies on → what breaks if it doesn't hold. Distinct from §9 Risks — this is what the plan takes as given, not what could go wrong with it.>

## 6. Who This Serves
<Table: role → what they need. One row per distinct stakeholder.>

## 7. Requirements
### 7.1 <Capability group name>
<Table: ID | What it delivers | Priority (Must/Should/Could)>
### 7.2 <Next capability group>
...

## 8. Success Metrics
<Numeric targets where they exist. Anything unset is **NEEDS DECISION** — never a fabricated number.>

## 9. Risks
<Table: risk | plain-language impact | mitigation, or **NEEDS DECISION**.>

## 10. Timeline / Phases
<V1/V2 scope split. No calendar dates unless actually committed — otherwise **NEEDS DECISION**.>

## 11. Glossary
<Any business or legal term a non-technical reader might not already know — GDPR, MoSCoW, whatever this doc used above.>
```

**Requirement IDs (§7):** a 2–3 letter prefix per capability group + a number — `CP1`, `CP2`, `DC1`. Assign the prefix once per group and never reuse it for a different group in the same doc. These IDs are the join key to the SDD — `authoring.md` carries the cross-reference check.

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

## 3. Interface & Application Details
<This app's own contract surface — not what it calls out to, that's §4. Pick the sub-heading(s) below that match what this repo actually is; omit the rest. This is the one section exempt from the doc-wide N/A rule — its shape is decided by app type, not by missing information. Add a sub-heading type not listed here if none fits.>

### API Contract
<Request/response shape or a link to the OpenAPI/proto/GraphQL file, versioning policy, auth model.>

### UI Design
<Key flows and states, accessibility target, link to the design file. Prose, not a table — inherently narrative.>

### Job / Batch Contract
<Trigger (cron, queue, manual), idempotency guarantee, failure/retry handling.>

## 4. Domain Events / Integrations
<What this system emits or consumes, and who else touches it. Event/message table if applicable.>

## 5. Security & Governance
<Table: threat → control.>

## 6. Testing Strategy
<Table: tier (unit/integration/contract/e2e/perf) → what it covers → coverage floor. Cite the language skill's tier definitions rather than restating them. Name the environments each tier runs against.>

## 7. Observability
<Table: signal (logs/metrics/traces/alerts) → what's captured → destination → who's paged. Unset alerting is **NEEDS DECISION**, matching the PRD convention.>

## 8. Operational Requirements
<Performance, scaling, availability targets. Unset targets are **NEEDS DECISION**, matching the PRD convention.>

## 9. Infrastructure & Delivery
<Hosting model, deploy strategy, rollout plan.>

## 10. Recovery
<Table: failure scenario → recovery mechanism → RTO/RPO. Backup/restore for §2's stores, rollback trigger and procedure for §9's delivery pipeline.>

## 11. Design Decisions
<Table: # | Decision | Chosen | Rejected | Why | ADR. Kept here so the full doc is still a 15-minute read — the ADR column links to `docs/adrs/NNN-slug.md` only for the decisions that earned a full write-up; most rows need no ADR at all. Fit test, numbering, and template: `adr` skill.>

## 12. Risks
<Technical risk table — implementation/operational risk, distinct from the PRD's business-risk table in its §8.>

## 13. Open Items
<Known gaps, stated as gaps — never presented as resolved when they aren't.>

## 14. References
<Links: prd.md, API contract, runbooks, diagrams/. Architecture Decision Records: link every file present in `docs/adrs/`, newest first. No directory means no ADRs yet — omit the line, don't create an empty one.>
```

