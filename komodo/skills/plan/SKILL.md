---
name: plan
description: /plan: draft task groups from the specs through the planner; each passes lint.
---

# Plan

You turn the PRD and specs into an epic and its task groups under `docs/backlog/`: one folder per epic, one folder per group, one file per task. The planner drafts; you write the tree through `komodo add` and prove it parses before you stop.

Never invent a field; the grammar below is everything the line parses.

{{rules/backlog}}

## The steps

1. Run `komodo migrate` first when the repo holds a BACKLOG.md, a TODO.md or flat files under `docs/backlog/`; it moves them into the tree, opens the imported groups in `REFINEMENT`, and leaves the source for a person to remove.
2. Name the docs the human gave `/plan`, or the PRD and low-level design when none are named.
3. Spawn the planner on them, with the goal and the existing epics and groups it must not duplicate. It reads and returns an epic, its goal and version, and the groups under it; it never edits.
4. Create the epic with `komodo add EPIC-NN "<title>" --version <version>`, which writes `docs/backlog/epic-NN/EPIC.md`; paste the goal paragraph into it.
5. Create each group with `komodo add TG-NN.M "<title>"`, which writes `docs/backlog/epic-NN/tg-NN.M/TG.md`, then each task with `komodo add TG-NN.M "<task title>" --files <a,b> --done-when "<command>"`, one `tsk-<id>.md` per task.
6. Run `komodo lint`. A non-zero exit means a file is wrong: fix it and lint again.
7. Report the epic, each group, its tasks, and the planner's gaps to the human.

## Rules

- **A task names its files and its `done_when` commands.** A command whose zero exit does not prove the task is not a `done_when`.
- **Dependencies make the waves.** `depends_on` is the only thing that orders work; never rely on the order on the page.
- **A gap is the human's decision.** Plan the rest, and list each gap; never settle one yourself.
- **Every task traces to a line in the docs.** Out-of-scope work is one line in the report, never a task.
- **A plan lands through a pull request,** like any other change.
- **A foreign repo's SDD or design doc still plans.** Name it to `/plan`; the planner maps it onto `docs/hld.md`, `docs/lld.md` and `docs/decisions/`.
