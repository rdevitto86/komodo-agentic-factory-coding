---
name: rules-source-control
description: Git guardrails beyond what the write guard blocks — commit, push, branch, stash, and worktree conventions. Load before running or advising on any git operation.
user-invocable: false
---

# Source Control Rules

`git_guard.py` decides every git command deterministically. You may stage, commit, create a `<type>/<kebab>` branch, switch between branches, push that branch, and open a pull request. You may never reach a protected ref (`main`, `master`, `trunk`, `prod`, `production`, `release/*`, `hotfix/*`), rewrite history, or merge. Still denied outright: checkout, restore, reset, revert, rebase, cherry-pick, clean, rm, mv, apply, worktree, tag, am, and `gh pr merge`/`close`/`review`/`release`.

**The capability ships behind a single switch.** `PUBLISH_ENABLED` at the top of `git_guard.py` returns every verb above to a blanket deny when flipped to `False`. If a command below is refused and the guard's reason reads "changes repository state", that switch is off — say so rather than working around it.

## Never route around the block

No `sh -c` / `bash -c` wrapper, no shell alias, no `--no-verify` or `--no-gpg-sign`, no editing files under `.git/` with Edit or Write, no `GIT_EDITOR` trick, no piping a mutating command through `tee` or a heredoc to dodge the scan. If a command is denied, it is denied — hand the user the exact command instead of retrying a disguised version of the same call.

Read-only inspection is unrestricted: `status`, `diff`, `log`, `show`, `blame`, `rev-parse`, `ls-files`, `fetch`. Use it freely to gather context before handing off a mutating command.

## Commits

Message format is owned by `generate-commit-message` — don't restate it here; load that skill whenever a message is needed. **Never add a `Co-Authored-By` or generated-by trailer** — the guard rejects the commit outright, and a harness default that wants one does not override this.

**Never pass `--no-verify` or `--amend`.** The first skips the pre-commit gate, the second rewrites a commit; both are denied. A failing pre-commit hook is fixed at its cause, never bypassed.

## Push

Push your own branch with an explicit remote and refspec — `git push -u origin <branch>`. A bare `git push` is denied because the destination isn't readable from the command. Never suggest `--force` or `--force-with-lease` unless the user names it first — even then, state in one line what it overwrites before handing over the command.

## Branches

Name a branch `<type>/<short-kebab-description>`, using the same `type` taxonomy `generate-commit-message` uses for commits (`feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`). The guard enforces that shape, so an off-taxonomy name is refused rather than corrected.

**Branch before the first commit, not after.** Landing work on a protected ref and then trying to move it is the case this rule exists to prevent.

## Recovery

A stale branch updates with `git merge --ff-only` or `git pull --ff-only`; anything needing a real merge or a rebase is the user's. `git stash push -u` and `git stash pop` are available for moving uncommitted work out of the way — `drop` and `clear` are not.

**A conflict is a stop, not a puzzle.** Say what conflicts, hand over the command, and wait.

## Worktrees

Raw `git worktree` is blocked outright. For session-level isolation, use the `EnterWorktree`/`ExitWorktree` tools — but only when the user or project instructions explicitly say "worktree"; never reach for it on your own initiative. For parallel subagents writing to the same checkout, use `Agent`'s or `Workflow`'s `isolation: "worktree"` option instead of asking the user to hand-create one.
