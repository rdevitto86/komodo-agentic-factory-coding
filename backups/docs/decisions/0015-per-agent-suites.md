# 0015. Guards, commands and hooks split into one suite per agent, and escalation is gone

**Status:** Accepted, 2026-10-05. Supersedes the tier split in 0008. Amends 0005: it no longer escalates to an orchestrator session; the running orchestrator settles a stop directly.

**Context.** 0008 split the guard into a global tier and one line tier shared by every role the harness starts. A builder, a reviewer and a planner have different jobs and different risks, so one shared tier either over-refuses one role or under-refuses another. 0005 and the LLD also routed every stop through a separate "escalation" session the conductor spawned. That session was a role with no work of its own, and a stop waited on it starting before anyone could act.

**Decision.**

- **One binary, nine suites.** `komodo guard --suite <name>` and the matching command and hook sets live in one binary, each suite permanently tied to one role: global, orchestrator (the person's session), builder, reviewer, architect, planner, tester, scout, researcher. `KOMODO_ROLE` picks the suite a session loads; a role can never borrow another's.
- **No escalation role, state or session.** A builder blocked, a stalled session, or a reviewer's third strike goes straight to the running orchestrator session, the person's own interactive session or the one unattended process minding the run. It settles the stop or stops the group; nothing spawns a fresh session to decide for it.
- **No responder.** A reviewer's findings and a builder's fixes pass directly through the harness; builders only build, reviewers only review, and neither waits on a third role to relay the other.
- **Builders get a window, not a session count.** 30 minutes per task list, up to 3 retries. A retry is reviewer pushback only, resumes the same builder, resets its window and counts against its 3, so a task list is never handed to a second builder. 10 minutes with no activity marks a builder stalled: the orchestrator pushes it once, waits 5 minutes' grace, then kills it (`spunDown`).
- **Reviewers get a shorter clock.** 10 minutes maximum, 10 minutes idle, the same push, grace and kill. Three strikes against one builder's submissions blocks that builder permanently, until a person clears it.
- **The orchestrator keeps a mechanical registry, per builder:** start time, window start, retry count, strike count, and state (`running`, `stalled`, `reviewing`, `blocked`, `spunDown`, `done`). One goroutine watches one builder, and the registry persists in run state so a restart reads it back.
- **Release prep moves into the orchestrator.** There is no release agent: once a reviewer approves, the orchestrator drafts the changelog entry and runs `komodo release`.
- **A plan is terminal output only.** Every plan a role drafts prints to the session's own terminal as text; no role writes it into a host's plan store, Claude's or any other's.
- **Testing stays unit and component, with mocks, at 85% per package or higher.** Integration tests are reserved for the local-LLM and OpenAI cross-communication work that is not yet ready; `tester` runs ad hoc, read-only QA against the specs and never writes a test.

**Alternatives.**

- **Keep the escalation session.** It added a cold start and a second role to reason about for every stop, and most stops needed only the orchestrator that was already running.
- **A keep-alive ping on a fixed interval.** It can't tell a thinking session from a stalled one; the idle timer measures silence directly.
- **An unbounded retry count.** It defers the loop instead of ending it, the same failure mode 0005 already rejected for review rounds.

**Consequences.**

- **A stop costs no cold-start session;** the orchestrator that is already running settles it or stops the group.
- **Nine fixed suites replace a growing allow-list on one shared tier,** so a role's refusals are its own, not another role's leftovers.
- **The registry, not a transcript, is what `komodo status` and a restart read**, so a builder's state survives a crash.
