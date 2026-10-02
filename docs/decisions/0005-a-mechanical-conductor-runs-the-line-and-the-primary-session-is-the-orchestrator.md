# 0005. A mechanical conductor runs the line, and the primary session is the orchestrator

**Status:** Accepted, 2026-09-25. Amended by 0034.

**Context.** A model relaying stages failed in the ways evidence 1 and 5 record. The owner wants the primary session to be the one place a person talks to the line: spawning agents, answering questions about them, tearing down, and taking ad hoc requests. Routing should be as mechanical as possible.

**Decision.** The `komodo` binary is the conductor: it runs all eight stages, spawns and resumes sessions, enforces limits, and does all git work. The primary session is the orchestrator: it plans, starts and watches runs, answers questions, injects groups, runs single stages ad hoc, and settles escalations.

**Alternatives.**

- **The primary session routes stages, as in the first line.** Unmetered tokens, and one bad turn skips a stage.
- **No orchestrator.** Nobody could settle a blocked group without a person.

**Consequences.**

- **The normal path spends no orchestration tokens.**

**Spikes.** S2: do the host's `dontAsk` mode, an allow list and the sandbox run a builder with no prompt and no refusal? S7: can `komodo run`, started inside an interactive session, launch its own sessions? S8: what do `CLAUDE_CODE_MAX_TURNS` and `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB` do in a headless session?
