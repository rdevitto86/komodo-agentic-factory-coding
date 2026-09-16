---
name: backlog
description: The BACKLOG.md task grammar the harness parses, and how to add or change a task. Use before editing BACKLOG.md or when asked to add work.
paths: "**/BACKLOG.md"
---

# Backlog grammar

`BACKLOG.md` is the queue. Humans and agents write it; the harness parses it deterministically. Nothing else feeds the pipeline.

## Shape
````markdown
## [EPIC-01] Title
### [TG-01.1] Group title
```yaml
type: feat          # feat fix chore docs test refactor perf build ci (branch and commit type)
mode: parallel      # parallel (default) or single: one builder takes the whole group
```
#### [TSK-01.1.1] Task title [P: H] [TODO]
```yaml
files: [internal/refund/handler.go, internal/refund/handler_test.go]
done_when:
  - go test ./internal/refund/...
depends_on: [TSK-01.1.0]
context: [docs/spec/SDD.md#refunds]
owner: agent        # or human
type: feat
```
````

## Rules
- **Priority** is `C`, `H`, `M`, or `L`. **Status** is `TODO`, `IN_PROGRESS`, `BLOCKED`, or `DONE`. The harness rewrites only the status token.
- **`files`** lists every path the task will create or edit. Tasks in different directories run in parallel; same directory serializes.
- **`done_when`** is shell commands that exit zero when the task is done. Never prose. Cover the whole package, not one file.
- **`depends_on`** only when a later task cannot compile or test without an earlier one.
- **`context`** points at spec sections or docs a builder reads first, with an optional `#anchor`.
- A task the harness cannot run fails `python3 -m komodo tasks lint`; run it after every edit.

## Adding a task
From a session: append a heading and block in the shape above, at the end of its group, with the next free `TSK-` suffix. Or run:
```
python3 -m komodo tasks add TG-01.1 "Add refund metrics" --files internal/refund/metrics.go --done-when "go test ./internal/refund/..."
```
Never delete a `[DONE]` task by hand; `komodo run` moves finished work into the changelog on the next run.
