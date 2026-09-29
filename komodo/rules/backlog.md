## The grammar

`docs/backlog/` is the queue: one file per task group, named `<group-id>-<slug>.md`. Humans and agents write it; the harness parses each file deterministically. Nothing else feeds the pipeline.

### Shape
````markdown
## [TG-01.1] Group title [P: H] [READY]

```yaml
type: feat          # feat fix chore docs test refactor perf build ci (branch and commit type)
version: 1.4.0      # the version this group ships, and picks the epic branch it cuts from
epic: EPIC-01        # the epic this group's file is removed alongside
base: ""             # the branch this group cuts from; empty defaults to its epic's branch
depends_on: []       # groups whose unmerged branch this one's PR stacks on instead
```

- [ ] **TSK-01.1.1** Task title
  - files: `internal/backlog/backlog.go`, `internal/backlog/backlog_test.go`
  - accept: a refund over the limit is refused
  - done_when: `go test ./internal/backlog/...`
  - owner: human
  - context: `docs/prd.md#refunds`, `internal/backlog/groupfile.go`
  - depends_on: TSK-01.1.0
  - priority: C
  - status: BLOCKED
````

### Fields
- **Priority** is `C`, `H`, `M`, or `L`. **Status** is `REFINEMENT`, `READY`, or `BLOCKED`, both on the group's own heading. A task's checkbox is its only status: the harness ticks it only once that task's checks pass.
- **`REFINEMENT`** is a group still being planned: the harness never runs it, and lint does not demand a task's `files`. Promote it to `READY` once every task names its files.
- **`version`** is required on every group, as `x.y.z`, or `x.y.z-alpha.n`, `x.y.z-beta.n` or `x.y.z-rc.n` for a prerelease; rc is optional, and a release may go straight from beta to stable. It is the changelog version ship writes a fragment for and the tag `komodo tag` cuts, so the two can never drift. Groups shipping together share one version, and a group's version picks the branch it cuts from: `feat/<version>`.
- **`epic`** is the `EPIC-` id the group's file carries. The PR of an epic's last open group deletes every group file that shares its epic; a group with no epic deletes its own file.
- **`base`** is the branch a group cuts from. Empty by default: a group then cuts from its epic's branch, `feat/<version>`, falling back to the remote's default branch while that epic branch is not yet cut.
- **`depends_on`** on a group is the groups whose unmerged branch this one's PR stacks on instead of its epic's. When a stacked parent merges, the child rebases onto the new base.
- **`files`** lists every path a task will create or edit, always including the caller that wires new code in — the command dispatch, the conductor, or a hook — not only the package it lives in; scope refuses any other. A task needs only a title and its `files`. Tasks that share no file run in parallel; a shared file, or a directory holding another task's path, serializes.
- **`accept`** is an optional line the correctness lens checks alongside the PRD, never a shell command.
- **`done_when`** are the shell commands whose zero exit proves the outcome through the real command, not only a unit test of the changed package; a `READY` agent task requires at least one. Never prose.
- **`owner`** is `agent` unless a task names `human`; a person's task stays in its own group instead of a whole group filed as `BLOCKED`.
- **`context`** lists paths, each with an optional `#anchor`, a builder reads before starting this task.
- **`depends_on`** on a task is the task ids that must be done before this one starts, narrower than the group's own `depends_on`.
- **`priority`** overrides the group's `[P: <letter>]` for one task, else it inherits the group's.
- **`status`** overrides the group's `[<STATUS>]` for one task, such as `BLOCKED` while a person acts, else it inherits the group's; a ticked checkbox always reads `DONE`.
- A group holds 1 to 12 tasks; lint refuses a larger one and suggests a split.

## Choosing a version

A group's `version:` is the newest tag with one segment raised, never a bare guess.

- **Segment.** A breaking change bumps major; a feat bumps minor; anything else, fix included, bumps patch. Bump above the newest tag, not the epic's last version.
- **Phase.** Cut straight to `x.y.z` only when every group in the epic is `READY` and proven by its checks. Use `x.y.z-alpha.n` while the shape can still move, `x.y.z-beta.n` once feature-complete and only fixes land, and `x.y.z-rc.n` only when the owner asks.
- **Prerelease order.** A prerelease sorts before its release: `1.43.56-alpha.1` precedes `1.43.56`, so it is a valid `version:` only while `1.43.56` is untagged.
- **Raising a prerelease line.** A prerelease keeps its `x.y.z` and raises `n` for the next round; it never jumps to a new `x.y.z` until the stable cut.

The README's Versions section links here rather than copying it.

### Adding a task
Append a checkbox in the shape above, at the end of its group file, with the next free `TSK-` suffix. Or run:
```
komodo add TG-01.1 "Add refund metrics"
```
Never delete a ticked task; it stays as the record, and ship writes its group's changelog line.
