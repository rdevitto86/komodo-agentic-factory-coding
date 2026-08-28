---
name: git-issue-review
description: Triage the repo's currently-open GitHub issues against BACKLOG.md and the codebase — flag one that's already resolved, a duplicate, or stale. Reports only; never closes or edits an issue.
argument-hint: []
---

# Git issue review

**Requires a GitHub remote.** If `gh repo view` fails (no remote, not a GitHub repo, `gh` not authenticated), say so and stop.

Triages `gh issue list`'s open issues against `BACKLOG.md` and the current codebase state. Reports only — this skill never calls `gh issue close`/`gh issue edit`. Closing or editing an issue is a human action, the same reasoning `git-pr-create` states for landing a PR through the merge button — issues just have no hook gate enforcing that, so the restraint here is a stated rule, not an enforced block.

## Process

1. `gh issue list` for the repo's open issues.
2. For each, check:
   - **Already resolved** — the code or `CHANGELOG.md` shows the described problem already fixed.
   - **Duplicate** — an open `BACKLOG.md` story already covers the same finding.
   - **Stale** — references code, a file, or a behavior that no longer exists.
3. Report each flagged issue with its number, title, and the reason (resolved/duplicate/stale), citing the `file:line` or `BACKLOG.md` line that supports the flag.

## Output

A flagged-issue report only. Never runs `gh issue close` or `gh issue edit` — closing or editing an issue is the user's call.
