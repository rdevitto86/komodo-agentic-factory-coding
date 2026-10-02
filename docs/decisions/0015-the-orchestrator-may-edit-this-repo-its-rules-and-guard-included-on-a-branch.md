# 0015. The orchestrator may edit this repo, its rules and guard included, on a branch

**Status:** Accepted, 2026-09-25.

**Context.** The owner was blocked from changing this repo by its own guard and rules, which protected `komodo/policy.json` and `bin/**`.

**Decision.** In this repo, the primary session may edit rules, skills, policy and guard source on a branch. A change applies only after a human merges it and the binary rebuilds, so no session loosens its own guard. A builder may edit those files only when its task list names them. Only the build writes binaries.

**Alternatives.**

- **Keep the self-protection.** It blocks the owner's own maintenance.

**Consequences.**

- **The policy's protected paths name installed copies,** not this repo's sources.
