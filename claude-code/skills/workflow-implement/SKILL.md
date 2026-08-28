---
name: workflow-implement
description: Execute one task to completion in a fork — write the code, write the tests it names, run its Done when commands.
argument-hint: <task text and its Done when commands>
context: fork
agent: workflow-implementer
background: false
---

# Implement

Task: **$ARGUMENTS**

**You cannot see the calling conversation.** If the task above names no `Done when` commands, stop and say so — guessing one is how a task reports green without being done.

## Order

1. **Read `AGENTS.md`** for this repo's commands, layout, and gotchas. Faster and more current than searching.
2. **Capture `git diff`** on any file you will change.
3. **Load** the language skill and whatever the touched paths trigger.
4. **Write the failing test first where the task names a test tier**, and confirm it fails. A test that passes before the implementation is broken — rewrite it. Where the task names no tier, the `Done when` commands are the whole contract; do not invent a test to satisfy a ritual.
5. **Implement**, then run every `Done when` command and keep each one's output.

Your standing rules on scope, craft, and stopping already apply. Nothing here overrides them.
