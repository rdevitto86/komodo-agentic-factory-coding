---
name: workflow-consolidate
description: Record finished work in a fork — changelog entry, version bump, doc refresh, backlog cleanup.
argument-hint: a builder brief — Task | Files | Context | Done when | Out of scope (all five required)
context: fork
agent: builder
background: false
---

# Consolidate

Brief: **$ARGUMENTS** — a `builder` brief whose `Task` slot carries the band's `TSK-` IDs plus their summaries, alongside the other four required slots.

**You cannot see the calling conversation.** That brief is what shipped. An absent or empty required slot is your standing stop, not something to infer from the repo.

**Load `changelog-write`** for the changelog format and the version rules, and **`backlog-modify`** for `BACKLOG.md`'s status tags, `Blocked By:`/`Recheck:` shape, and `SUB-`/`Done when:` shape — the two steps below edit against those rules, not ones restated here. **Never read the SDD** — this phase does not touch the spec, and there is nothing here that needs it.

## Order

1. **Read `CHANGELOG.md`.** `[Unreleased]` already carries this band's entries — `workflow-loop`'s P2.4 closeout wrote them before this phase ran. Never author a new bullet here; a shipped story with no matching entry is a gap to report, not something to backfill silently.
2. **Release `[Unreleased]`** into a `## [X.Y.Z] — <date>` heading, at the bump its entries earn per `changelog-write`'s versioning table.
3. **Sync the language manifest** to the new heading, never the reverse. No manifest means no sync — the heading is the version, and `workflow-complete`'s P4 tags it from there once the branch is pushed.
4. **Clear stale blocks.** For every `[BLOCKED]` task in `BACKLOG.md`, run its `Blocked By:` bullet's `Recheck:` condition. A pass means the block is gone — set the task's status back to `[TODO]` and remove the `Blocked By:` bullet. A fail leaves the task exactly as it was.
5. **Confirm the band before deleting it.** For each `TSK-` this band shipped, run every one of its `SUB-` lines' `Done when:` commands and confirm each exits zero, and confirm every `Acceptance Criteria` box the task carries is ticked (`- [x]`). A red command or an unticked box means that task is not deleted — leave it `[TODO]` and add a one-line `Notes:` entry to this phase's report naming the command that failed, or the AC that's still unticked.
6. **Delete the completed stories from `BACKLOG.md`.** Leave `[BLOCKED]` stories in place, and never delete a story you cannot confirm shipped — that includes any task step 5 just kept for a failing command.
7. **Refresh `README.md` only where this change invalidated it** — a new endpoint, a changed command, a new environment variable. Load `readme-modify` for its shape. Never rewrite it wholesale.

**Never rewrite a released changelog section.** A correction is a new entry.

**This phase documents; it does not summarise the project.** If the entry is longer than the diff deserves, it is wrong.
