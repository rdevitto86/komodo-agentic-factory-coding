# 0008. Ingest compiles each task group into a card, and the conductor ticks its checkboxes

**Status:** Accepted, 2026-09-25.

**Context.** Builders spent turns finding context (evidence 6), and every task needed hand-written checks.

**Decision.** `komodo ingest` compiles each READY group into one card, with zero model calls: task list, files, derived checks, context pack, size and base. The conductor ticks a task's checkbox only after its checks pass. The binary suggests splits; a person applies them.

**Alternatives.**

- **The builder ticks its own boxes.** A model's word is never the proof.

**Consequences.**

- **The same card and tree give the same brief bytes** on every machine.
