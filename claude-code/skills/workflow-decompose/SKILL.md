---
name: workflow-decompose
description: Read the spec and backlog in a fork, and return an executable task queue.
argument-hint: a pm brief — Task (target state + scope) | Context | Out of scope (all three required)
context: fork
agent: pm
background: false
---

# Decompose

Brief: **$ARGUMENTS** — a `pm` brief whose `Task` slot carries the target state and the scope it is narrowed to, alongside the required `Context` and `Out of scope`. A `Task` naming a target state but no narrower domain or story means the current `## Now` state, every story not `[BLOCKED]` after step 6 below; a missing or empty required slot is your standing stop.

**You cannot see the calling conversation.** Everything you need is on disk, and the queue you return is the only thing that reaches it.

## Order

1. **Read the sources** in your standing order, stopping at the first one missing.
2. **Task shape**: each task's `SUB-` lines each carry their own nested `* **Done when:**` bullet — a subtask is the acceptance criterion, not a separate list from it. Full rules live in `backlog-modify`, not needed here to parse the queue.
3. **Drop anything already in `CHANGELOG.md`.** A story recorded there has shipped.
4. **A task with no `SUB-` lines, or any `SUB-` line missing a `Done when:` bullet (or whose bullets are prose, not commands), is a gap** — report it, don't invent one.
5. **If the `Task` slot names a domain or story substring, narrow to matching stories** before the checks below. No match named: every story in the target state is in scope.
6. **Test every `[BLOCKED]` story's `Recheck:` condition.** Satisfied → drop `[BLOCKED]` and its subnote, queue the story like any other. Not satisfied → leave it blocked and out of the returned queue. **A `[BLOCKED]` story with no `Recheck:` line is a `## Gaps` finding** — every block needs a testable exit condition, not a permanent one.
7. **Check the queue for a chain** before returning.
8. **Mark transitive blocks.** A story that names a `Depends on`/`(after: ...)` edge to a story still `[BLOCKED]` after step 6 is itself blocked, even if nothing marks it so directly — carry that forward so P2.0 can pick around the whole chain instead of discovering it task by task.
9. **Give every task a `Files` manifest, then return a `## Parallel` section naming every set your standing parallel predicate admits.** That predicate is stated once, in your own rules — apply it as written rather than reasoning one out here. Never omit the heading; `None` is an answer.

10. **Draw the queue's `## Graph` section** — an inline Mermaid graph LR fence, one node per task number and one arrow per `After` edge, exactly as `pm.md`'s template specifies. It accompanies the `## Queue` table and never substitutes for it; omit the heading when no task carries an `After` edge and the queue holds 5 tasks or fewer.

**If `BACKLOG.md` is absent**, say so in `## Gaps` and stop. Creating it is the caller's job, not yours.

Your standing rules on reading order, inventing nothing, and output shape already apply.
