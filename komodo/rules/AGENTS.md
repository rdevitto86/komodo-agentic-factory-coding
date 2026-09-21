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
- Work on a `<type>/<kebab-name>` branch. Never commit to or push `main`, `master`, `trunk`, `prod`, `production`, `release/*`, or `hotfix/*`.
- Never force-push, rebase, amend, or rewrite published history. Never add a co-author or generated-by trailer.
- Landing into a protected branch is the human's merge button. Hand the user the command for anything refused.
- Move with `git switch`. `git checkout`, `git restore`, and `rm` are refused; each discards work that is not committed.
- For anything larger than a one-line fix in a repo with a `BACKLOG.md`, run the harness: `python3 -m komodo run <group>`. `python3 -m komodo --help` lists every command.

## Comments
- Every public function gets a one-line doc comment in the language's convention. A private function gets one only when it is long or non-obvious. Everything else is silent unless the line cannot say it itself.
- At most twenty words, what the code does. Never a restatement of the name, a version, a ticket, first person, a hedge, or history.
- `python3 -m komodo comments check` is the lint; the pre-commit hook runs it.

## Writing for a human (mandatory, every human-facing output)
- **Answer first.** The conclusion or outcome is the first line. Detail follows only if needed.
- **One idea per line.** Sentences under twenty words. A new sentence instead of a clause chain.
- **Caps:** five bullets per list, three sentences per paragraph, three options per question, three heading levels.
- **Bold the first words of a bullet.** Headings mark context boundaries. Code, commands, paths, and errors go in code blocks, never in prose.
- **No walls of text.** Over a cap means restructure, not shrink. No filler, no preamble, no closing summary.
- **Turn-end summary** when a turn changed something, exactly these headings in this order, omitting an empty bucket: `## ✅ Successful Changes`, `## ❌ Blocked Changes`, `## 📌 Callouts`, `## ⚠️ Warnings`. A callout is something worth knowing; a warning is something that went wrong.
- **Exempt only:** a worker returning JSON to the orchestrator.
