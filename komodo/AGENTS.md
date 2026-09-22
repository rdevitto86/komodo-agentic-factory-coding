# Agent Rules

Universal rules for every model and tool working on Komodo software, hosted or local. Plain markdown, no platform syntax.

## Working
- **Recommend before rewriting.** A patch or a snippet, not a wholesale redo.
- **Read freely, write on a directive.** Search and read without asking. Edit only on an instruction with a directive verb (implement, fix, add, remove, update). Review, assess, and consider mean analysis only.
- **Assume and state it.** Ask only when blocked on a decision only the user can make, or before an irreversible or shared action.
- **Never widen scope.** Out-of-task work is one line in `BACKLOG.md`, not a change. Touch only the lines a task needs.
- **Report honestly.** A failure, a skipped step, or an unfinished part is stated plainly with its evidence.
- **Verify the real source.** Never design around a limit from memory; read the file, the manifest, or the docs.

## Git
- **Inside your worktree you are free.** Delete files, reset, checkout, restore, force-push your own branch, delete your own branches. Nothing there is precious and nothing else exists.
- **Never touch a critical ref.** No commit, push, merge, delete, or force on `main`, `master`, or any ref `komodo/policy.json` lists. Work on a `<type>/<kebab-name>` branch.
- **Never leave the worktree.** No edit, write, delete, or move on a path outside its root, and none at all on a host or toolkit config: the host home directories, the machine overlay, `.git/config`, `.git/hooks`, and the toolkit's binaries.
- **Never add a trailer.** No co-author and no generated-by line in a commit message.
- **Landing is the human's merge button.** Hand the user the command for anything refused.
- For anything larger than a one-line fix in a repo with a `BACKLOG.md`, run the line: `/run <group>` in a session, or `komodo run <group>` headless. `komodo --help` lists every command.

## Comments
- Every public function gets a one-line doc comment in the language's convention. A private function gets one only when it is long or non-obvious. Everything else is silent unless the line cannot say it itself.
- At most twenty words, what the code does. Never a restatement of the name, a version, a ticket, first person, a hedge, or history.
- `komodo comments check` is the lint; the gate runs it before every commit.

{{accessibility}}
