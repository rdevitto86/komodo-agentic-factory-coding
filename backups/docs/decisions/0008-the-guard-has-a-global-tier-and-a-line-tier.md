# 0008. The guard has a global tier and a line tier

**Status:** Accepted, 2026-10-02. Amended by 0012, 2026-10-02. Superseded by 0015, 2026-10-05: one binary now holds a fixed suite per role instead of one shared line tier.

**Context.** Most of the first harness's loops came from hooks: the guard judged the harness's own commands, refused a script whose text held "git", and ended the orchestrator's session for spawning builders. One set of rules can't fit both a captive line session and the person's own orchestrator.

**Decision.**

| Tier | Applies to | Refuses | Limit |
|---|---|---|---|
| Global | Every session | Writing a critical ref, force push, `--no-verify`, commit trailers, host and toolkit config, `gh pr merge` and `gh pr create` | None |
| Line | A session `KOMODO_ROLE` names | Everything global, plus writes outside the worktree, isolated spawns, and pushing or merging onto an epic branch | The session ends as blocked after 3 refusals |

- **Every hook has one check,** names an alternative on every refusal, and fails open on its own error. It never judges the conductor's commands and never parses a command for hidden intent.
- **Each role's allow list covers its stage,** so a line session isn't refused work it needs.
- **In this repo, the orchestrator may edit the guard and its rules on a branch;** a change applies only after a person merges it and the binary rebuilds.

**Alternatives.**

- **One tier with orchestrator exceptions.** Every new orchestrator behaviour needed a carve-out.
- **More guard rules.** Each one invited new bypass findings and new loops.

**Consequences.**

- **A refusal costs a line session a few turns at most,** and guard-bypass findings never block review.
