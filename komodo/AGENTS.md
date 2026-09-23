# Agent Rules

Universal rules for every model and tool on Komodo software, hosted or local, plain markdown.

## Working
- **Recommend before rewriting.** A patch, not a rewrite.
- **Read freely, write on a directive.** Read, search anytime; edit only on implement, fix, add, remove, or update; review, assess, consider stay analysis.
- **Assume and state it.** Ask only on a user-only decision, or before an irreversible or shared step.
- **Never widen scope.** Out-of-task work is a `BACKLOG.md` line, not a change.
- **Report honestly.** State any failure, skip, or gap plainly, with evidence.
- **Verify the real source.** Read the file, manifest, or docs; never rely on memory for a limit.
- **Suggested languages; existing code keeps its own.** Zig embedded, C++ robotics and modules, Rust routers and nodes, Go web and cloud, Python AI/ML, TypeScript with Vue or Svelte for UIs; C only when a vendor SDK or a hot path forces it.

## Git
- **Inside your worktree you are free.** Delete, reset, checkout, restore, rebase, or delete your branches; nothing is precious.
- **Never rewrite pushed history.** No force push, `--force-with-lease`, or `+refspec` on any branch; push a new commit.
- **Never touch a critical ref.** No commit, push, merge, delete, or force on `main`, `master`, or a `komodo/policy.json` ref; work on a `<type>/<kebab-name>` branch.
- **Never leave the worktree.** No edit, write, delete, or move outside its root, or on host/toolkit config: home dirs, machine overlay, `.git/config`, `.git/hooks`, and toolkit binaries.
- **Never add a trailer.** No co-author, no generated-by line.
- **Landing is the human's merge button.** Hand the user any refused command.
- Past a one-line fix, run the line: `/run <group>` in session, `komodo run <group>` headless; `komodo --help` lists commands.

## Comments
- A public function gets a one-line doc comment per its language; a private one only when long or non-obvious.
- At most twenty words, what the code does: never a restated name, version, ticket, first person, hedge, or history.
- `komodo comments check` is the lint; the gate runs it before every commit.

{{accessibility}}
