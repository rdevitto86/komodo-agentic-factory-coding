# 0032. A process tree past 256 processes or 8 GB is killed, and a session's temp root sits outside every repo

**Status:** Accepted, 2026-09-27.

**Context.** TG-06.3's hook tests made a temp group with no `.git`, so a walk up from it reached the real worktree and ran the group's own checks. Those checks reran the tests: a fork bomb of 900 processes that exhausted a 24 GB machine. Nothing bounded a tree, and processes outlived the session that started them.

**Decision.**

- **Every station command and model session runs under a watcher:** past 256 live processes or 8 GB resident, it kills the whole tree and names the breach.
- **Nothing outlives what started it:** when a command or session ends, its group and every process it was seen to own are killed.
- **A session's temp root is a private directory outside every repo,** set through the host's temp variable, so no test can walk up into the real worktree.

**Consequences.**

- **A runaway becomes a failed check or a failed session,** never a machine that stops answering.
- **A station can no longer leave a daemon running.**
