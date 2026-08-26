---
name: rules-source-control
description: Git guardrails beyond what the write guard blocks — commit, push, branch, publish, and worktree conventions. Load before running or advising on any git operation.
user-invocable: false
---

# Source Control Rules

`git_guard.py` lets you branch, add, commit, push, stash, and open/update a PR — never on `main`/`master`/`trunk`/`prod`/`production`/`release/*`/`hotfix/*`, and never a merge, rebase, pull, checkout, restore, reset, revert, cherry-pick, force-push, `--amend`, `--no-verify`, `--no-gpg-sign`, or a commit trailer. `PUBLISH_ENABLED=0` in the environment reverts all of that to a blanket deny with no source edit. This skill is what to do with the capability, not a second copy of the block.

## Never route around the block

No `sh -c` / `bash -c` wrapper, no shell alias, no editing files under `.git/` with Edit or Write, no `GIT_EDITOR` trick, no piping a denied command through `tee` or a heredoc to dodge the scan. A protected branch, a merge, a rebase, a force-push — if it's denied, it's denied; hand the user the exact command instead of retrying a disguised version of the same call.

Read-only inspection is unrestricted: `status`, `diff`, `log`, `show`, `blame`, `rev-parse`, `ls-files`, `fetch`, plain `git branch`/`git stash` listing. Use it freely.

## Branches

Create one before committing anything: `git switch -c <type>/<short-kebab-description>`, the same `type` taxonomy `generate-commit-message` uses (`feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`). `git_guard.py` enforces the shape and rejects a protected name outright — a badly-named branch never reaches disk, so there's nothing to catch after the fact.

**Resuming a `[WIP]` band reuses its existing branch** — `git switch <existing-branch>`, never a second `-c` for work already in flight. Match the branch name to the WIP story, not to whatever's currently checked out.

## Commits

Message format is owned by `generate-commit-message` — don't restate it here; load that skill whenever a message is needed, then `git add` + `git commit -m "<message>"` directly. Never `--amend`, never `--no-verify`, never a co-author or generated-by trailer — the guard rejects all three, but don't rely on that; write it right the first time.

Committing on a protected branch is always denied, regardless of `PUBLISH_ENABLED` — there is no "temporarily on main" case that makes it safe.

## Push

`git push -u origin <branch>` on your own non-protected branch is allowed. Force-push (`-f`/`--force`/`--force-with-lease`/`--force-if-includes`) is always denied — never suggest it, even if the user names it first; tell them to run it themselves if they truly want it. Pushing directly to a protected branch is denied the same as committing to one.

## Opening the PR

`generate-pr` runs `gh pr create`/`gh pr edit` directly once the branch is pushed — `workflow-loop`'s P4 (Publish) is the normal call site. `git_guard.py`'s `gh` allowlist scopes `pr edit`/`pr comment` to the current branch's own PR number; it cannot reach or touch anyone else's.

## Merging

Never. `git merge`, `git rebase`, `git pull` (fast-forward or not) all stay denied regardless of `PUBLISH_ENABLED` — landing a branch into `main` is the one step that stays entirely the user's, whether through the PR's merge button or their own CLI.

## Stashing

`git stash push -u`/`pop`/`apply` are allowed; `drop`/`clear` are denied (discarding saved state outright is not part of the publish capability). Use `push -u` (the `-u` catches untracked files) when uncommitted work is in the way of something else you need to do, never to dodge writing a real commit.

## Worktrees

Raw `git worktree` is blocked outright. For session-level isolation, use the `EnterWorktree`/`ExitWorktree` tools — but only when the user or project instructions explicitly say "worktree"; never reach for it on your own initiative. For parallel subagents writing to the same checkout, use `Agent`'s or `Workflow`'s `isolation: "worktree"` option instead of asking the user to hand-create one.
