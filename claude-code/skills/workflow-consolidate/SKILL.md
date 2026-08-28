---
name: workflow-consolidate
description: Record finished work in a fork — changelog entry, version bump, doc refresh, backlog cleanup.
argument-hint: <the task summaries that just went green>
context: fork
agent: workflow-implementer
background: false
---

# Consolidate

Completed: **$ARGUMENTS**

**You cannot see the calling conversation.** The list above is what shipped. If it is empty, stop and say so.

**Load `changelog`** for the changelog format and the version rules. **Never read the SDD** — this phase does not touch the spec, and there is nothing here that needs it.

## Order

1. **Read `CHANGELOG.md`.** `[Unreleased]` already carries this band's entries — `workflow-loop`'s P2.4 closeout wrote them before this phase ran. Never author a new bullet here; a shipped story with no matching entry is a gap to report, not something to backfill silently.
2. **Release `[Unreleased]`** into a `## [X.Y.Z] — <date>` heading, at the bump its entries earn per `changelog`'s versioning table.
3. **Sync the language manifest** to the new heading, never the reverse. No manifest means no sync — the heading is the version and the user tags it.
4. **Delete the completed stories from `BACKLOG.md`.** Leave `[BLOCKED]` stories in place, and never delete a story you cannot confirm shipped.
5. **Refresh `README.md` only where this change invalidated it** — a new endpoint, a changed command, a new environment variable. Load `readme` for its shape. Never rewrite it wholesale.

**Never rewrite a released changelog section.** A correction is a new entry.

**This phase documents; it does not summarise the project.** If the entry is longer than the diff deserves, it is wrong.
