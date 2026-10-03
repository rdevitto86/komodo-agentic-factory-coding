# Agent Rules

Rules for every model and tool on Komodo software.

## Working
- **Recommend before rewriting.** A patch, not a rewrite.
- **Read freely, write on a directive.** Read, search anytime; edit only on implement, fix, add, remove, or update; review, assess, consider stay analysis.
- **Assume and state it.** Ask only on a user-only decision, or before an irreversible or shared step.
- **Never widen scope.** Out-of-task work is a `komodo add` task in `docs/backlog/`, not a change.
- **Report honestly.** State any failure, skip, or gap, with evidence.
- **Verify the real source.** Read the file, manifest, or docs, never memory.

## Git
- **Inside your worktree you are free.** Reset, restore, rebase, or delete; keep a linked worktree detached.
- **Set work aside with a WIP commit, never `git stash`.** Every worktree and session shares one stash list; the sweep archives a week-old stash and drops it.
- **Never rewrite pushed history.** No force push, `--force-with-lease`, or `+refspec`; push a new commit.
- **Never touch a critical ref.** No commit, push, merge, delete, or force on `main`, `master`, or a `komodo/policy.json` ref; work on a `<type>/<kebab-name>` branch.
- **Never touch host or toolkit config.** Home dirs, machine overlay, `.git/config`, `.git/hooks`, toolkit binaries.
- **Never add a trailer or session link.** No co-author, generated-by, or session URL.
- **Open a pull request with `komodo pr create`,** never `gh pr create`; `komodo pr label` labels one opened elsewhere.
- **Landing is a person merging the epic PR.** Hand the user a refused command; after a merge, run `komodo sync`.
- **A working builder leases its branch.** Its push frees it, or 2 hours.
- Past a one-line fix, run: `/run <group>` in session, `komodo run <group>` headless.

## Comments
- A public function gets a one-line doc comment per its language; a private one only when long or non-obvious.
- At most twenty words, what the code does: never a restated name, version, ticket, first person, hedge, or history.
- The gate lints them with `komodo comments check`.

{{accessibility}}
