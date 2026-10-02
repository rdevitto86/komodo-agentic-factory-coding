# 0006. Every session is pinned, hermetic and scoped to its role

**Status:** Accepted, 2026-09-25. Amended by 0033.

**Context.** Sessions loaded personal configuration and floating model aliases, and the host auto-updated (evidence 10). Skills loaded everywhere, costing tokens in sessions that never used them.

**Decision.** Pin the host CLI, the full model IDs, the `komodo` release, the toolchains and LF line endings. Each line session loads a line-owned config directory and its role's own plugin, and nothing personal. The primary session loads only the orchestrator's skills. The meter reads the host's own totals.

**Alternatives.**

- **The host's bare mode.** It shuts out personal config, but also drops subscription login and hooks.
- **Light-tier builders for small work.** The 3 Haiku builds averaged 75 turns (evidence 7).

**Consequences.**

- **Every machine runs the same agents.** Doctor fails on any pin that differs.

**Spikes.** S3: does a line-owned config directory shut out personal instructions, plugins and MCP, and keep the host login? S4: does the final stream event carry turns, usage, cost and a session ID that resume accepts? S5: does schema output hold over a long builder session?
