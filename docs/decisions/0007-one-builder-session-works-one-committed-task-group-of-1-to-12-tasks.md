# 0007. One builder session works one committed task group of 1 to 12 tasks

**Status:** Accepted, 2026-09-25. Consolidated 2026-10-02 from 0007, 0008 and 0009.

**Context.** The owner's model is a task list handed to one agent and cross-checked by one reviewer, the size of one engineering story. A session per task pays the fixed prompt again for every task. Builders spent turns finding context (evidence 6). A single `BACKLOG.md` became a database (evidence 12).

**Decision.**

- **A group of 1 to 12 tasks gets one builder session in one worktree,** and its task list is the brief. `komodo lint` refuses a larger group. Groups that share no file run in parallel.
- **Ingest compiles each READY group into a card with zero model calls:** task list, files, derived checks, context pack, size and base. The conductor ticks a task's checkbox only after its checks pass.
- **Each group is a committed file in `docs/backlog/`,** carrying its epic's ID, and plan changes land through pull requests. The conductor ticks boxes on the group's own branch, so its PR shows the code and the completed list together.
- **Finished files go when their epic ends;** a group with no epic deletes its own file. Each group's PR adds a `changelog.d/` fragment. There is no index and no archive: `komodo backlog` lists the open groups.

**Alternatives.**

- **One session per task.** It repeats the fixed prompt, and the reviewer sees fragments of one story.
- **The builder ticks its own boxes.** A model's word is never the proof.
- **A local backlog, or an archive of finished groups.** Other machines couldn't see the plan, and an archive duplicates the changelog.

**Consequences.**

- **One group is one pull request,** bounded by its time limit.
- **The same card and tree give the same brief bytes** on every machine.
- **Parallel groups never edit the same file,** so their PRs don't conflict over the plan.
