# 0014. The backlog is a tree of epics, groups and tasks

**Status:** Accepted, 2026-10-04. Amends 0004, 0006, 0011 and 0012.

**Context.** Flat files, one per group, made the epic implicit: a yaml field every group repeated, with its version, and seven readers each re-deriving which groups shared one. A review could grow a running group past its cap, since nothing counted its files. An epic's goal had no place to live, so its pull request opened with no text. One file per task lets a split, or a filed follow-up, add a file instead of rewriting one.

**Decision.**

- **The layout is a tree:** `docs/backlog/epic-NN/EPIC.md`, `docs/backlog/epic-NN/tg-NN.M/TG.md`, and `docs/backlog/epic-NN/tg-NN.M/tsk-<id>.md`, one file per task.
- **`EPIC.md` holds the epic's heading, `version`, optional `type` and `groups_max`, and one goal paragraph.** The paragraph is the epic pull request's title and body. Every group under the epic inherits the version and the epic; lint refuses either field on `TG.md`.
- **Numbers place things.** Folder and file names are the lower-cased IDs and must match the IDs inside; `TG-15.3` lives in `epic-15`, and `tsk-15.3.2.md` in `tg-15.3`. Task files sort numerically.
- **Caps are lint's.** A group holds 1 to 12 buildable tasks; its open tasks declare at most 20 unique files, a ticked task's files not counted. An epic holds at most `groups_max` groups, 6 by default.
- **The tree is written through `komodo add`** and left by `komodo migrate`, which moves a flat backlog, `BACKLOG.md` or TODO.md into it and leaves the source for a person to remove. A flat file under `docs/backlog/` is a lint note and a doctor warning.
- **The ship commit of an epic's last open group deletes `docs/backlog/epic-NN/`.** An epic PR adds no backlog file, so `main` never holds one.

**Alternatives.**

- **Keep flat files with an `epic` field.** No place for the epic's goal, no cap on an epic, and seven readers each re-deriving the epic from a field.
- **One file per epic holding all its groups.** Two branches that each add a task conflict on the same file; 0004's `BACKLOG.md` problem returns.
- **A database.** Rejected in 0004: a committed plan keeps agents in sync across workloads.

**Consequences.**

- **Every reader walks the tree:** lint, ingest, status, doctor, `komodo add` and `komodo migrate`.
- **`komodo migrate` moves old layouts** into the tree in `REFINEMENT`.
- **The epic PR carries the goal,** and one section per landed group.
- **A group cannot grow past its cap mid-run;** a split adds a sibling group folder under the same epic.
