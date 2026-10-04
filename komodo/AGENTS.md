# Agent Rules

Rules for every model and tool on Komodo software.

## Working
- **Act, then report.** Do anything reversible in your own branch or worktree unasked; stop only for a critical ref, remote delete, external post, or spending.
- **A directive is a change; a question is analysis.** Implement, fix, add, remove, update: change code. Review, assess, consider: give findings and the change you recommend.
- **Assume and state it.** Pick the sensible default, name it, and keep going; ask only for a decision that is the user's alone.
- **Disagree once, with evidence, then do it their way.**
- **Patch; keep scope.** The smallest change that does the job; out-of-task work is a `komodo add` task in `docs/backlog/`.
- **Report verdicts.** Say what passed, failed, or was skipped, with evidence; no "might" or "probably".
- **Verify the real source.** Read the file, manifest, or docs, not memory.

## Git
- **Inside your worktree you are free.** Reset, restore, rebase, or delete; keep a linked worktree detached.
- **Set work aside with a WIP commit, not `git stash`.** Every worktree and session shares one stash list.
- **Push new commits; pushed history is fixed.** No force push, `--force-with-lease`, or `+refspec`.
- **Critical refs belong to people.** Work on a `<type>/<kebab-name>` branch; no commit, push, merge, delete, or force on `main`, `master`, or a `komodo/policy.json` ref.
- **Leave host and toolkit config alone.** Home dirs, machine overlay, `.git/config`, `.git/hooks`, toolkit binaries.
- **Commits and pull requests carry no trailer or session link.** No co-author, generated-by, or session URL.
- **Open a pull request with `komodo pr create`.**
- **Landing is a person merging the epic PR.** Hand the user a refused command; after a merge, run `komodo sync`.
- **A working builder leases its branch.** Its push frees it, or 2 hours.
- Past a one-line fix, run: `/run <group>` in session, `komodo run <group>` headless.

## Comments
- A public function gets a one-line doc comment per its language; a private one only when long or non-obvious.
- At most twenty words, what the code does: not a restated name, version, ticket, first person, hedge, or history.

{{accessibility}}
