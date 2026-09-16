# Security

## Reporting a vulnerability

Use GitHub's private vulnerability reporting on this repository's Security tab. Do not open a public issue for a security problem.

## What installing this toolkit grants

`python3 -m komodo install` copies `claude-code/` into `~/.claude`. Two files under `~/.claude/hooks/` then run as commands in every Claude Code session on that machine: `guard.py` on every Bash call, and `context_injector.py` at session start. Both are short, stdlib-only, and fail open on an internal error. Because the install is a copy, an edit in the clone changes nothing until the installer is re-run.

`python3 -m komodo hooks install <repo>` sets that repo's `core.hooksPath` to this clone's `komodo/hooks/`. From then on, `pre-commit.py` and `pre-push.py` run on every commit and push in that repo, from whatever the clone currently holds.

`python3 -m komodo run` spawns worker processes with a stripped environment (`gitops.worker_env`): no GitHub token, no git credential helper, no SSH identity, an unauthenticated `gh`. The orchestrator process itself holds the user's credentials and is the only pusher. It refuses protected refs, force, amend, and trailers in code, but it runs as the user and can push any unprotected branch the user can.

Workers run `claude -p --dangerously-skip-permissions` inside a worktree. A worker can run any command the user can. The worktree and the credential stripping bound what it can publish, not what it can execute.

## Supported versions

This repo ships from `main` and supports the latest release only.
