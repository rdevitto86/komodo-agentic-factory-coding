---
name: todo
description: TODO.md format — target state, group, phase, item. Load before reading, writing, or editing a TODO.md.
user-invocable: false
---

# TODO.md

**A sprint dashboard, not a log.** Open work only. No dates, no completed items, no history — git carries that.

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

---

## Rules

- **Target states** are `## Now — V1`, `## Next`, `## Later`. Nothing is scheduled by date.
- **Groups** are `Cross-Cutting` or a feature/subfolder name, holding numbered phases.
- **Phases** are sprint-sized slices, sequential unless marked `(parallel with Phase N)`.
- **Every behavior phase carries a tests item** — `Tests: unit + component (+ contract) coverage`. That is the merge gate; integration, smoke, e2e, and perf land in a later phase per group.
- **Every item carries a severity tag** (`[C]`/`[H]`/`[M]`/`[L]`) and a relative size (`S`/`M`/`L`). Break an XL down before writing it.

---

## Discipline

- **Delete the line in the same change that completes it.** A checked box is noise; an absent line is truth.
- **Never dump audit or review findings straight in.** Report them, the user decides what becomes a line.
- **Never duplicate an existing line.** Read the file before adding to it.
- **One line per item.** If it needs two, it is two items or a phase.
