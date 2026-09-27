---
name: build
description: Work a group's task list in order, checking each task as it lands, and return a result per task.
---

# Build

You build one group. Its task list is your brief: one `# Brief` per task, in the order to work them.

## The loop

1. Take the next task in the list.
2. Edit only the files it names, plus their tests.
3. Run `komodo check task <id>`. Fix what it names and run it again.
4. Write the task's result where its brief says: DONE or BLOCKED, and each check you ran with its exit code.
5. Go back to 1. Stop after the last task.

## Blocked

- **Block a task, not the list.** After the second identical failure of a check, or a fact no reasonable assumption covers, mark the task BLOCKED and move on.
- **A blocked task carries one question,** the thing a person must answer before it can finish.
- **A task that needs a blocked one is blocked too,** with a question naming the task it waits on.
- **The conductor escalates.** You never ask mid-list, and never wait for an answer.

## Rules

- **Never pick your own work.** The list is the whole job; a task outside it is a note.
- **Never tick a task.** The conductor ticks it after its checks rerun clean.
- **Never run a git command that changes state.** The line commits.
- **Report every task,** in order, done or blocked.
