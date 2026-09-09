---
name: git-pr-create
description: Open the PR for the current branch — title, body filled from the repo's own PR template, and a label — as one action, not a text dump. Also the source for git branch/push/merge/protected-ref conventions and what git_guard.py enforces for each.
argument-hint: [--draft] [--labels "<category>[,<authorship>]"]
---

# Open PR

**Inside a git repository, on a non-default branch with at least one commit ahead of the base** — if any of those isn't true, say so and stop. `git_guard.py`'s `gh` allowlist scopes `pr edit`/`pr comment` to the current branch's own PR number — it has no way to touch anyone else's.

**It is an action, not a text dump.** `git_guard.py` already permits `gh pr create` and `gh pr edit` — this skill runs them directly. Never print the filled body to the terminal for the user to paste; the terminal is not where a PR description belongs.

## Fill the template, not the commit log

Read every commit ahead of the base (`git log <base>..HEAD`) and `git diff <base>..HEAD --stat` — the body describes the *change*, not the commit history. Open a single file's diff only when its stat line and the commit messages leave the change genuinely ambiguous.

- **Repo has `.github/PULL_REQUEST_TEMPLATE.md`:** fill it section-for-section as written on disk — do not paraphrase the template's own headings or drop a section. Populate:
  - `Summary`: one or two sentences on what the PR is for. Never a file list.
  - `Changes`: one bullet per area (not per file), `**<area>** — <what changed>`.
  - `Validation Evidence`: only what a green CI run can't show — a live-dependency happy path, a cURL against STG, a behavior with no automated coverage yet. `Covered by CI` is a complete answer; never paste unit/component/contract results here.
  - `Dependencies`: numbered, in landing order — another PR in this repo or a sibling repo (internal), or a package/service/API version this PR requires (external). Omit the whole section when there are none; never write `none` as a list item.
  - Leave the template's HTML comments in place — they're instructions to a human filling the form by hand, and this is filling the same form, just automated. Strip a comment only if the repo's own template already omits it.
- **Repo has no PR template:** fall back to a `## Summary` + `## Changes` body in the same shape (a sentence, then bulleted areas) — never dump raw commit subjects as the body.

Title: `<type>: <summary>` — the same convention `git-commit-message` uses, max 72 chars, imperative, no trailing period. Use the most recent commit's subject if it already fits; otherwise write one that names the overall change.

## Label it

**If `$ARGUMENTS` already names labels (`--labels`), use them as-is** — `workflow-loop`'s P3 decides them right before its own commit, off the full band diff, so this phase never re-derives from a diff that also includes that commit. Skip straight to applying them below.

Otherwise, pick exactly one category label from what the repo's own `gh label list` returns — never invent a label name or create one. Map from the diff, in this priority order:

1. **Skill** — the diff's primary content is under a skill directory (new skill, rewrite, split, or rename).
2. **Documentation** — every changed file is a doc (`.md`, `docs/`, comments) with no functional or config change riding along.
3. Otherwise, by the commit type prefix (`git-commit-message`'s taxonomy): `fix` → **Bug**, `feat`/`chore`/`refactor`/`perf`/`build`/`ci`/`test` → **Enhancement**.

**Also add the authorship label, if the repo has one.** This skill only ever runs agent-side, so if the label set includes `@agent` (or an equivalently-named "opened by an agent" label), always add it alongside the category label — it's a fact about who's calling, not a judgment call.

**Duplicate** and **Do not merge** are never inferred — apply one only when the user says so or you find a genuinely open PR this duplicates (`gh pr list`), and name which PR in your report either way.

## Size it

Rule of thumb, not a hard gate — nothing enforces it, and applying it is a judgement call at PR-creation time.

| Metric | Ideal | Max |
|---|---|---|
| Lines changed | 100–300 | 500 |
| Files changed | 1–5 | 12 |
| Commits | 1–5 | 10 |
| Review time | 10–30 min | 45 min |

Check before creating: `git diff <base>...HEAD --shortstat` for lines/files, `git log <base>..HEAD --oneline | wc -l` for commits. Review time is your own estimate from the diff's shape (touched-file count, and how mechanical vs. dense the change reads) — no command produces it.

**Under the max:** create as planned. **Over the max:** decide, don't auto-split — a split only makes sense when the band's own tasks partition cleanly along file/commit boundaries with no shared edits. If the diff is one entangled change (shared files, a refactor that touches every caller), ship it as one oversized PR and say so in the body rather than forcing an artificial cut. A genuine split needs a second branch cut from the base for the remaining commits (`git switch -c <type>/<desc> <base>`) — never a rebase or history rewrite of the branch already in flight; `git_guard.py` denies both outright, so a split decided after commits already exist on one branch is not available here — size the band before implementing (`workflow-loop`'s P1) rather than trying to divide it after the fact.

Note the measured metrics against the thresholds in your report either way, so the user sees the call being made, not just its result.

## Run it

New PR:
```
gh pr create --title "<title>" --body "<filled body>" --label "<category label>" --label "<authorship label>"
```

Existing PR (description/label edit only, no new commits): `gh pr edit <number> --body "<filled body>" --add-label "<category label>" --add-label "<authorship label>"`.

Add `--draft` to `gh pr create` when the caller passed it. Never add a trailer — no co-author line, no generated-by line.

## Return

The `gh pr create`/`gh pr edit` output (it prints the PR URL) — nothing else.

---

# Git lifecycle: conventions and enforcement

This section is the merged source for every git branch/push/merge/protected-ref/stash/worktree fact in the toolkit. `git_guard.py` enforces every "Enforcement" line below directly, in code — this is what to do with the capability, not a second copy of the block. `git-pr-review`, `git-pr-comment`, `git-issue-create`, `git-issue-review`, `git-commit-message`, `git-repo-init`, `workflow-loop`, and `git-merge-conflict` all point here rather than restating any of it.

## Never route around the block

No `sh -c` / `bash -c` wrapper, no shell alias, no editing files under `.git/` with Edit or Write, no `GIT_EDITOR` trick, no piping a denied command through `tee` or a heredoc to dodge the scan. A protected branch, a rebase, a force-push — if it's denied, it's denied; hand the user the exact command instead of retrying a disguised version of the same call.

Read-only inspection is unrestricted: `status`, `diff`, `log`, `show`, `blame`, `rev-parse`, `ls-files`, `fetch`, plain `git branch`/`git stash` listing. Use it freely.

`PUBLISH_ENABLED=0` in the environment reverts everything below to a blanket deny with no source edit.

## Protected refs

`main`, `master`, `trunk`, `prod`, `production`, `release/*`, and `hotfix/*` are protected. Nothing commits or pushes to one directly, and nothing lands a branch into one except through the PR's merge button — that step belongs to the user, never to the agent, regardless of what capability is otherwise available. Enforcement: `git_guard.py` denies a commit or a push to any of these outright, regardless of `PUBLISH_ENABLED` — there is no "temporarily on main" case that makes either safe.

## Branches

Naming: `<type>/<short-kebab-description>` — the same `type` taxonomy `git-commit-message` uses for commit prefixes: `feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`. Create the branch before the first commit, not after — work should never land on a protected ref and then need to be moved.

**Resuming a `[WIP]` band reuses its existing branch**, never a second branch cut for work already in flight. Match the branch name to the story already underway, not to whatever happens to be checked out.

Enforcement: `git_guard.py` checks the shape and rejects a protected or malformed name outright, so a badly-named branch never reaches disk and there's nothing to catch after the fact.

## Commits

Message format — subject line, body, trailers — is owned entirely by `git-commit-message`. The one convention that lives here: a commit never carries `--amend`, never skips hooks with `--no-verify`, and never adds a co-author or generated-by trailer, on any branch, under any circumstance.

Enforcement: `git_guard.py` rejects `--amend`, `--no-verify`, `--no-gpg-sign`, and any co-author/generated-by trailer outright — don't rely on that as a safety net, write it right the first time. Committing on a protected branch is always denied, regardless of `PUBLISH_ENABLED`.

## Push

Push names its remote and branch explicitly — `origin <branch>` — never a bare push, since the destination has to be readable from the command itself. A force-push (in any of its flag spellings) is never part of the normal flow; if one is truly needed, that decision and that command belong to the user, not to an agent acting on their behalf.

Enforcement: `git push -u origin <branch>` on your own non-protected branch is allowed; a bare `git push` is denied because the destination isn't readable from the command. Force-push (`-f`/`--force`/`--force-with-lease`/`--force-if-includes`) is always denied outright by the guard. Pushing directly to a protected branch is denied the same as committing to one.

## Merging

Only one direction is ever a convention here: the protected base merges into your branch, to pull in its latest changes and surface conflicts early — never your branch into the base. A strategy flag that auto-resolves a conflict without ever showing it defeats the point of merging early, so the convention is always to resolve conflicts by hand, one file at a time, and commit the result with the merge's own default message. Landing a branch into its protected base is a human action end to end, taken through the PR's merge button (or the user's own CLI) — never a step an agent takes.

Enforcement: `git merge <main-or-equivalent>` (bare name or `origin/`-prefixed) is allowed from a non-protected branch; `--abort`/`--continue` are open as escape hatches; a strategy flag that auto-resolves without a visible conflict (`-X`, `--strategy`, `-s`, `--squash`) is denied by the guard — the point is to see the conflict, not paper over it.

Resolve conflicts by editing the marked files directly, `git add` each one, then `git commit --no-edit` (accepts git's own merge message, non-interactive — a bare `git commit` here opens an editor and hangs). Push normally afterward; a merge commit is a fast-forward from the remote's point of view, so it never needs force.

`git rebase` stays denied regardless of `PUBLISH_ENABLED`: it rewrites history and would need a force-push to publish. `git pull` is allowed only with `--ff-only`; use `git fetch` (already unrestricted) plus this merge instead for anything that would actually need to resolve something.

## Stashing

`git stash push -u`/`pop`/`apply` are allowed; `drop`/`clear` are denied (discarding saved state outright is not part of the publish capability). Use `push -u` (the `-u` catches untracked files) when uncommitted work is in the way of something else you need to do, never to dodge writing a real commit.

## Worktrees

Raw `git worktree` is blocked outright. For session-level isolation, use the `EnterWorktree`/`ExitWorktree` tools — but only when the user or project instructions explicitly say "worktree"; never reach for it on your own initiative. For parallel subagents writing to the same checkout, use `Agent`'s or `Workflow`'s `isolation: "worktree"` option instead of asking the user to hand-create one.
