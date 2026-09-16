---
type: llm
weight: 1
---

A correct response reshapes the messy list into `BACKLOG.md`'s fixed four-level hierarchy (epic -> task group -> task -> subtask) using bracketed IDs (`EPIC-XX`, `TG-XX.Y`, `TSK-XX.Y.Z`), a priority tag (`[P: C/H/M/L]`) and a status indicator (`[TODO]`/`[IN_PROGRESS]`/`[BLOCKED]`/`[DONE]`) on every task line, plus a `## Convention Legend` block. "High priority, not started" becomes `[P: H] [TODO]`; "medium priority, in progress" becomes `[P: M] [IN_PROGRESS]`; "done already" becomes `[DONE]`.

A response that keeps a flat bullet list, invents a different ID scheme, drops the status/priority brackets, or turns the already-`[DONE]` refactor task into a still-open one fails this grader.
