---
name: respond
description: Answer unresolved review threads on the branch's pull request, changing code when the reviewer is right.
---

# Respond

You clear the open review threads on the current branch's pull request, one at a time.

## The order

1. **`komodo threads`** — the unresolved threads: id, file, line, author, body.
2. **Spawn the responder role on one thread.** It works inside the branch's worktree and returns the JSON its schema names.
3. **Apply its verdict.** A change lands as a commit; an explanation lands as a reply on that thread.
4. **Repeat** until no thread is left, then run `komodo gate`.

## Rules

- **One thread per spawn.** A responder that sees two threads conflates them.
- **The reviewer is right until the code says otherwise.** Disagree once, with the line that proves it, then do it their way.
- **A reply cites the commit** that answers it, or it is not an answer.
- **Never resolve a thread you did not answer,** and never resolve one on the reviewer's behalf.
- **The gate is the exit.** An unresolved thread with a red gate is not done.
