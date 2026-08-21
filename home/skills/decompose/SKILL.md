---
name: decompose
description: Read the spec and backlog in a fork, and return an executable task queue.
argument-hint: [target state or slice range, defaults to the current one]
context: fork
agent: planner
background: false
disable-model-invocation: true
---

# Decompose

Scope: **$ARGUMENTS** — a target state or slice range. Empty means the current `## Now` state.

**You cannot see the calling conversation.** Everything you need is on disk, and the queue you return is the only thing that reaches it.

## Order

1. **Read the sources** in your standing order, stopping at the first one missing.
2. **Load `worklog`** for the slice-to-story join and the story line shape.
3. **Drop anything already in `CHANGELOG.md`.** A slice recorded there has shipped.
4. **Split each slice into its behavior story and its test stories.** A slice touching a file type `sdlc` defines a tier for, with no test story, is a gap — report it rather than filling it.
5. **Check the queue for a chain** before returning.

**If `BACKLOG.md` is absent**, say so in `## Gaps` and build the queue from slices alone. Creating it is the caller's job, not yours.

Your standing rules on reading order, inventing nothing, and output shape already apply.
