---
name: generate-changelog
description: Fixed format for CHANGELOG.md — Keep a Changelog groups, PRD requirement citations, SemVer version bumps. Load before creating or editing it.
paths: "**/CHANGELOG.md"
---

# CHANGELOG.md — the shipped record

Repo root, not `docs/` — it is a published artifact, and release tooling and readers both expect it there. `BACKLOG.md` is what is still open; this file is what shipped. `generate-backlog` owns the former.

```markdown
# Changelog

Notable changes to this project. Format follows Keep a Changelog; versions follow SemVer.

## [Unreleased]

## [0.2.0] — 2026-08-21

### Added
- Token refresh endpoint (`AU2`, `AU3`)

### Fixed
- Rate limiter counted preflight requests against the caller's quota (`S3`)
```

- **Append-only.** Never rewrite a released section; a correction is a new entry.
- **Group under `Added` / `Changed` / `Fixed` / `Removed` / `Security`.** Omit any group with no entries.
- **Cite the PRD requirement ID where the shipped story carried one.** That keeps the trace from shipped code back to the reason it exists — load `generate-prd` if `docs/prd.md` doesn't exist yet.
- **One line per entry**, written for someone who did not do the work.

## Versioning

**The `CHANGELOG.md` heading is the version source of truth.** The language manifest (`package.json`, and so on) is synced to match it, never the reverse. Go repos have no manifest, so the heading *is* the version and the user tags it.

| The change | Bump |
|---|---|
| Satisfies a PRD requirement ID | Minor |
| Fixes behavior, no new requirement ID | Patch |
| Breaks a published contract | Major |
