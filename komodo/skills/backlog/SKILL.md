---
name: backlog
description: Write and repair BACKLOG.md in the grammar the line parses. Add tasks, lint them, and plan a group.
---

# Backlog

`BACKLOG.md` is the only queue. You write it in the grammar and prove it parses before you stop.

## The grammar

`komodo/rules/backlog.md` is the grammar the line parses. Read it before writing a task; never invent a field.

## The commands

- **`komodo list [--json]`** — every task, or one group's.
- **`komodo add <group> <title>`** — append a task to a group, in the grammar.
- **`komodo lint`** — the parser's verdict. Run it after every edit; a non-zero exit means the edit is wrong.
- **`komodo next [--json]`** — the next ready group, its waves, and the machines that serve them.

## Rules

- **A task names its files and its `done_when` commands.** A command whose zero exit does not prove the task is not a `done_when`.
- **Dependencies make the waves.** `depends_on` is the only thing that orders work; never rely on the order on the page.
- **Plan with the planner role.** It reads and returns tasks; it never edits `BACKLOG.md` itself.
- **Out-of-scope work is one line here,** never a change in the same turn.
