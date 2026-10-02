# 0021. Readiness is measured by `komodo eval`, never by a model's score

**Status:** Accepted, 2026-09-25.

**Context.** A self-graded score defined "done" (evidence 4), and green could mean nothing ran (evidence 15).

**Decision.** `komodo eval` runs golden task groups in real repos, with hidden tests, against the PRD's success criteria. The gate fails loudly when it finds no build check. A local model is optional and never a proof.

**Alternatives.**

- **A rubric a model scores.** It swung between 72 and 88 on the same code.

**Consequences.**

- **Only a proof moves readiness.**
