---
name: workflow-decompose
description: Read the spec and backlog in a fork, and return an executable task queue.
argument-hint: [target state, defaults to the current one]
context: fork
agent: planner
background: false
---

# Decompose

Scope: **$ARGUMENTS** — a target state. Empty means the current `## Now` state.

**You cannot see the calling conversation.** Everything you need is on disk, and the queue you return is the only thing that reaches it.

## Order

1. **Read the sources** in your standing order, stopping at the first one missing.
2. **Story line shape**: `- [SEV][WIP] <text> · <size> · <req-id, optional> → \`<done when>\`` — full rules live in `generate-backlog`, not needed here to parse the queue.
3. **Drop anything already in `CHANGELOG.md`.** A story recorded there has shipped.
4. **A story with no `Done when` command is a gap** — report it, don't invent one.
5. **Check the queue for a chain** before returning.

**If `BACKLOG.md` is absent**, say so in `## Gaps` and stop. Creating it is the caller's job, not yours.

Your standing rules on reading order, inventing nothing, and output shape already apply.
