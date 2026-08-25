---
name: audit-changelog
description: Audit CHANGELOG.md against git history and generate-changelog's rules — missing/backfillable entries, format inconsistencies, version-bump and tag misalignment — filed as BACKLOG.md stories. Pass --report to skip the write.
argument-hint: [--report]
disable-model-invocation: true
---

# Changelog audit

Findings only, never edits `CHANGELOG.md` — `generate-changelog` owns the format and is the only skill that writes it. This locates backfill candidates, format inconsistencies, and version/tag misalignment, and hands the fix to the user or a follow-up `/generate-changelog` pass, same division of labor `audit-backlog` keeps with `generate-backlog`.

Load `generate-changelog` first — every check below tests against rules it owns (grouping, versioning, tag sync), not rules restated here.

## Process

1. **Read `CHANGELOG.md` in full**, every released section plus `[Unreleased]`.
2. **List existing tags** by reading `.git/refs/tags/` and `.git/packed-refs` directly — never `git tag -l`, it's blocked. Flag any released section with no matching tag, and any tag with no matching section.
3. **Check version ordering** — strictly descending below `[Unreleased]`, dates non-increasing top to bottom.
4. **Check each release's bump against the rule table** (Minor needs a cited PRD requirement ID, Patch needs none, Major implies a broken contract) — flag a section whose bump size the cited entries don't support.
5. **Check manifest sync** — if the repo has a language manifest (`package.json` and so on), its version field must match the most recent released heading. Go repos have none; skip.
6. **Find backfill candidates** — for the gap below each released section (down to the tag before it, or to repo start for the oldest), walk `git log --oneline <range>` for commits representing user-visible behavior with no matching bullet in that section's entries. Cite the commit hash.
7. **Find append-only violations** — `git log -p --follow -- CHANGELOG.md`; a hunk that edits or removes a line under an already-released heading (not the most-recently-added section) rewrote shipped history. Cite the commit hash.
8. **Find format drift** — an entry not grouped under `Added`/`Changed`/`Fixed`/`Removed`/`Security`, or a Minor-bump section missing its PRD requirement ID citation.

## Report

| Kind | Where | Finding | Evidence |
|---|---|---|---|
| Missing entry | `## [0.4.0]` | commit ships a new endpoint, no matching bullet | `git log` hash |
| Version misalignment | `## [0.5.0]` | Minor bump, no PRD requirement ID cited | section text |
| Format inconsistency | `## [0.3.1]` | entry outside the five groups | line text |

**Sev**: Critical (a released heading no longer matches the manifest, or an already-released section was rewritten — the record no longer matches what shipped) · High (non-monotonic version order, a bump size the entries don't support, a released section with no tag) · Medium (a tagged release or merged change with no matching entry — backfill candidate) · Low (grouping or citation drift, non-blocking).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <finding> · S → \`/audit-changelog\` reports it clear`. Append under the current target state's `Cross-Cutting` domain — full story-line rules live in `generate-backlog`. `--report` prints the table only; nothing is written.
