---
name: generate-prd
description: Fixed section template for the frozen PRD — docs/prd.md. Business scope, requirement IDs, jargon-free language. Load before creating or editing it.
paths: "**/docs/prd.md, **/prd.md"
---

# The PRD

`docs/prd.md` answers *what are we building and why*, for business and technical readers alike — other systems named only by business role ("the login system"), never by repo name, AWS service, or code symbol. If a sentence needs one of those to make sense, it belongs in the SDD instead. Target ~1,900–2,700 words. `standards-docs` states the frozen/entry-point/load-before-touching behavior shared with the SDD and its satellites; this file is the PRD's shape.

**[authoring.md](authoring.md)** carries the writing procedure — the interview order for producing one from nothing, the cross-reference check against the SDD, formatting rules, and what to do with a non-conforming doc. **Load it only when writing or migrating.** Reading the PRD for reference needs nothing beyond this file.

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

**Requirement IDs (§7):** a 2–3 letter prefix per capability group + a number — `CP1`, `CP2`, `DC1`. Assign the prefix once per group and never reuse it for a different group in the same doc. These IDs are the join key to the SDD — `generate-sdd` must cite every one; `authoring.md` carries the check.
