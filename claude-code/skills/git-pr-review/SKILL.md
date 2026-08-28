---
name: git-pr-review
description: Review the current branch's own open PR for merge-readiness — description completeness, label correctness, CI status, and reviewer response. Never approves, requests changes on, or merges anyone's PR.
argument-hint: []
---

# Review own PR

**Scope: the current branch's own open PR only.** `git_guard.py` denies `gh pr review`, `gh pr merge`, and `gh pr close` outright — this skill never attempts any of them, on this PR or anyone else's. `gh pr edit`/`gh pr comment` are allowed but scoped by the guard to the current branch's own PR number, so this skill works only within that scope — it never reviews another repo's or another branch's PR. Branch/push/merge/protected-ref conventions live in `git-pr-create` — load it rather than restating them here.

## What to check

1. **Find the PR** — `gh pr view` (or `gh pr status`) for the current branch. If there is none, say so and stop; this skill doesn't open one — `git-pr-create` does.
2. **Description completeness** — diff the PR body against the repo's own `.github/PULL_REQUEST_TEMPLATE.md`, section-for-section (the same template `git-pr-create` fills); flag a section missing or left as unfilled boilerplate.
3. **Label correctness** — the same category-label mapping `git-pr-create` uses (Skill / Documentation / Bug / Enhancement, by diff content and commit-type prefix); flag a wrong or missing label.
4. **CI/check status** — `gh pr checks` for pass/fail/pending; report which check, if any, is blocking.
5. **Reviewer response** — `gh pr view --json reviews,reviewRequests` to see whether requested reviewers have responded; report who's still pending.

## Output

A merge-readiness report: description status, label status, check status, reviewer status, and one verdict — ready or not, naming what's missing if not. Never runs `gh pr merge`/`close`/`review` — landing or approving the PR is the user's call once this reports ready.
