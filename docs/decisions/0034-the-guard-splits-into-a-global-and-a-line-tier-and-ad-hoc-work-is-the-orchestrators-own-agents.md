# 0034. The guard splits into a global and a line tier, and ad hoc work is the orchestrator's own agents

**Status:** Accepted, 2026-09-29. Amends 0005, 0012 and 0013.

**Context.** The guard judged the orchestrator as a line session and ended its session for spawning isolated builders, a plain `komodo run` refusal limit hit the orchestrator's own parallel spawns, and a full drain still ran a role-less model relaying `/run`, the loop 0005 meant to retire. Nothing named which rules held for every session and which held only for the line.

**Decision.**

- **The guard has a global tier for every session:** critical refs, force push, `--no-verify`, commit trailers, and host and toolkit config paths, with no refusal limit.
- **The guard has a line tier for a session `KOMODO_ROLE` names:** writes outside the worktree, isolated spawns, `LineRefusedPaths`, the epic branch's push and merge, and the refusal limit of 0013. `komodo run` sets the marker in its own environment, so every session and subagent it starts inherits it.
- **The line is captive: `/run`, which runs `komodo run`, is its one entry.** A drain drives each group through the conductor directly; no session relays the loop.
- **Ad hoc work is the orchestrator spawning its own default agents, outside the line and its line tier, with no skill of its own.** The `adhoc` skill and `komodo stage` go.

**Alternatives.**

- **Keep one guard tier and add an orchestrator exception.** Every new orchestrator behavior would need its own carve-out.
- **Keep the `adhoc` skill and `komodo stage`.** They duplicated the orchestrator's own ability to spawn an agent for one stage, with an extra skill to keep pinned and scoped.

**Consequences.**

- **The orchestrator spawns parallel builders and isolated agents without tripping the line's rules.**
- **A session outside the line still answers for the rules that hold everywhere.**
