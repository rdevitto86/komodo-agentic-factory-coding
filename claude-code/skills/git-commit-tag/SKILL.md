---
name: git-commit-tag
description: Check whether the most recently released CHANGELOG.md section has a matching git tag, and create + push it if not.
argument-hint: []
---

# Tag sync

**Inside a git repository, with `CHANGELOG.md` present** — if either isn't true, say so and stop.

A released section isn't real until it's both merged and tagged. **Tag creation is allowed to you** — `git_guard.py` permits `git tag` (lightweight or annotated); only `-d`/`-D`/`--delete`/`-f`/`--force` stay denied. Creating one is not committing to or pushing a protected branch, so it carries none of that restriction. Pushing the tag itself (`git push origin vX.Y.Z`) is likewise not a push of a protected branch.

## Check

Run this against the section directly below `[Unreleased]` (the most recently released one) — or the section the caller names, if it names one.

1. **List existing tags** with `git tag -l`, or by reading `.git/refs/tags/` and `.git/packed-refs` directly.
2. **If a matching `vX.Y.Z` tag already exists**, stop — nothing to do.
3. **If it's missing, find the commit that introduced that heading**: `git log -p --follow -- CHANGELOG.md`, the commit whose diff adds that exact `## [X.Y.Z]` line. If no commit has it yet, the section itself is still uncommitted — skip silently, this resolves itself next time this check runs.
4. **Otherwise, create and push it yourself**: `git tag -a vX.Y.Z <hash> -m "<one-line summary>"` then `git push origin vX.Y.Z` — report the tag you created alongside whatever else you're reporting, rather than handing the user a command you were able to run.

## Callers

- `changelog`'s Part 1 invokes this by name before appending a new version section, rather than restating the check.
- `workflow-loop`'s P4 (`workflow-complete`) invokes this by name once the band's push has landed, against the section that band just released.
