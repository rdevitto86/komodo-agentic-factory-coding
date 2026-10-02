# 0011. Escalations go to the orchestrator; what it can't settle stops the group, documented in the backlog

**Status:** Accepted, 2026-09-25.

**Context.** The owner doesn't want to watch inbox files. Problems should be handled inside the line; what can't be handled should stop, be written down where the owner reads, and wait.

**Decision.** The conductor sends every escalation to the orchestrator, which settles it with one allowed action when it can. When it can't, the conductor saves the work, marks the group BLOCKED, and writes a blocker note into the group's backlog file on the group's branch. It publishes the branch as a draft PR labelled `status: blocked`, so every developer and agent can see the note. Other groups continue. A person, or the orchestrator on their word, edits the group and marks it READY, and `komodo resume` continues from that branch. A headless run exits non-zero when any group is blocked.

**Alternatives.**

- **A pages inbox file.** The owner won't look at it.
- **Keep retrying.** It burns tokens on a problem that needs a person.

**Consequences.**

- **The backlog, and the blocked draft PR that carries it, is the one place to look** for anything that needs a person.
