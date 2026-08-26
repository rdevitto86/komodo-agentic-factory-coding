---
name: generate-pr-description
description: Open the PR for the current branch, body filled from the real diff against the repo's own PR template — not the CLI, not a text dump.
argument-hint: [--draft]
---

# Open PR

**Inside a git repository, on a non-default branch with at least one commit ahead of the base** — if any of those isn't true, say so and stop.

**It is an action, not a text dump.** `rules-source-control` already permits `gh pr create` — this skill runs it directly. Never print the filled body to the terminal for the user to paste; the terminal is not where a PR description belongs.

## Fill the template, not the commit log

Read every commit ahead of the base (`git log <base>..HEAD`) and the full diff (`git diff <base>..HEAD`) — the body describes the *change*, not the commit history.

- **Repo has `.github/PULL_REQUEST_TEMPLATE.md`:** fill it section-for-section as written on disk — do not paraphrase the template's own headings or drop a section. Populate:
  - `Depends on`: the PR # this depends on, or `none`.
  - `Summary`: one or two sentences on what the PR is for. Never a file list.
  - `Changes`: one bullet per area (not per file), `**<area>** — <what changed>`, with the PRD requirement ID appended in parens where the story carried one.
  - `Validation Evidence`: only what a green CI run can't show — a live-dependency happy path, a cURL against STG, a behavior with no automated coverage yet. `Covered by CI` is a complete answer; never paste unit/component/contract results here.
  - Leave the template's HTML comments in place — they're instructions to a human filling the form by hand, and this is filling the same form, just automated. Strip a comment only if the repo's own template already omits it.
- **Repo has no PR template:** fall back to a `## Summary` + `## Changes` body in the same shape (a sentence, then bulleted areas) — never dump raw commit subjects as the body.

Title: `<type>: <summary>` — the same convention `generate-commit-message` uses, max 72 chars, imperative, no trailing period. Use the most recent commit's subject if it already fits; otherwise write one that names the overall change.

## Run it

```
gh pr create --title "<title>" --body "<filled body>"
```

Add `--draft` when the caller passed it. Never add a trailer — no co-author line, no generated-by line.

## Return

The `gh pr create` output (it prints the PR URL) — nothing else.
