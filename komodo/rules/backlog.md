## The grammar

`BACKLOG.md` is the queue. Humans and agents write it; the harness parses it deterministically. Nothing else feeds the pipeline.

### Shape
````markdown
## [EPIC-01] Title
### [TG-01.1] Group title
```yaml
type: feat          # feat fix chore docs test refactor perf build ci (branch and commit type)
version: 1.4.0      # the version this group ships; ship files the group's changelog line under it
mode: parallel      # parallel (default) or single: one builder takes the whole group
base: main          # the branch this group is cut from; omit for the remote's default branch
depends_on: [TG-01.0]  # groups whose branch base may name
```
#### [TSK-01.1.1] Task title [P: H] [READY]
```yaml
files: [internal/refund/handler.go, internal/refund/handler_test.go]
done_when:
  - go test ./internal/refund/...
depends_on: [TSK-01.1.0]
context: [docs/system-design.md#interfaces]
owner: agent        # or human
type: feat
tier: heavy         # light, standard, or heavy; overrides the role's tier for this task
facets: [postgres]  # facet names this task adds to detection
```
````

### Fields
- **Priority** is `C`, `H`, `M`, or `L`. **Status** is `REFINEMENT`, `READY`, `IN_PROGRESS`, `BLOCKED`, or `DONE`. The harness rewrites only the status token, and appends the finding tasks it files.
- **`REFINEMENT`** is a task still being planned: the harness never runs it, never picks its group, and lint does not demand `files` or `done_when`. Promote it to `READY` once both are real.
- **`version`** is required on every group, as `x.y.z`, or `x.y.z-alpha.n` or `x.y.z-beta.n` for a prerelease. It is the changelog version ship writes a fragment for and the tag `komodo tag` cuts, so the two can never drift. Groups shipping together share one version.
- **`base`** is the branch a group cuts from. Omit it for the remote's default branch, write `main`, or name the branch of a group listed in this group's `depends_on`; any other value fails `komodo lint`.
- **`files`** lists every path the task will create or edit. Tasks that share no file run in parallel; a shared file, or a directory holding another task's path, serializes.
- **`done_when`** is shell commands that exit zero when the task is done. Never prose. Cover the whole package, not one file.
- **`depends_on`** on a task, only when a later task cannot compile or test without an earlier one. On a group, the groups whose branch this group's `base` may stack on.
- **`context`** points at spec sections or docs a builder reads first, with an optional `#anchor`.
- **`tier`** is `light`, `standard`, or `heavy`; it overrides the role's tier for this task alone. Omit it to use the role's own tier.
- **`facets`** lists facet names this task adds, beyond what detection and `.komodo/facets` already select.
- A task the harness cannot run fails `komodo lint`; run it after every edit.

### Adding a task
Append a heading and block in the shape above, at the end of its group, with the next free `TSK-` suffix. Or run:
```
komodo add TG-01.1 "Add refund metrics"
```
Never delete a `[DONE]` task; it stays as the record, and ship writes its group's changelog line.
