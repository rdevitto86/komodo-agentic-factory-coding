# 0003. Only `internal/mount/` names a host

**Status:** Accepted, 2026-09-21.

## Context

The line must survive a change of host with no change outside one directory. That is the exit test.

## Decision

Every vendor name, host tool name, host path, and host flag lives in `internal/mount/<host>/`. A mount registers itself: its config paths, its tool names for the guard, its install render, its tiers, and its usage reader. `komodo doctor` fails on any vendor name outside the mounts.

## Alternatives

- **A config file per host.** Data alone cannot carry a host's install render, usage parsing, or denial encoding.
- **A plugin binary per host.** It adds a second artifact to build and version for no gain at two or three hosts.

## Consequences

- **A new host is a new directory.** Nothing else changes.
- **The doctor is the enforcement.** A leak fails the gate before it lands.
- **Tests register a fake host.** The guard and profile tests never depend on a real one.
