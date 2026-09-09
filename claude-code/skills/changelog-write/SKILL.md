---
name: changelog-write
description: Append or amend a CHANGELOG.md entry in Keep a Changelog format with the right SemVer bump. Owns the CHANGELOG.md format.
argument-hint: <entry>
---

# Changelog write — CHANGELOG.md

Repo root, not `docs/` — it is a published artifact, and release tooling and readers both expect it there. `BACKLOG.md` is what is still open; this file is what shipped. `backlog-modify` owns the former.

Appends or amends `[Unreleased]`. Add a new entry in the right group, bump the version when the section is released. `changelog-audit` checks this file against git history and this skill's own rules, filing findings as `BACKLOG.md` stories — it never edits `CHANGELOG.md` itself, same division `backlog-audit` keeps with `backlog-modify`'s planning/normalize modes.

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

**Before appending a new version section**, invoke `git-commit-tag` by name — it checks whether the section directly below `[Unreleased]` (the most recently released one) already has a matching tag, and creates + pushes it if not.
