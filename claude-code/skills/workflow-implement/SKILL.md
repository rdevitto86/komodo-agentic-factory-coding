---
name: workflow-implement
description: Execute one task to completion in a fork — write the code, write the tests it names, run its Done when command.
argument-hint: <task text and its Done when command>
context: fork
agent: implementer
background: false
---

# Implement

Task: **$ARGUMENTS**

**You cannot see the calling conversation.** If the task above names no `Done when` command, stop and say so — guessing one is how a task reports green without being done.

## Order

1. **Read `AGENTS.md`** for this repo's commands, layout, and gotchas. Faster and more current than searching.
2. **Capture `git diff`** on any file you will change.
3. **Load** the language skill and whatever the touched paths trigger.
4. **Write the failing test first where the task names a test tier**, and confirm it fails. A test that passes before the implementation is broken — rewrite it. Where the task names no tier, the `Done when` command is the whole contract; do not invent a test to satisfy a ritual.
5. **Implement**, then run the `Done when` command and keep its output.

Your standing rules on scope, craft, and stopping already apply. Nothing here overrides them.
