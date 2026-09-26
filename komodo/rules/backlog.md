## The grammar

`docs/backlog/` is the queue: one file per task group, named `<group-id>-<slug>.md`. Humans and agents write it; the harness parses each file deterministically. Nothing else feeds the pipeline.

### Shape
````markdown
## [TG-01.1] Group title [P: H] [READY]

```yaml
type: feat          # feat fix chore docs test refactor perf build ci (branch and commit type)
version: 1.4.0      # the version this group ships; ship files the group's changelog line under it
epic: EPIC-01        # the epic this group's file is removed alongside
depends_on: []       # groups whose unmerged branch this one's PR stacks on; empty for the default branch
```

- [ ] **TSK-01.1.1** Task title
  - files: `internal/backlog/backlog.go`, `internal/backlog/backlog_test.go`
  - accept: a refund over the limit is refused
  - checks: `go test ./internal/backlog/...`
````

### Fields
- **Priority** is `C`, `H`, `M`, or `L`. **Status** is `REFINEMENT`, `READY`, or `BLOCKED`, both on the group's own heading. A task's checkbox is its only status: the harness ticks it only once that task's checks pass.
- **`REFINEMENT`** is a group still being planned: the harness never runs it, and lint does not demand a task's `files`. Promote it to `READY` once every task names its files.
- **`version`** is required on every group, as `x.y.z`, or `x.y.z-alpha.n` or `x.y.z-beta.n` for a prerelease. It is the changelog version ship writes a fragment for and the tag `komodo tag` cuts, so the two can never drift. Groups shipping together share one version.
- **`epic`** is the `EPIC-` id the group's file carries. The PR of an epic's last open group deletes every group file that shares its epic; a group with no epic deletes its own file.
- **`depends_on`** on a group is the groups whose unmerged branch this one's PR stacks on; empty targets the remote's default branch. When a stacked parent merges, the child rebases onto the new base.
- **`files`** lists every path a task will create or edit. A task needs only a title and its `files`. Tasks that share no file run in parallel; a shared file, or a directory holding another task's path, serializes.
- **`accept`** is an optional line the correctness lens checks alongside the PRD, never a shell command.
- **`checks`** are optional shell commands that add to the ones derived per detected language. Never prose.
- A group holds 1 to 12 tasks; lint refuses a larger one and suggests a split.

### Adding a task
Append a checkbox in the shape above, at the end of its group file, with the next free `TSK-` suffix. Or run:
```
komodo add TG-01.1 "Add refund metrics"
```
Never delete a ticked task; it stays as the record, and ship writes its group's changelog line.
