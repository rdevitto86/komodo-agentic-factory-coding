# Security

## Reporting a vulnerability

Use GitHub's private vulnerability reporting on this repository's Security tab. Do not open a public issue for a security problem.

## What installing this toolkit grants

`scripts/install.py` symlinks `claude-code/` into `~/.claude`. Because these are symlinks, not copies, every file under `claude-code/hooks/` runs as a `PreToolUse`, `PostToolUse`, or `SessionStart` command on every tool call in every Claude Code session on that machine — and editing a file under `claude-code/` in the clone changes every session's behavior on its next start, with no reinstall step. This also means an upstream sync (`git pull` or equivalent) moves every session on that machine, not just this repo.

`scripts/install.py --ref <tag>` detaches the clone at a release tag before linking, which pins the install: it only moves when you re-run the installer against a different ref. Installing without `--ref` tracks whatever the clone's working tree currently holds.

`CODEOWNERS` requires a named reviewer on `claude-code/AGENTS.md`, `claude-code/settings.json`, and everything under `claude-code/hooks/` — the paths with the widest blast radius, since they are exactly what the symlink puts on every tool call.

## Supported versions

This repo ships from `main` and supports the latest release only. There is no maintained backport branch.
