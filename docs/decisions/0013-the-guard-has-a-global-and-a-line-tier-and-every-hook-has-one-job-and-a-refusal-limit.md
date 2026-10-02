# 0013. The guard has a global and a line tier, and every hook has one job and a refusal limit

**Status:** Accepted, 2026-09-25. Consolidated 2026-10-02 from 0013, 0014, 0015 and 0034's guard tiers.

**Context.** Most loops came from hooks and guards (evidence 2 and 5), including the guard judging the line's own commands, refusing a Python script whose text held "git", and ending the orchestrator's session for spawning builders. Agents were refused commands their work needed. The owner was blocked from changing this repo by its own guard.

**Decision.**

- **Every hook follows the contract in `lld.md#hooks`:** one check, one kind of session, an alternative named on every refusal, and fail-open on its own error. Hooks never judge the conductor's commands and never parse a command for hidden intent.
- **The global tier holds for every session:** critical refs, force push, `--no-verify`, commit trailers, host and toolkit config paths, and `gh` merging or opening a pull request, with no refusal limit.
- **The line tier holds for a session `KOMODO_ROLE` names:** writes outside the worktree, isolated spawns, the epic branch's push and merge, and a refusal limit of 3, after which the session ends as blocked. `komodo run` sets the marker, so every session it starts inherits it.
- **Each role's allow list covers its stage,** applied with the host's `dontAsk` mode. Builders may delete and move files in their worktree; switching, syncing, committing and cleanup belong to the conductor.
- **In this repo, the orchestrator may edit rules, skills, policy and guard source on a branch.** A change applies only after a human merges it and the binary rebuilds, so no session loosens its own guard.

**Alternatives.**

- **More guard rules, or one tier with an orchestrator exception.** Each rule invited new bypass findings; each new behaviour would need a carve-out.
- **Bypass permissions, with the guard as the only wall.** Evidence 2.

**Consequences.**

- **A refusal costs a few turns at most,** and guard-bypass findings never block review.
- **The orchestrator spawns parallel builders without tripping the line's rules,** and still answers for the global ones.
