---
name: write-changelog
description: Fixed format for CHANGELOG.md — Keep a Changelog groups, SemVer version bumps. Load before creating or editing it.
paths: "**/CHANGELOG.md"
---

# CHANGELOG.md — the shipped record

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

**The `CHANGELOG.md` heading is the version source of truth.** The language manifest (`package.json`, and so on) is synced to match it, never the reverse. Go repos have no manifest, so the heading *is* the version and the user tags it.

| The change | Bump |
|---|---|
| Adds a new backward-compatible capability | Minor |
| Fixes behavior, no new capability | Patch |
| Breaks a published contract | Major |

## Tag sync

A released section isn't real until it's both merged and tagged. **Tag writes are denied to you** — a tag names a commit on the default branch, and you never work there. Every tag is the user's to create.

**Before appending a new version section**, check whether the section directly below `[Unreleased]` (the most recently released one) already has a matching tag:

- List existing tags with `git tag -l`, or by reading `.git/refs/tags/` and `.git/packed-refs` directly.
- If it's missing, find the commit that introduced that heading: `git log -p --follow -- CHANGELOG.md`, the commit whose diff adds that exact `## [X.Y.Z]` line. If no commit has it yet, the section itself is still uncommitted — skip silently, this resolves itself next time this check runs.
- Otherwise hand the user the exact line — `git tag -a vX.Y.Z <hash> -m "<one-line summary>"` — alongside whatever else you're reporting.
