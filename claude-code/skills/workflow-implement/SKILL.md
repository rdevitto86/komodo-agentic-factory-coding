---
name: workflow-implement
description: Execute one task to completion in a fork — write the code, write the tests it names, run its Done when commands.
argument-hint: <task text and its Done when commands>
context: fork
agent: builder
background: false
---

# Implement

Task: **$ARGUMENTS**

**You cannot see the calling conversation.** If the task above names no `Done when` commands, stop and say so — guessing one is how a task reports green without being done.

## Order

1. **Verify the task's premise before doing anything else.** Read the function or file the task names and confirm the defect or gap it describes is still present against current code — a stale task can reach an implementer whose premise the repo has already outgrown. If the premise no longer holds, stop: report it back rather than forking ahead, so the task can be re-aimed or removed from `BACKLOG.md` instead of shipping nothing. A premise that still holds, even with code that looks already-written, is inherited state to verify (run the fork's own `Done when` commands against it) — never a reason to skip the task; see `workflow-loop/SKILL.md`'s "run the fork even when the code already appears to exist" rule.
2. **Read `AGENTS.md`** for this repo's commands, layout, and gotchas. Faster and more current than searching.
3. **Capture `git diff`** on any file you will change.
4. **Load** the language skill and whatever the touched paths trigger.
5. **Write the failing test first where the task names a test tier**, and confirm it fails. A test that passes before the implementation is broken — rewrite it. Where the task names no tier, the `Done when` commands are the whole contract; do not invent a test to satisfy a ritual.
6. **Implement**, then run every `Done when` command and keep each one's output.
7. **Tick the Acceptance Criteria box your subtask satisfied.** Once a subtask's `Done when` command(s) all exit zero, and the parent task's `Acceptance Criteria` block names a specific `AC-N` that subtask proves, flip that box from `- [ ]` to `- [x]` in `BACKLOG.md` as part of reporting completion. When an AC doesn't map cleanly to one subtask, ticking it is a judgment call you make and state plainly in your own report — never guess silently, and never tick a box no subtask actually proved.
8. **Comments, last** — per your standing rules; the `Done when` commands must already be green. See `write-comments/reference.md` for the taxonomy each proposal must clear.

Your standing rules on scope, craft, and stopping already apply. Nothing here overrides them.
