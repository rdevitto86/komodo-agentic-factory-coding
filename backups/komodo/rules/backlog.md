## The grammar

`docs/backlog/` is the queue: a tree of epics, groups and tasks. One folder per epic, one folder per group inside it, one file per task. Humans and agents write it; the harness parses every file deterministically. Nothing else feeds the pipeline.

### Shape
````markdown
docs/backlog/epic-01/EPIC.md
## [EPIC-01] Epic title [READY]
```yaml
version: 1.4.0       # the version every group under this epic ships; the epic branch is feat/<version>
type: feat           # optional
groups_max: 6        # optional, default 6
```
One paragraph: the goal. It becomes the epic pull request's title and body.
docs/backlog/epic-01/tg-01.1/TG.md
## [TG-01.1] Group title [P: H] [READY]
```yaml
type: feat          # feat fix chore docs test refactor perf build ci (branch and commit type)
mode: parallel       # parallel or single; single runs the whole group on one machine
base: ""             # the branch this group cuts from; empty defaults to its epic's branch
depends_on: []       # groups whose unmerged branch this one's PR stacks on instead
```
docs/backlog/epic-01/tg-01.1/tsk-01.1.1.md
- [ ] **TSK-01.1.1** Task title
  - files: `internal/backlog/backlog.go`, `internal/backlog/backlog_test.go`
  - accept: a refund over the limit is refused
  - done_when: `go test ./internal/backlog/...`
  - owner: human                 # also context, depends_on, priority, status, tier, facets: see Fields
````

### Placement
- **Names are the lower-cased IDs** and must match the IDs inside. A group's number places it under its epic: `TG-15.3` lives in `epic-15`. A task's number places it in its group, and task files sort numerically.
- **`TG.md` carries no `version` and no `epic`;** both come from `EPIC.md`, and lint refuses them on a group. A blocker note sits after the group's yaml.
- **Caps.** A group holds 1 to 12 buildable tasks; its open tasks declare at most 20 unique files, a ticked task's files not counted. An epic holds at most `groups_max` groups. Lint refuses more and suggests a split.
- **A flat file under `docs/backlog/`** is a lint note and a doctor warning. `komodo migrate` moves a flat backlog, `BACKLOG.md` or TODO.md into the tree and leaves the source for a person to remove. The ship commit of an epic's last open group deletes `docs/backlog/epic-NN/`; never a backlog file on `main`, so an epic PR adds none.

### Fields
- **Priority** is `C`, `H`, `M`, or `L`. **Status** is `REFINEMENT`, `READY`, or `BLOCKED`, both on the group's own heading. A task's checkbox is its only status: the harness ticks it only once that task's checks pass.
- **`REFINEMENT`** is a group still being planned: the harness never runs it, and lint does not demand a task's `files`. Promote it to `READY` once every task names its files.
- **`version`** is required on every epic, as `x.y.z`, or `x.y.z-alpha.n`, `x.y.z-beta.n` or `x.y.z-rc.n` for a prerelease; rc is optional, and a release may go straight from beta to stable. It is the changelog heading a release writes and the tag `komodo tag` cuts, so the two can never drift. Every group under the epic ships it and cuts from `feat/<version>`.
- **`mode`** is `parallel` unless the group asks for `single`, one machine running the whole group instead of a session per task.
- **`base`** is the branch a group cuts from. Empty by default: a group then cuts from its epic's branch, `feat/<version>`, falling back to the remote's default branch while that epic branch is not yet cut.
- **`depends_on`** on a group is the groups whose unmerged branch this one's PR stacks on instead of its epic's. When a stacked parent merges, the child rebases onto the new base.
- **`files`** lists every path a task will create or edit, always including the caller that wires new code in, such as the command dispatch, the conductor, or a hook, not only the package it lives in; scope refuses any other. A task needs only a title and its `files`. Tasks that share no file run in parallel; a shared file, or a directory holding another task's path, serializes.
- **`accept`** is an optional line the correctness lens checks alongside the PRD, never a shell command.
- **`done_when`** are the shell commands whose zero exit proves the outcome through the real command, not only a unit test of the changed package; a `READY` agent task requires at least one. Never prose.
- **`owner`** is `agent` unless a task names `human`; a person's task stays in its own group instead of a whole group filed as `BLOCKED`.
- **`context`** lists paths, each with an optional `#anchor`, a builder reads before starting this task.
- **`depends_on`** on a task is the task ids that must be done before this one starts, narrower than the group's own `depends_on`.
- **`priority`** overrides the group's `[P: <letter>]` for one task, else it inherits the group's.
- **`status`** overrides the group's `[<STATUS>]` for one task, such as `BLOCKED` while a person acts, else it inherits the group's; a ticked checkbox always reads `DONE`.
- **`tier`** overrides the role's machine size for one task: `light`, `standard`, or `heavy`; an agent task may never be `light` (REQ-30).
- **`facets`** lists facet names this task adds to detection, beyond what the tree and the repo already override.

## Choosing a version

An epic's `version:` is the newest tag with one segment raised, never a bare guess.

- **Segment.** A breaking change bumps major; a feat bumps minor; anything else, fix included, bumps patch. Bump above the newest tag, not the epic's last version.
- **Phase.** Cut straight to `x.y.z` only when every group in the epic is `READY` and proven by its checks. Use `x.y.z-alpha.n` while the shape can still move, `x.y.z-beta.n` once feature-complete and only fixes land, and `x.y.z-rc.n` only when the owner asks.
- **Prerelease order.** A prerelease sorts before its release: `1.43.56-alpha.1` precedes `1.43.56`, so it is a valid `version:` only while `1.43.56` is untagged.
- **Raising a prerelease line.** A prerelease keeps its `x.y.z` and raises `n` for the next round; it never jumps to a new `x.y.z` until the stable cut.

The README's Versions section links here rather than copying it.

### Adding work
`komodo add EPIC-15 "Title" --version 1.0.0-beta.6` creates an epic; `komodo add TG-15.3 "Title"` a group under its epic; `komodo add TG-15.3 "Task title" --files a.go,a_test.go --done-when "go test ./..."` the next `tsk-` file. Never delete a ticked task; it stays as the record.
