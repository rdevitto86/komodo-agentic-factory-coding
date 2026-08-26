---
name: git-create-issue
description: File a repo's major findings or open questions as GitHub issues via `gh issue create`, after confirming the exact title/body with the user.
argument-hint: [finding or question to file, or "review" to scan recent findings]
---

# Git issue

Scoping: **$ARGUMENTS** (default: ask which finding/question to file)

Turns a finding, question, or `BACKLOG.md` line into a GitHub issue. Reachable by name only — never fires on a bare, undirected request, since filing an issue is a shared, outward-visible action nobody should trigger by accident.

**Requires a GitHub remote.** If `gh repo view` fails (no remote, not a GitHub repo, `gh` not authenticated), say so and stop.

## Process

1. Identify the finding or question to file — from `$ARGUMENTS`, from a recent audit's report, or from a `BACKLOG.md` line the user points at.
2. Draft the issue:
   - **Title** — one line, states the problem or question, no ticket-speak.
   - **Body** — what was found, where (`file:line` if code-derived), why it matters, and any evidence (command output, diff snippet). A question gets the question plus the context that makes it answerable.
   - **Labels** — only if the repo's existing issues show a convention worth matching; never invent a label taxonomy.
3. **Show the drafted title and body to the user and get explicit confirmation before running `gh issue create`.** This is the one non-negotiable step — `AGENTS.md`'s shared-action rule reserves confirmation for exactly this: an action that reaches outside the local repo. Never fire-and-forget.
4. On confirmation, write the title and body to temp files and run `gh issue create --title-file <title-path> --body-file <body-path>` (plus `--label` if step 2 set any) — never interpolate title/body into an inline `--title "<title>" --body "<body>"` string, since evidence pulled from command output or a diff snippet routinely contains backticks, `$()`, or quotes that break or hijack the constructed shell command. Report the returned issue URL.
5. On decline or edit request, revise and re-confirm — never file a version the user hasn't seen.

## Scope

One issue per finding or question, not one per file. A batch of related findings from the same audit can be filed as one issue with a checklist body if the user prefers — ask which shape they want when filing more than one.

## Output

The confirmed title/body before filing; the issue URL after. Never both silently — the confirmation step is not optional even when `$ARGUMENTS` already looks complete.
