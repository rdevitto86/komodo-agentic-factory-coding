---
name: consolidate
description: Record finished work in a fork — changelog entry, version bump, doc refresh, backlog cleanup.
argument-hint: <the slice ids and task summaries that just went green>
context: fork
agent: implementer
background: false
disable-model-invocation: true
---

# Consolidate

Completed: **$ARGUMENTS**

**You cannot see the calling conversation.** The list above is what shipped. If it is empty, stop and say so.

**Load `worklog`** for the changelog format and the version rules. **Never load `docs`** — this phase does not touch the specs, and paying for their templates here is waste.

## Order

1. **Read `CHANGELOG.md`** for the current version and existing entries.
2. **Append a new version section** at the bump the shipped slices earn, citing slice IDs and any PRD requirement IDs.
3. **Sync the language manifest** to the new heading, never the reverse. No manifest means no sync — the heading is the version and the user tags it.
4. **Delete the completed stories from `BACKLOG.md`.** Leave `[BLOCKED]` stories in place, and never delete a story you cannot confirm shipped.
5. **Refresh `README.md` only where this change invalidated it** — a new endpoint, a changed command, a new environment variable. Load `readme` for its shape. Never rewrite it wholesale.

**Never rewrite a released changelog section.** A correction is a new entry.

**This phase documents; it does not summarise the project.** If the entry is longer than the diff deserves, it is wrong.
