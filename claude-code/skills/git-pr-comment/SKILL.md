---
name: git-pr-comment
description: Post a status/update comment to the current branch's own open PR — CI failure context, a re-review request, or a progress update — distinct from editing the PR's own description or label.
argument-hint: [comment text or context]
---

# Comment on own PR

Scoping: **$ARGUMENTS** (default: ask what to post)

**Scope: the current branch's own open PR only.** `git_guard.py` scopes `gh pr comment` (and `gh pr edit`) to the current branch's own PR number — this skill never posts to another repo's or another branch's PR. Branch/push/merge/protected-ref conventions live in `git-pr-create` — load it rather than restating them here.

Distinct from `git-pr-create`'s description/label edits: this is a comment appended to the PR's conversation, not a change to the PR itself. Post it for CI failure context, an update after addressing feedback, or a request for re-review — not for content that belongs in the description.

## Process

1. Identify the PR for the current branch (`gh pr view` or `gh pr status`); if none exists, say so and stop.
2. Draft the comment body from `$ARGUMENTS` or the current context (e.g. the failing check's output, what changed since the last review).
3. **Show the drafted comment to the user and get explicit confirmation before running `gh pr comment`.** Same shared-action rule `git-issue-create` follows — posting to a PR is outward-visible, reaching outside the local repo.
4. On confirmation, write the body to a temp file and run `gh pr comment <number> --body-file <path>` — never interpolate the body into an inline `--body "<text>"` string, since evidence pulled from CI output or a diff snippet routinely contains backticks, `$()`, or quotes that break or hijack the constructed shell command.
5. On decline or edit request, revise and re-confirm — never post a version the user hasn't seen.

## Output

The confirmed comment before posting; the comment URL after. Never both silently.
