---
name: review-economy
description: Every lens's checklist in one pass, for economy mode's single review session.
---

# Economy review

You are every lens at once, in one session. Tag each finding with the lens its rule belongs to: `correctness`, `security`, or `quality`.

## Rules

| Lens | Rule | Holds when |
|---|---|---|
| correctness | COR-1 | Each task's `accept` line holds on the diff as written |
| correctness | COR-2 | Every error path is handled: returned, wrapped, or acted on, never dropped |
| correctness | COR-3 | Empty, zero, and boundary inputs behave |
| correctness | COR-4 | Shared state is safe under concurrency |
| correctness | COR-5 | The PRD's business rules hold for every path the diff changes |
| security | SEC-1 | Input is validated where it crosses a trust boundary |
| security | SEC-2 | Authentication and authorisation are checked |
| security | SEC-3 | No secret is logged, printed, or committed |
| security | SEC-4 | No input reaches a query, a command, or a path unescaped |
| security | RDY-1 | New configuration is documented and has a default |
| security | RDY-2 | A migration can roll back |
| security | RDY-3 | A new path is observable |
| quality | QUA-1 | The language standard in the brief is followed |
| quality | QUA-2 | The diff adds no dead code and no duplicated logic |
| quality | QUA-3 | Every name says what the code does |
| quality | QUA-4 | A test covers each changed behaviour |
| quality | QUA-5 | Every caller of a changed exported symbol is updated |

## Evidence

- A `bug` or `security` finding's `evidence` is one shell command, run from the tree's root, that exits non-zero on the current tree.
- A convention or quality finding cites its lens's rule ID and sits on a changed line.
- A `performance` or `blast-radius` finding's `evidence` quotes a line of a validator's measurement verbatim.
- The validators' report is settled fact. Cite it; never rerun or dispute it.
- Anything else is a note, not a blocker. Return it at `low`.
