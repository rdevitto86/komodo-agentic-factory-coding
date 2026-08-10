---
name: backlog
description: TODO.md format and backlog planning — hierarchy, severity, sizing, merge discipline. Load before reading or editing any TODO.md. Invoke as /backlog to turn a goal into a phased V1/V2 breakdown.
argument-hint: [what you want to build]
---

# Backlog

**A sprint dashboard, not a log.** Open work only. No dates, no completed items, no history — git carries that.

Part 1 is the file contract, and applies any time `TODO.md` is touched. Part 2 is the planning workflow, and runs only on `/backlog`.

---

# Part 1 — The file

Hierarchy is fixed: **target state → group → phase → item.**

```markdown
# TODO
Severity: [C] Critical · [H] High · [M] Medium · [L] Low · Status: `[ ]`/`[~]`

## Now — V1
### Orders API
#### Phase 1 · Create + fetch
- [ ] [C] Idempotent POST /orders · M
- [ ] [M] Tests: unit + component coverage · S
#### Phase 2 · Post-merge validation
- [ ] [M] e2e: order lifecycle · M
```

## Rules

- **Target states** are `## Now — V1`, `## Next`, `## Later`. Nothing is scheduled by date.
- **Groups** are `Cross-Cutting` or a feature/subfolder name, holding numbered phases.
- **Phases** are sprint-sized slices, sequential unless marked `(parallel with Phase N)`.
- **Every behavior phase carries a tests item** — `Tests: unit + component (+ contract) coverage`. That is the merge gate; integration, smoke, e2e, and perf land in a later phase per group. `sdlc` defines the tiers.
- **Every item carries a severity tag** (`[C]`/`[H]`/`[M]`/`[L]`) and a relative size (`S`/`M`/`L`). Break an XL down before writing it.

## Discipline

- **Delete the line in the same change that completes it.** A checked box is noise; an absent line is truth.
- **Never dump audit or review findings straight in.** Report them, the user decides what becomes a line.
- **Never duplicate an existing line.** Read the file before adding to it.
- **One line per item.** If it needs two, it is two items or a phase.

## Merging into an existing file

- **Append under the right target state and group.** Never create a second group with the same name.
- **Never renumber existing phases.** A phase number is a reference someone may already be using.
- **Never remove an item you did not add** unless the user says so.

---

# Part 2 — The planning run

**`/backlog` only.** Target: **$ARGUMENTS**

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

- **V1** — the smallest thing that is genuinely usable. Not a prototype, not feature-complete. Lands under `## Now — V1`.
- **V2** — the next coherent increment. Lands under `## Next`.

Anything beyond V2 is speculation and does not belong in a plan. **A planning run never writes `## Later`** — that section holds parked work the user put there.

## Step 4 — Decompose

Part 1 governs shape. Three constraints are the planning run's own:

- **Cap each run at 3 groups and 4 phases per group.** If the work genuinely exceeds that, plan V1 only and say V2 needs its own pass. An unbounded dump is what made the old ledger useless.
- **A phase is one shippable slice** — mergeable on its own, with a visible result. Sequential by default; mark `(parallel with Phase N)` only where there is genuinely no dependency.
- **Every item names its acceptance condition** in one line. An item nobody else can check is not planned, it is hoped for.

## Step 5 — Present, then stop

Show the plan and **wait for approval**. Do not write to `TODO.md` yet.

```markdown
## 🎯 Target states
- **V1** — <one line>
- **V2** — <one line>

## 📋 Plan
### <group>
#### Phase 1 · <slice>
| Item `[sev]` `size` | Done when |
|---|---|
| <what> `[H]` `M` | <checkable condition> |

## ⚠️ Risks
- **<thing>** — why it could bite

## ❓ Open
- <anything still unresolved>
```

## Step 6 — Write on approval only

Merge into `TODO.md` per Part 1. Never write before approval.
