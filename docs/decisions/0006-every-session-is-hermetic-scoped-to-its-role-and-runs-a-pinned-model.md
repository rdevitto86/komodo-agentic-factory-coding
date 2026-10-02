# 0006. Every session is hermetic, scoped to its role, and runs a pinned model

**Status:** Accepted, 2026-09-25. Consolidated 2026-10-02 from 0006, 0025, 0027 and 0033.

**Context.** Sessions loaded personal configuration and floating model aliases, and the host auto-updated (evidence 10). Skills loaded everywhere, costing tokens in sessions that never used them. Spikes S2 to S5, S7 and S8 ran on 2026-09-26 on macOS with CLI 2.1.283 on a Pro subscription:

- **S2:** a builder under `dontAsk`, an allow list and the sandbox built and tested a Go module in 5 turns with 0 denials, once `GOCACHE` and `GOTMPDIR` sat inside the worktree. The sandbox is the wall, not the Bash allow list.
- **S3:** a fresh `CLAUDE_CONFIG_DIR` logs the host out. The default directory with `--strict-mcp-config` and a narrow `--setting-sources` kept the login and shut out personal instructions, plugins, agents and MCP servers.
- **S4 and S5:** the result event carries turns, usage, cost and a session ID that resume accepts, and schema output held over a 38-turn builder session.
- **S7 and S8:** `komodo run` inside an interactive session launches its own sessions. `CLAUDE_CODE_MAX_TURNS` caps tool calls; `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB` left forge tokens in place and reset `dontAsk`.

A pinned host CLI never chose the CLI that ran; it only stopped the gate every time the host shipped a release.

**Decision.**

- **Pinned:** full model IDs, the `komodo` release, the toolchains and LF line endings. Doctor fails on any that differ.
- **Not pinned: the host CLI.** Preflight checks only that it runs and holds a login, and each line session sets `DISABLE_AUTOUPDATER=1` so it never changes mid-run.
- **A line session keeps the host's default config directory and isolates by flags:** `--setting-sources local`, `--strict-mcp-config`, `--plugin-dir` for its role and `--settings` for its permissions. A role sees only its own plugin's skills; its repo rules come from its brief.
- **The conductor sets Go's caches inside the worktree, and caps each role's turns** with `CLAUDE_CODE_MAX_TURNS`. It removes forge credentials itself and never sets `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB`.
- **The primary session loads only the orchestrator's skills.** The meter reads the host's own totals.

**Alternatives.**

- **A line-owned config directory with a long-lived token.** The line would store a login secret, and the built-ins would still load.
- **The host's bare mode.** It drops subscription login and hooks.
- **Light-tier builders for small work.** The 3 Haiku builds averaged 75 turns (evidence 7).

**Consequences.**

- **Every machine runs the same agents,** and a host update no longer stops the gate or the line.
- **The host's built-in skills stay in every session's context,** and the worktree's `CLAUDE.md` doesn't load in a line session.
- **A host change that breaks the line shows up as a failed session,** not a doctor finding.
