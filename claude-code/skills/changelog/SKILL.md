---
name: changelog
description: Create, edit, and audit CHANGELOG.md — append a new entry in Keep a Changelog format with the right SemVer bump, or audit it against git history for missing/backfillable entries, format drift, and version-bump/tag misalignment, filed as BACKLOG.md stories. Owns the CHANGELOG.md format.
argument-hint: [write <entry> | audit [scope]]
---

# Changelog — CHANGELOG.md

**Two modes, one file.** The first token in `$ARGUMENTS` picks the mode:

- **`write <entry>`** → Part 1, appending or amending `[Unreleased]`. Add a new entry in the right group, bump the version when the section is released.
- **`audit [scope]`** → Part 2, the audit. Check `CHANGELOG.md` against git history and Part 1's own rules, filing findings as `BACKLOG.md` stories. Default scope is the whole file.

If `$ARGUMENTS` is empty or names no mode token, treat it as `audit` with no scope (its own default already covers that case).

Part 1 is the only mode that writes `CHANGELOG.md`. Part 2 never edits it — findings only, same division `backlog`'s audit mode keeps with its own planning/normalize modes.

---

# Part 1 — Write (`write <entry>`)

Repo root, not `docs/` — it is a published artifact, and release tooling and readers both expect it there. `BACKLOG.md` is what is still open; this file is what shipped. `backlog` owns the former.

```markdown
# Changelog

Notable changes to this project. Format follows Keep a Changelog; versions follow SemVer.

## [Unreleased]

## [0.2.0] — 2026-08-21

### Added
- Token refresh endpoint

### Fixed
- Rate limiter counted preflight requests against the caller's quota
```

- **Append-only.** Never rewrite a released section; a correction is a new entry.
- **Group under `Added` / `Changed` / `Fixed` / `Removed` / `Security`.** Omit any group with no entries.
- **One line per entry**, written for someone who did not do the work.

## Versioning

**The `CHANGELOG.md` heading is the version source of truth.** The language manifest (`package.json`, and so on) is synced to match it, never the reverse. Go repos have no manifest, so the heading *is* the version, tagged per "Tag sync" below.

| The change | Bump |
|---|---|
| Adds a new backward-compatible capability | Minor |
| Fixes behavior, no new capability | Patch |
| Breaks a published contract | Major |

## Tag sync

A released section isn't real until it's both merged and tagged. **Tag creation is allowed to you** — `git_guard.py` permits `git tag` (lightweight or annotated); only `-d`/`-D`/`--delete`/`-f`/`--force` stay denied. Creating one is not committing to or pushing a protected branch, so it carries none of that restriction.

**Before appending a new version section**, check whether the section directly below `[Unreleased]` (the most recently released one) already has a matching tag:

- List existing tags with `git tag -l`, or by reading `.git/refs/tags/` and `.git/packed-refs` directly.
- If it's missing, find the commit that introduced that heading: `git log -p --follow -- CHANGELOG.md`, the commit whose diff adds that exact `## [X.Y.Z]` line. If no commit has it yet, the section itself is still uncommitted — skip silently, this resolves itself next time this check runs.
- Otherwise, create and push it yourself: `git tag -a vX.Y.Z <hash> -m "<one-line summary>"` then `git push origin vX.Y.Z` — report the tag you created alongside whatever else you're reporting, rather than handing the user a command you were able to run.

---

# Part 2 — Audit (`audit [scope]`)

Scoping: **$ARGUMENTS**, minus the `audit` token (default: the whole file)

Findings only, never edits `CHANGELOG.md` — Part 1 owns the format and is the only mode that writes it. This locates backfill candidates, format inconsistencies, and version/tag misalignment, and hands the fix to the user or a follow-up `/changelog write` pass, same division of labor `backlog`'s audit mode keeps with its own planning/normalize modes.

Every check below tests against Part 1's own rules (grouping, versioning, tag sync), not rules restated here.

## Process

1. **Read `CHANGELOG.md` in full**, every released section plus `[Unreleased]`.
2. **List existing tags** with `git tag -l`, or by reading `.git/refs/tags/` and `.git/packed-refs` directly. Flag any released section with no matching tag, and any tag with no matching section.
3. **Check version ordering** — strictly descending below `[Unreleased]`, dates non-increasing top to bottom.
4. **Check each release's bump against the rule table** (Minor adds a new backward-compatible capability, Patch fixes behavior with no new capability, Major implies a broken contract) — flag a section whose bump size the entries don't support.
5. **Check manifest sync** — if the repo has a language manifest (`package.json` and so on), its version field must match the most recent released heading. Go repos have none; skip.
6. **Find backfill candidates** — for the gap below each released section (down to the tag before it, or to repo start for the oldest), walk `git log --oneline <range>` for commits representing user-visible behavior with no matching bullet in that section's entries. Cite the commit hash.
7. **Find append-only violations** — `git log -p --follow -- CHANGELOG.md`; a hunk that edits or removes a line under an already-released heading (not the most-recently-added section) rewrote shipped history. Cite the commit hash.
8. **Find format drift** — an entry not grouped under `Added`/`Changed`/`Fixed`/`Removed`/`Security`.

## Report

| Kind | Where | Finding | Evidence |
|---|---|---|---|
| Missing entry | `## [0.4.0]` | commit ships a new endpoint, no matching bullet | `git log` hash |
| Version misalignment | `## [0.5.0]` | Minor bump, no new capability in the entries | section text |
| Format inconsistency | `## [0.3.1]` | entry outside the five groups | line text |

**Sev**: Critical (a released heading no longer matches the manifest, or an already-released section was rewritten — the record no longer matches what shipped) · High (non-monotonic version order, a bump size the entries don't support, a released section with no tag) · Medium (a tagged release or merged change with no matching entry — backfill candidate) · Low (grouping drift, non-blocking).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <finding> · S → \`/changelog audit\` reports it clear`. Append under the current target state's `Cross-Cutting` domain — full story-line rules live in `backlog`. `--report` prints the table only; nothing is written.
</content>
