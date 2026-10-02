# 0009. The backlog is a committed plan, one file per group, and finished files go at the end of their epic

**Status:** Accepted, 2026-09-25.

**Context.** A single `BACKLOG.md` became a database (evidence 12). The owner wants the backlog committed as a long-running plan that keeps agents in sync across workloads and shows that work was completed correctly. Files that are no longer needed should go when their epic or group ends, with `CHANGELOG.md` as the record, not an archive.

**Decision.** Each task group is a committed file in `docs/backlog/`, carrying its epic's ID. Changes to the plan land through pull requests like any other change. The conductor ticks a group's boxes on the group's own branch, so its PR shows the code and the completed task list together. When the last open group of an epic merges, that group's PR also deletes the epic's group files; a group with no epic deletes its own file. Each group's PR adds its line to `CHANGELOG.md`. There is no index and no archive: `komodo backlog` lists the open groups.

**Alternatives.**

- **One committed `BACKLOG.md`.** Evidence 12.
- **A local, uncommitted backlog.** Agents on other machines couldn't see the plan.
- **An archive of finished groups.** It duplicates the changelog.

**Consequences.**

- **Parallel groups never edit the same file,** so their PRs don't conflict over the plan.
- **An epic's finished groups stay visible until the epic ends,** so later groups can see what was done.
