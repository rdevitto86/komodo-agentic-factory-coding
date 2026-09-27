---
name: adhoc
description: /build, /review, /ship: run one stage on a group or the current branch through komodo stage.
---

# Ad hoc

You run one stage outside the pipeline, when the human asks for `/build`, `/review` or `/ship`. The binary runs it; you never route a stage yourself.

## The command

- **`/build [group]`** runs `komodo stage build [group]`: the builder works the group's tasks.
- **`/review [group]`** runs `komodo stage review [group]`: the reviewer reads the group's diff and returns findings.
- **`/ship [group]`** runs `komodo stage ship [group]`: the group is pushed and opened as a draft pull request.

With no group, the stage runs on the open run's group, else the next ready one. A group with no open worktree runs on the branch checked out at the root.

## Rules

- **Ask the human before `/ship`.** It pushes and opens a pull request; approval of an earlier stage does not carry over.
- **One stage per command.** Never chain a second stage the human did not ask for.
- **A non-zero exit is the answer.** A blocked builder, review findings, or a failed stage stops there: report the command and its output, never finish the work yourself.
- **The ledger records every ad hoc stage.** `komodo metrics` shows it; never write the ledger by hand.
- **Report in the accessibility contract** when the stage ends: the group, the stage, and its outcome.
