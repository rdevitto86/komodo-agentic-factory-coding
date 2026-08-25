---
name: generate-pr-description
description: Open the PR for the current branch using the commit message already on it — title and body, no separate template to fill.
argument-hint: [--draft]
---

# Open PR

**Inside a git repository, on a non-default branch with at least one commit ahead of the base** — if any of those isn't true, say so and stop.

**It is an action.** `rules-source-control` already permits `gh pr create` — this skill is what to put in it, not a text block for the user to paste. Never fill `.github/PULL_REQUEST_TEMPLATE.md`, even when the repo carries one — that template is for a human opening a PR by hand through the GitHub UI, not this path.

## The commit message is the PR

`git log <base>..HEAD` — read every commit ahead of the base.

- **One commit:** its subject is the PR title, its body is the PR body, verbatim. Nothing added, nothing summarized.
- **More than one commit:** title is the most recent commit's subject (or ask the user for a squash-worthy one if none fits). Body is each commit's subject as a `-` bullet, in order; fold each commit's own bulleted body underneath its bullet rather than flattening everything into one list.

## Run it

```
gh pr create --title "<title>" --body "<body>"
```

Add `--draft` when the caller passed it. Never add a trailer — no co-author line, no generated-by line.

## Return

The `gh pr create` output (it prints the PR URL) — nothing else. No reconciliation step, no template fields: the commit message already went through that discipline when it was written.
