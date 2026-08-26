---
name: generate-pr
description: Open or update the pull request for the current branch directly via gh — title, body, and a label picked from the diff.
argument-hint: [--draft]
---

# Open PR

**Inside a git repository, on a non-protected branch with at least one commit ahead of its base** — if any of those isn't true, say so and stop. `git_guard.py` only ever lets this run against the current branch's own PR — it has no way to touch anyone else's.

**It runs `gh pr create`/`gh pr edit`.** `rules-source-control` is what makes that call available; this skill is what fills it in. Never print the body to the terminal for the user to paste — the terminal isn't where a PR description lives.

## Fill the body from the diff, not the commit log

Read every commit ahead of the base (`git log <base>..HEAD`) and the full diff (`git diff <base>..HEAD`). The body describes the *change*, not a list of commit subjects.

**Repo has `.github/PULL_REQUEST_TEMPLATE.md`:** fill it section-for-section as written on disk, dropping nothing. **No template:** use this shape instead —

- `## Summary` — one or two sentences on what the PR is for, never a file list.
- `## Changes` — one bullet per area, not per file: `**<area>** — <what changed>`.
- `## Validation Evidence` — only what CI can't already show: a live-dependency happy path, a behavior with no automated coverage. `Covered by CI` is a complete answer on its own.

Title: `<type>: <summary>` — `generate-commit-message`'s own convention, max 72 chars, imperative, no trailing period. Reuse the branch's own commit subject if one already fits.

## Label it

**Always run `gh label list` first** — every label applied has to come from its actual output; never invent a name, never create one (`git_guard.py`'s `gh` allowlist doesn't grant `label create` for exactly this reason: this skill has no legitimate use for it).

Two independent dimensions, not one label:

- **Authorship, if the repo tracks it.** This skill only ever runs agent-side, so if the label set has an `agent pr` (or equivalently-named "opened by an agent") label, always add it — it's a fact about who's calling, not a judgment call.
- **Category, exactly one.** Priority order: **skill** when the diff's primary content sits under a skill directory; **documentation** when every changed file is a doc with no functional change riding along; otherwise map the commit type prefix (`fix` → **bug**, everything else → **enhancement**).

If the repo's label set has no match for a dimension, skip it — never force a name that isn't there.

## Run it

No open PR for this branch yet:
```
gh pr create --title "<title>" --body "<body>" [--label "<category>"] [--label "<authorship>"] [--draft]
```

PR already exists (description/label edit only):
```
gh pr edit <number> --body "<body>" [--add-label "<category>"] [--add-label "<authorship>"]
```

Never add a trailer to the body — no co-author line, no generated-by line, matching `generate-commit-message`'s own rule.

## Return

The `gh pr create`/`gh pr edit` output (it prints the URL) — nothing else.
