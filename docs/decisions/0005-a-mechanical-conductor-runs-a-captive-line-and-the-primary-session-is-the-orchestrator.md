# 0005. A mechanical conductor runs a captive line, and the primary session is the orchestrator

**Status:** Accepted, 2026-09-25. Consolidated 2026-10-02 with 0034's line and ad hoc parts.

**Context.** A model relaying stages failed in the ways evidence 1 and 5 record. The owner wants the primary session to be the one place a person talks to the line: spawning agents, answering questions about them, tearing down, and taking ad hoc requests. A full drain that still ran a role-less model relaying `/run` was the loop this decision meant to retire.

**Decision.**

- **The `komodo` binary is the conductor.** It runs all eight stages, spawns and resumes sessions, enforces limits, and does all git work.
- **The line is captive: `/run`, which runs `komodo run`, is its one entry.** A drain drives each group through the conductor directly; no session relays the loop.
- **The primary session is the orchestrator.** It plans, starts and watches runs, answers questions, injects groups, and settles escalations.
- **Ad hoc work is the orchestrator spawning its own default agents,** outside the line and its guard tier, with no skill of its own.

**Alternatives.**

- **The primary session routes stages, as in the first line.** Unmetered tokens, and one bad turn skips a stage.
- **No orchestrator.** Nobody could settle a blocked group without a person.
- **An `adhoc` skill and `komodo stage`.** They duplicated the orchestrator's own ability to spawn an agent for one stage.

**Consequences.**

- **The normal path spends no orchestration tokens.**
- **`docs/prd.md` and REQ-39 still promise single stages run ad hoc;** they follow this decision.
