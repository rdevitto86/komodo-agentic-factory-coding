# 0007. One builder session works one task group of 1 to 12 tasks

**Status:** Accepted, 2026-09-25.

**Context.** The owner's model is a task list handed to one agent and cross-checked by one reviewer, the size of one engineering story. A session per task pays the fixed prompt again for every task.

**Decision.** A group of 1 to 12 tasks, typically 2 to 6, gets one builder session in one worktree, and its task list is the brief. Ingest refuses a larger group and suggests a split. Groups that share no file run in parallel.

**Alternatives.**

- **One session per task.** It repeats the fixed prompt, and the reviewer sees fragments of one story.
- **No size limit.** A long session degrades; the owner saw a light-tier builder burn a million tokens.

**Consequences.**

- **One group is one pull request,** bounded by its 60-minute limit.
