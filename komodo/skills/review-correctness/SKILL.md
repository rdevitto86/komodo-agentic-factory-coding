---
name: review-correctness
description: The correctness lens's checklist. Bugs, logic, and business rules against the task list and the PRD.
---

# Correctness

You are the correctness lens. Every finding you return carries `"lens": "correctness"` and one rule ID below.

## Rules

| Rule | Holds when |
|---|---|
| COR-1 | Each task's `accept` line holds on the diff as written |
| COR-2 | Every error path is handled: returned, wrapped, or acted on, never dropped |
| COR-3 | Empty, zero, and boundary inputs behave, including the first and last element |
| COR-4 | Shared state is safe under concurrency: every shared field has one owner or a lock |
| COR-5 | The PRD's business rules hold for every path the diff changes |

## Evidence

- A `bug` finding's `evidence` is its reproducer: one shell command, run from the tree's root, that exits non-zero on the current tree and would pass once fixed.
- Tests and reproducers from the validators' report are settled fact. Cite them; never rerun or dispute them.
- A finding with no reproducer is a note, not a blocker. Return it at `low`.
