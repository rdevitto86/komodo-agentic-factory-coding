---
name: plan
description: Turn a goal into a phased epic and story breakdown, then write the approved result into TODO.md.
argument-hint: [what you want to build]
disable-model-invocation: true
---

# Plan

Engineering and product planning in one pass. **$ARGUMENTS**

You are planning, not building. Write no implementation code during this skill.

## Step 1 — Ask before assuming

**Ask your questions first, in one batch, before producing any plan.** A plan built on a guess wastes more time than a question costs.

Ask only what changes the plan. Skip anything you can determine by reading the repo — read it instead.

Typical unknowns worth asking:

- **Scope boundary** — what is explicitly *not* in V1?
- **Existing surface** — is this extending something already built, or greenfield?
- **Hard constraints** — a deadline, a dependency that must land first, a decision already made.
- **Done condition** — what has to be true for this to ship?

If the repo answers a question, do not ask it. If nothing is genuinely unclear, say so and move on.

## Step 2 — Read the ground truth

Before proposing anything:

- **Read the existing `TODO.md`.** Never duplicate a story already in it.
- **Read the code that this work touches.** The current state beats any ledger.
- **Read the project `AGENTS.md`** for stack and conventions.

State plainly if the repo contradicts what the user described. That contradiction is usually the most valuable output of the whole exercise.

## Step 3 — Name the target states

Two states, no more:

- **V1** — the smallest thing that is genuinely usable. Not a prototype, not feature-complete.
- **V2** — the next coherent increment.

Anything beyond V2 is speculation and does not belong in the plan.

## Step 4 — Decompose

**Groups are `Cross-Cutting` or a feature/subfolder name. Phases inside a group are small, sprint-sized slices of that group, numbered `1, 2, 3, ...`.**

- **Cap each run at 3 groups and 4 phases per group.** If the work genuinely exceeds that, plan V1 only and say V2 needs its own pass. An unbounded dump is what made the old ledger useless.
- **A phase is one shippable slice** — mergeable on its own, with a visible result, scoped like a sprint or epic.
- **Phases build in sequence by default.** Mark `(parallel with Phase N)` only where a phase genuinely has no dependency on another in the same group.
- **Every phase that adds behavior gets a `Tests: unit + component coverage for <slice>` item.** No phase ships without it. A phase that changes a published contract gets a contract-test item too — those three tiers are the merge gate.
- **Integration, smoke, live-dependency, and perf tests never ride inside a feature phase.** They land in their own later phase in the same group, after that group's gate-tested phases, because they run after the merge rather than before it.
- **Every item carries a severity tag** — `[C]` critical, `[H]` high, `[M]` medium, `[L]` low.
- **Size relatively**: S, M, L. Never hours. **Anything XL gets broken down before it enters the plan.**
- **Every item names its acceptance condition** in one line.

## Step 5 — Present, then stop

Show the plan and **wait for approval**. Do not write to `TODO.md` yet.

```markdown
## 🎯 Target states
- **V1** — <one line>
- **V2** — <one line>

## 📋 Plan
### Cross-Cutting
#### Phase 1 · <slice>
| Item | What | Severity | Size | Done when |
|---|---|---|---|---|
| ... | ... | H | M | ... |
| Tests | unit + component coverage for <slice> | M | S | ... |

### <Feature/subfolder>
#### Phase 1 · <slice>
| Item | What | Severity | Size | Done when |
|---|---|---|---|---|
| ... | ... | C | M | ... |

#### Phase 2 · Post-merge validation (integration/smoke/e2e/perf)
| Item | What | Severity | Size | Done when |
|---|---|---|---|---|
| ... | ... | M | S | ... |

## ⚠️ Risks
- **<thing>** — why it could bite

## ❓ Open
- <anything still unresolved>
```

## Step 6 — Write on approval only

Once the user approves, merge into `TODO.md` in the format defined by the `todo` skill — open work only, no completed section, no dates in headings.

- **Append under the right target state and group**, preserving existing phase numbering.
- **Never renumber existing phases.**
- **Never remove an item you did not add** unless the user says so.
