---
name: rules-source-control
description: Git guardrails beyond what the write guard blocks — commit, push, branch, stash, and worktree conventions. Load before running or advising on any git operation.
user-invocable: false
---

# Source Control Rules

`git_guard.py` and `settings.json`'s `permissions.deny` already stop you from running any git command that changes repository state — commit, push, branch, checkout, switch, restore, reset, revert, merge, rebase, cherry-pick, stash (anything but `list`/`show`), clean, rm, mv, apply, worktree, tag, am. Only the user runs those. This skill is what to do instead, not a second copy of the block.

## Never route around the block

No `sh -c` / `bash -c` wrapper, no shell alias, no `--no-verify` or `--no-gpg-sign`, no editing files under `.git/` with Edit or Write, no `GIT_EDITOR` trick, no piping a mutating command through `tee` or a heredoc to dodge the scan. If a command is denied, it is denied — hand the user the exact command instead of retrying a disguised version of the same call.

Read-only inspection is unrestricted: `status`, `diff`, `log`, `show`, `blame`, `rev-parse`, `ls-files`, `fetch`. Use it freely to gather context before handing off a mutating command.

## Commits

You never stage or commit. Message format is owned by `generate-commit-message` — don't restate it here; load that skill (directly, or via `workflow-complete` at task/phase end) whenever a message is needed. If the user asks you to run `git commit` directly, give them the exact command instead of attempting it.

## Push

You never push. Never suggest `--force` or `--force-with-lease` unless the user names it first — even then, state in one line what it overwrites (their own unpushed work, or a shared branch's history) before handing over the command.

## Branches

Name a branch you're proposing as `<type>/<short-kebab-description>`, using the same `type` taxonomy `generate-commit-message` uses for commits (`feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`). Never propose committing directly to `main` or another default branch for anything beyond a trivial one-line fix — hand over a branch-creation command first.

## Stashing

You cannot run `git stash push` — it's blocked like every other mutation. When uncommitted work sits in the way of a destructive command the user is about to run (checkout, reset, restore, clean), say so and hand them `git stash push -u` (the `-u` catches untracked files) or tell them to commit first. Your job is flagging the risk, never performing the stash.

## Worktrees

Raw `git worktree` is blocked outright. For session-level isolation, use the `EnterWorktree`/`ExitWorktree` tools — but only when the user or project instructions explicitly say "worktree"; never reach for it on your own initiative. For parallel subagents writing to the same checkout, use `Agent`'s or `Workflow`'s `isolation: "worktree"` option instead of asking the user to hand-create one.
