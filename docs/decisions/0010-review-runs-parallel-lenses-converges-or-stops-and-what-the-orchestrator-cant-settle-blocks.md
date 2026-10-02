# 0010. Review runs parallel lenses, converges or stops, and what the orchestrator can't settle blocks

**Status:** Accepted, 2026-09-25. Consolidated 2026-10-02 from 0010 and 0011.

**Context.** Review never converged (evidence 3). The owner wants a large, unbiased review against a checklist, backed by mechanical validators, waiting in stasis during repairs. The owner doesn't watch inbox files: what the line can't handle should stop, be written down where the owner reads, and wait.

**Decision.**

- **Validators run first.** Three lenses then run in parallel on the group's diff and task list: correctness, security and readiness, and quality. Economy mode runs one combined lens.
- **A finding blocks only with evidence the conductor verifies.** Repair resumes the builder with a fix list. Re-review resumes the lens sessions, which may only close their findings or flag lines the repair changed. A round that closes nothing ends the loop.
- **Escalations go to the orchestrator,** which settles each with one allowed action when it can.
- **When it can't, the group is BLOCKED.** The conductor saves the work, writes a blocker note into the group's backlog file on its branch, and publishes the branch as a draft PR labelled `status: blocked`. Other groups continue; a headless run exits non-zero.
- **A person, or the orchestrator on their word, marks the group READY,** and `komodo resume` continues from that branch.

**Alternatives.**

- **A fixed repair count.** Arbitrary, and it defers the loop rather than ending it.
- **One reviewer for every group, or a fresh reviewer each round.** The first stops being unbiased; the second raises new objections to code it never saw, as in TG-03.22.
- **An inbox file, or retrying.** The owner won't read the file, and retrying burns tokens on a problem that needs a person.

**Consequences.**

- **Differences between reviewers' reports can't cause loops,** since only verified findings block.
- **The blocked draft PR is the one place to look** for anything that needs a person.

**Open.** `internal/conductor/drive.go` gives each lens one cold pass, a fresh reviewer, after 2 warm rounds. This decision rejects fresh reviewers; either record the cold pass here and in `lld.md`, or remove it.
