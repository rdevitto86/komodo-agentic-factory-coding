---
name: git-create-pr
description: Open the PR for the current branch — title, body filled from the repo's own PR template, and a label — as one action, not a text dump.
argument-hint: [--draft]
---

# Open PR

**Inside a git repository, on a non-default branch with at least one commit ahead of the base** — if any of those isn't true, say so and stop. `git_guard.py`'s `gh` allowlist scopes `pr edit`/`pr comment` to the current branch's own PR number — it has no way to touch anyone else's.

**It is an action, not a text dump.** `rules-source-control` already permits `gh pr create` and `gh pr edit` — this skill runs them directly. Never print the filled body to the terminal for the user to paste; the terminal is not where a PR description belongs.

## Fill the template, not the commit log

Read every commit ahead of the base (`git log <base>..HEAD`) and the full diff (`git diff <base>..HEAD`) — the body describes the *change*, not the commit history.

- **Repo has `.github/PULL_REQUEST_TEMPLATE.md`:** fill it section-for-section as written on disk — do not paraphrase the template's own headings or drop a section. Populate:
  - `Summary`: one or two sentences on what the PR is for. Never a file list.
  - `Changes`: one bullet per area (not per file), `**<area>** — <what changed>`.
  - `Validation Evidence`: only what a green CI run can't show — a live-dependency happy path, a cURL against STG, a behavior with no automated coverage yet. `Covered by CI` is a complete answer; never paste unit/component/contract results here.
  - `Dependencies`: numbered, in landing order — another PR in this repo or a sibling repo (internal), or a package/service/API version this PR requires (external). Omit the whole section when there are none; never write `none` as a list item.
  - Leave the template's HTML comments in place — they're instructions to a human filling the form by hand, and this is filling the same form, just automated. Strip a comment only if the repo's own template already omits it.
- **Repo has no PR template:** fall back to a `## Summary` + `## Changes` body in the same shape (a sentence, then bulleted areas) — never dump raw commit subjects as the body.

Title: `<type>: <summary>` — the same convention `write-commit-message` uses, max 72 chars, imperative, no trailing period. Use the most recent commit's subject if it already fits; otherwise write one that names the overall change.

## Label it

Pick exactly one category label from what the repo's own `gh label list` returns — never invent a label name or create one. Map from the diff, in this priority order:

1. **Skill** — the diff's primary content is under a skill directory (new skill, rewrite, split, or rename).
2. **Documentation** — every changed file is a doc (`.md`, `docs/`, comments) with no functional or config change riding along.
3. Otherwise, by the commit type prefix (`write-commit-message`'s taxonomy): `fix` → **Bug**, `feat`/`chore`/`refactor`/`perf`/`build`/`ci`/`test` → **Enhancement**.

**Also add the authorship label, if the repo has one.** This skill only ever runs agent-side, so if the label set includes `@agent` (or an equivalently-named "opened by an agent" label), always add it alongside the category label — it's a fact about who's calling, not a judgment call.

**Duplicate** and **Do not merge** are never inferred — apply one only when the user says so or you find a genuinely open PR this duplicates (`gh pr list`), and name which PR in your report either way.

## Run it

New PR:
```
gh pr create --title "<title>" --body "<filled body>" --label "<category label>" --label "<authorship label>"
```

Existing PR (description/label edit only, no new commits): `gh pr edit <number> --body "<filled body>" --add-label "<category label>" --add-label "<authorship label>"`.

Add `--draft` to `gh pr create` when the caller passed it. Never add a trailer — no co-author line, no generated-by line.

## Return

The `gh pr create`/`gh pr edit` output (it prints the PR URL) — nothing else.
