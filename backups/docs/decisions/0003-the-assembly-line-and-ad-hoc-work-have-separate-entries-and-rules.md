# 0003. The coding harness and ad hoc work have separate entries and rules

**Status:** Accepted, 2026-10-02. Amended by 0015, 2026-10-05: the guard row is now the orchestrator suite against every other role's own suite, not a shared line tier.

**Context.** The primary session is where a person talks to the coding harness, and most requests are not a task group: a question, an exploration, a one-off fix. When the two shared one path, the coding harness's refusal limits ended the orchestrator's own session, and a model relayed `/run` turn by turn.

**Decision.**

| | Coding harness | Ad hoc |
|---|---|---|
| Entry | `/run`, which runs `komodo run` | The orchestrator spawns its own agents |
| Driven by | The binary, stage by stage | The orchestrator |
| Unit of work | A READY task group from `docs/backlog/` | Any request |
| Guard | Global, plus the role's own suite (0015) | Global and orchestrator suites only |
| Git | The conductor commits, pushes and opens PRs | The agent works its own branch and opens a PR with `komodo pr create` |
| Output | A reviewed draft PR into the epic branch | Whatever the request needs |

- **The orchestrator also plans, starts and watches runs, and settles what a run can't.** It never relays a stage.
- **Use the coding harness for repeatable work that needs proof;** use ad hoc for everything else.

**Alternatives.**

- **An `adhoc` skill and `komodo stage`.** They duplicated the orchestrator's own ability to spawn an agent.
- **No orchestrator.** Nobody could settle a blocked group without a person.

**Consequences.**

- **The orchestrator runs parallel agents without tripping the coding harness's limits.**
