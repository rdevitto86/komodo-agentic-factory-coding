---
name: workflow-decompose
description: Read the spec and backlog in a fork, and return an executable task queue.
argument-hint: [target state] [scope: a domain or story text, defaults to everything not blocked]
context: fork
agent: workflow-planner
background: false
---

# Decompose

Scope: **$ARGUMENTS** — a target state, optionally narrowed to one domain or story. Empty means the current `## Now` state, every story not `[BLOCKED]` after step 6 below.

**You cannot see the calling conversation.** Everything you need is on disk, and the queue you return is the only thing that reaches it.

## Order

1. **Read the sources** in your standing order, stopping at the first one missing.
2. **Task shape**: each task's `SUB-` lines each carry their own nested `* **Done when:**` bullet — a subtask is the acceptance criterion, not a separate list from it. Full rules live in `backlog-modify`, not needed here to parse the queue.
3. **Drop anything already in `CHANGELOG.md`.** A story recorded there has shipped.
4. **A task with no `SUB-` lines, or any `SUB-` line missing a `Done when:` bullet (or whose bullets are prose, not commands), is a gap** — report it, don't invent one.
5. **If `$ARGUMENTS` names a domain or story substring, narrow to matching stories** before the checks below. No match named: every story in the target state is in scope.
6. **Test every `[BLOCKED]` story's `Recheck:` condition.** Satisfied → drop `[BLOCKED]` and its subnote, queue the story like any other. Not satisfied → leave it blocked and out of the returned queue. **A `[BLOCKED]` story with no `Recheck:` line is a `## Gaps` finding** — every block needs a testable exit condition, not a permanent one.
7. **Check the queue for a chain** before returning.
8. **Mark transitive blocks.** A story that names a `Depends on`/`(after: ...)` edge to a story still `[BLOCKED]` after step 6 is itself blocked, even if nothing marks it so directly — carry that forward so P2.0 can pick around the whole chain instead of discovering it task by task.
9. **Return a `## Parallel` section naming every set of two or more queued tasks that share no file and no `Depends on`/`(after: ...)` edge among them** — this is what `workflow-loop`'s P2.1 dispatches together with `isolation: worktree` instead of one at a time. State `None` when no such set exists; never omit the heading.

**If `BACKLOG.md` is absent**, say so in `## Gaps` and stop. Creating it is the caller's job, not yours.

Your standing rules on reading order, inventing nothing, and output shape already apply.
