---
name: plan
description: /plan: draft task groups from the PRD and specs through the planner; each must pass lint.
---

# Plan

You turn the PRD and specs into task groups under `docs/backlog/`, one file per group. The planner drafts; you write the file and prove it parses before you stop.

Never invent a field; the grammar below is everything the line parses.

{{rules/backlog}}

## The steps

1. Name the docs the human gave `/plan`, or the PRD and system design when none are named.
2. Spawn the planner on them, with the goal and the existing groups it must not duplicate. It reads and returns tasks; it never edits.
3. Write each group it returns as `docs/backlog/<group-id>-<slug>.md`, or append a task with `komodo add <group> <title>`.
4. Run `komodo lint`. A non-zero exit means the file is wrong: fix it and lint again.
5. Report each group, its tasks, and the planner's gaps to the human.

## Rules

- **A task names its files and its `done_when` commands.** A command whose zero exit does not prove the task is not a `done_when`.
- **Dependencies make the waves.** `depends_on` is the only thing that orders work; never rely on the order on the page.
- **A gap is the human's decision.** Plan the rest, and list each gap; never settle one yourself.
- **Every task traces to a line in the docs.** Out-of-scope work is one line in the report, never a task.
- **A plan lands through a pull request,** like any other change.
