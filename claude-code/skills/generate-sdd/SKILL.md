---
name: generate-sdd
description: Fixed section template for the frozen SDD — docs/sdd.md. Architecture, data model, security, testing, observability, recovery. Load before creating or editing it.
paths: "**/docs/sdd.md, **/sdd.md"
---

# The SDD

`docs/sdd.md` answers *how does it actually work* — full technical detail, but every term is either replaced with plain language or defined in §0 before its first heavy use; never assume the reader is an engineer. Target ~2,800–3,800 words. `standards-docs` states the frozen/entry-point/load-before-touching behavior shared with the PRD and its satellites; this file is the SDD's shape.

**Purely technical writing** — it describes how the system works. Breaking that description into buildable work happens in `BACKLOG.md`, whose format lives in `generate-backlog`; the SDD never carries a work queue.

**[authoring.md](authoring.md)** carries the writing procedure — deriving one from an approved PRD, the cross-reference check, formatting rules, and what to do with a non-conforming doc. **Load it only when writing or migrating.** Reading the SDD for reference needs nothing beyond this file.

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
<Table: # | Decision | Chosen | Rejected | Why | ADR. Kept here so the full doc is still a 15-minute read — the ADR column links to `docs/adrs/NNN-slug.md` only for the decisions that earned a full write-up; most rows need no ADR at all. Fit test, numbering, and template: `generate-adr` skill.>

## 12. Risks
<Technical risk table — implementation/operational risk, distinct from the PRD's business-risk table in its §8.>

## 13. Open Items
<Known gaps, stated as gaps — never presented as resolved when they aren't.>

## 14. References
<Links: README.md, prd.md, API contract, diagrams/. Architecture Decision Records: link every file present in `docs/adrs/`, newest first — `generate-adr` owns the template. Runbooks: link every file present in `docs/runbooks/` — `generate-runbook` owns the template. Either directory missing means none exist yet — omit the line, don't create an empty one.>
```

**Requirement IDs:** every ID minted in the PRD's §7 must appear at least once in §1 or §11 here — the join key back to *why*. `generate-prd` owns minting them; `authoring.md` carries the check.
