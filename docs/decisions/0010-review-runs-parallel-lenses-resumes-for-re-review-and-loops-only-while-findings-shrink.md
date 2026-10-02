# 0010. Review runs parallel lenses, resumes for re-review, and loops only while findings shrink

**Status:** Accepted, 2026-09-25.

**Context.** Review never converged (evidence 3). The owner wants a large, unbiased review against a checklist, backed by mechanical validators, waiting in stasis during repairs. A fixed repair count is arbitrary and only defers the loop.

**Decision.** Validators run first. Three lenses run in parallel on the group's diff and task list: correctness, security and readiness, and quality; economy mode runs one combined lens. A finding blocks only with evidence the conductor verifies. Repair resumes the builder with a fix list. Re-review resumes the lens sessions, which may only close their findings or flag lines the repair changed. A round that closes nothing ends the loop.

**Alternatives.**

- **A fixed repair count.** Arbitrary, and it defers the loop rather than ending it.
- **One reviewer for every group.** Its context grows with each group and it stops being unbiased.
- **A fresh reviewer each round.** It raises new objections to code it never saw, as in TG-03.22.

**Consequences.**

- **Differences between reviewers' reports can't cause loops,** since only verified findings block.
