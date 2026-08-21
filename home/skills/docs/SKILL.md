---
name: docs
description: Fixed section templates for the frozen specs — docs/prd.md and docs/sdd.md. Requirement-ID cross-reference, slice contract, jargon rule, length budget. Load before creating or editing either.
paths: "**/docs/**, **/prd.md, **/sdd.md"
---

# The frozen specs

Every repo's `docs/prd.md` and `docs/sdd.md` follow the same fixed sections in the same order, so a reader who knows one repo can navigate any other cold. This skill exists because three repos already drifted into three different formats — mirroring an existing file is not a substitute for this template, even a good-looking one.

**Both are source of truth, not living docs.** Once approved they are frozen; changing either is a human decision, never a step inside a build. **Slice status therefore never writes back into SDD §10** — the SDD *defines* slices, and tracking which are open or done belongs to `BACKLOG.md` and `CHANGELOG.md`, whose formats live in `worklog`.

**Load this before touching either file** — editing an already-open doc mid-session counts the same as creating one.

**Each must be readable start to finish in 15 minutes.** Fixed section order, a length budget, tables over prose, zero unexplained jargon.

**[authoring.md](authoring.md)** carries the writing procedure — the PRD/SDD split, the interview order for producing one from nothing, the cross-reference check, formatting rules, and what to do with a non-conforming doc. **Load it only when writing or migrating a spec.** Reading §10 to plan work needs nothing beyond this file.

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

**Requirement IDs (§6):** a 2–3 letter prefix per capability group + a number — `CP1`, `CP2`, `DC1`. Assign the prefix once per group and never reuse it for a different group in the same doc. These IDs are the join key to the SDD — `authoring.md` carries the cross-reference check.

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

## 10. Implementation Slices
<Table: ID | Delivers | Satisfies | Depends on | Done when. One row per independently buildable unit. See the slice contract below.>

## 11. References
<Links: prd.md, API contract, runbooks, diagrams/.>
```

---

## Slice contract — SDD §10

**A slice is a unit of work that can be built without any other slice in flight.** This section is what turns a design into buildable work; an SDD without it produces a single linear queue no matter how good the architecture is.

| ID | Delivers | Satisfies | Depends on | Done when |
|---|---|---|---|---|
| `S1` | Session store schema + migration | `AU1` | — | `make verify` green |
| `S2` | Token refresh endpoint | `AU2`, `AU3` | `S1` | `go test ./auth/...` green |
| `S3` | Rate-limit middleware | `AU4` | — | `go test ./mw/...` green |

- **`Done when` is a command, not a description.** "Handles refresh correctly" is not a slice boundary; a command with an exit code is. This column is what lets an agent close its own loop and what `.claude/verify.sh` runs.
- **`Depends on` is the parallelism map.** Slices sharing no dependency edge run at the same time — above, `S1` and `S3` start together and `S2` waits. Nothing else in either doc encodes this.
- **Two slices that edit the same file are one slice.** Splitting them creates a write collision, not parallelism.
- **`Satisfies` cites PRD §6 requirement IDs**, same join key as §1 and §7.
- **A slice needing more than one sitting is too big.** Split it until each row is independently verifiable.

**Sizing rule:** if every slice depends on the one above it, the decomposition failed — go back to §1 and find the seams. A chain of 9 dependent slices is one slice with extra rows.

