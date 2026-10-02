# 0013. Every hook has one job, one stage and a refusal limit

**Status:** Accepted, 2026-09-25. Amended by 0034.

**Context.** Most loops came from hooks and guards (evidence 2 and 5), including a regression where the guard judged the line's own commands. While these docs were written, the guard refused a Python script because its text held the word "git", and refused editing a script and running it in one command.

**Decision.** Every hook follows the contract in `lld.md#hooks`: one check, one kind of session, an alternative named on every refusal, a refusal limit after which the session ends as blocked, and fail-open on its own error. Hooks never judge the conductor's commands and never parse a command for hidden intent.

**Alternatives.**

- **More guard rules.** Each new rule invited new bypass findings and new loops.

**Consequences.**

- **A refusal costs a few turns at most** before the group escalates.
- **Guard-bypass findings never block review.**
