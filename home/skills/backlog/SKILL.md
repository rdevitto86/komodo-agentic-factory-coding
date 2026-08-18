---
name: backlog
description: TODO.md format and backlog planning — hierarchy, severity, sizing, merge discipline. Load before reading or editing any TODO.md. Invoke as /backlog to turn a goal into a phased V1/V2 breakdown, or to normalize an existing messy file into this shape.
argument-hint: [what you want to build, or a messy file/path to normalize]
paths: "**/TODO.md"
---

# Backlog

**A sprint dashboard, not a log.** Open work only. No dates, no completed items, no history, no checkboxes — git carries that.

Part 1 is the file contract, and applies any time `TODO.md` is touched. Part 2 is the planning workflow, and runs only on `/backlog`. Part 3 is the normalize workflow, for turning an existing unstructured file into Part 1's shape.

---

# Part 1 — The file

Hierarchy is fixed: **target state (version) → group (domain/feature) → phase → item.**

```markdown
# TODO
Severity: [C] Critical · [H] High · [M] Medium · [L] Low · In progress: `[WIP]`

## Now — V1
### Orders API
#### Phase 1 · Create + fetch
- [C] Idempotent POST /orders · M
- [M] Tests: unit + component coverage · S
#### Phase 2 · Post-merge validation
- [M] e2e: order lifecycle · M
```

## Rules

- **No checkboxes, ever.** `- [ ]`/`- [x]` are never written here — a line's mere presence means it's open. Marking one "done" instead of deleting it is the exact drift this file exists to prevent.
- **Target states** are `## Now — V1`, `## Next`, `## Later`. Nothing is scheduled by date.
- **Groups** are `Cross-Cutting` or a feature/subfolder name, holding numbered phases.
- **Phases** are sprint-sized slices, sequential unless marked `(parallel with Phase N)`.
- **Every behavior phase carries a tests item** — `Tests: unit + component (+ contract) coverage`. That is the merge gate; integration, smoke, e2e, and perf land in a later phase per group. `sdlc` defines the tiers.
- **Every item carries a severity tag** (`[C]`/`[H]`/`[M]`/`[L]`) and a relative size (`S`/`M`/`L`). Break an XL down before writing it.
- **Active work carries `[WIP]` right after the severity tag** — `- [C][WIP] Idempotent POST /orders · M`. No tag means not started. Remove `[WIP]` the same change the item's line is deleted, never leave it dangling on a finished item.

## Discipline

- **Work one phase to completion before starting the next.** A phase "blocked" by a missing SDK capability is never a reason to skip ahead — fix the SDK directly, per `coding-principles`' reuse order, and finish the phase you're on.
- **Delete the line in the same change that verifies it complete.** Marking it done and leaving it is noise; an absent line is the record — `git log TODO.md` carries the history.
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

---

# Part 3 — Normalizing an existing file

Same command, different ask: **$ARGUMENTS** names a messy or unstructured file — a `TODO.md` with no hierarchy, checkboxes, dates, or already-done items; scattered `// TODO` comments; a plain notes file — instead of a new goal.

## Step 1 — Read everything, invent nothing

Read the source file(s) in full. Every line becomes exactly one of:

- **A genuinely open item** → keep, rewritten to Part 1's shape, no `[WIP]` unless the source says work is active.
- **Something already done** (phrased as done, or contradicted by the current code) → drop. Verify against the code before dropping — never drop on the comment's word alone.
- **A date, a name, a status log entry, a checked box** → drop; git already carries that history.
- **Too vague to act on** (no acceptance condition derivable) → keep as a `[L]` item naming exactly what's unclear. Never invent detail the source didn't state.

## Step 2 — Sort into the hierarchy

Assign each surviving item a target state (default `## Now — V1` unless the source clearly marks it future work), a group (the domain/feature it belongs to — infer from the file's path or the item's own text, never a new taxonomy), and a phase (sequential, sized per Step 4's caps above).

## Step 3 — Present, then stop

Show the normalized file as a diff against the source and **wait for approval** before writing — this replaces the user's existing file, so it is never silent.
