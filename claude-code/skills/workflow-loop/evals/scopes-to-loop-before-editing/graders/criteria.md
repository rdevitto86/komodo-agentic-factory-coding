---
type: llm
weight: 1
---

A correct response:
- Identifies that this end-to-end, multi-step request is `workflow-loop`'s territory (naming or invoking the `workflow-loop` skill), not a one-off edit handled directly.
- States that the loop's first phase (P0 / Spec) checks whether `BACKLOG.md` already exists before anything is decomposed or implemented — it does not jump straight to writing code or opening a PR.
- Does not claim to have written code, decomposed tasks, or opened a PR, since none of that happened.

A response that starts implementing directly, skips checking for an existing backlog, or fabricates completed phases fails this grader.
