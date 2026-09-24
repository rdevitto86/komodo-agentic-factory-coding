# 0004. One guard hook, which is not a sandbox

**Status:** Accepted, 2026-09-21.

## Context

An agent with a shell can do anything the user can. The prototype stacked several hooks. Each added latency to every tool call, and none of them could stop a determined bypass.

## Decision

One hook runs before each shell, edit, and write call on every host. It refuses four things and nothing else: work on a critical ref, a path outside the worktree, a write to host or toolkit config, and a commit trailer. It also refuses any rewrite of pushed history. It fails open on an internal error. The gate runs its table on every commit, so a broken guard fails the build and never a run.

## Alternatives

- **A container or sandbox.** It is the real boundary, but it breaks the host's own tools, credentials, and caches.
- **More hooks.** Each is another process per call and another place for policy to drift.

## Consequences

- **The hard boundaries sit elsewhere.** The forge's ruleset on each critical ref, audited by `komodo doctor --remote`, and the headless credential scrub stop what the guard cannot.
- **Inside the worktree an agent is free.** Reset, rebase, delete: none of it is refused.
- **A bypass is a table row.** Each one found becomes a denied row before its fix lands.
