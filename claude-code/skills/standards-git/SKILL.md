---
name: standards-git
description: Git branch, push, merge, and protected-ref conventions — what the conventions ARE, not their enforcement (see rules-source-control) or commit message format (see write-commit-message). Load before running or advising on any git operation.
user-invocable: false
---

# Git conventions

This is domain knowledge — the shape a branch, a push, or a merge is supposed to take. `rules-source-control` is the enforcement layer: what `git_guard.py` actually blocks and how to avoid routing around it. `write-commit-message` owns commit message format. Load whichever of the three answers the question at hand; this skill never restates their content.

## Protected refs

`main`, `master`, `trunk`, `prod`, `production`, `release/*`, and `hotfix/*` are protected. Nothing commits or pushes to one directly, and nothing lands a branch into one except through the PR's merge button — that step belongs to the user, never to the agent, regardless of what capability is otherwise available.

## Branch naming

`<type>/<short-kebab-description>` — the same `type` taxonomy `write-commit-message` uses for commit prefixes: `feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`. Create the branch before the first commit, not after — work should never land on a protected ref and then need to be moved.

**Resuming a `[WIP]` band reuses its existing branch**, never a second branch cut for work already in flight. Match the branch name to the story already underway, not to whatever happens to be checked out.

## Commits

Message format — subject line, body, trailers — is owned entirely by `write-commit-message`; this skill states no format of its own. The one convention that lives here rather than there: a commit never carries `--amend`, never skips hooks with `--no-verify`, and never adds a co-author or generated-by trailer, on any branch, under any circumstance.

## Push

Push names its remote and branch explicitly — `origin <branch>` — never a bare push, since the destination has to be readable from the command itself. A force-push (in any of its flag spellings) is never part of the normal flow; if one is truly needed, that decision and that command belong to the user, not to an agent acting on their behalf.

## Merging

Only one direction is ever a convention here: the protected base merges into your branch, to pull in its latest changes and surface conflicts early — never your branch into the base. A strategy flag that auto-resolves a conflict without ever showing it defeats the point of merging early, so the convention is always to resolve conflicts by hand, one file at a time, and commit the result with the merge's own default message.

Landing a branch into its protected base is a human action end to end, taken through the PR's merge button (or the user's own CLI) — never a step this convention set describes an agent taking.
