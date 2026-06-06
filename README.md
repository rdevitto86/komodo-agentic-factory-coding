# komodo-claude-core

Shared Claude Code configuration for all Komodo projects — agents, hooks, and settings symlinked into `~/.claude/` so every project inherits them.

The authoritative description of the system (agent roster, modes, MCP agents, hard rules) lives in [`CLAUDE.md`](CLAUDE.md). This README is just the quick start.

## Structure

```
claude/
├── agents/        # one self-contained folder per agent (agent.md + its standards/skills/modes/docs)
├── hooks/         # global shell hooks triggered by Claude Code events
└── settings.json  # global permissions, plugins, allowed commands, hook registration
scripts/
└── validate-refs.sh  # checks that all ~/.claude/agents/... references resolve in the repo
.gitignore         # excludes the local working files below from git
TODO.md            # local task tracker (git-ignored) — outstanding work across sessions
MEMORY.md          # local session cache (git-ignored) — resume state across sessions
```

Each agent owns its own standards, skills, and mode folders — there is no global `standards/` or `skills/` directory. Cross-cutting coding standards (principles, comments, security, PRs, stack, git-flow, logging, readme-maintenance) live under `claude/agents/swe/`. See `CLAUDE.md` for the encapsulation model and the per-agent **modes** that keep spawned-agent context lean.

## Setup

Run once after cloning:

```bash
bash setup.sh
```

Symlinks everything in `claude/` into `~/.claude/` and removes stale links from the previous layout. Also validates cross-agent references (`scripts/validate-refs.sh`); pass `--dry-run` to preview what would be linked without making changes. Restart Claude Code afterward.

## Agents

Claude subagents (advisor, project-manager, swe, quality-assurance, devops, electrical-engineer, mechatronics, data-analyst, cyber-security, machinist, botanist, logistics), plus MCP-primary agents (`pm`, `qa`, lawyer, marketing, customer-servicing, tax) on the komodo bridge. The advisor is the default orchestrator — it is the entry point for everything and dispatches specialists with scoped modes. Full roster, triggers, and routing notes: [`CLAUDE.md`](CLAUDE.md).

## Project-level config

Projects keep only project-specific overrides in their own `.claude/settings.json`. Global permissions and agents live here and are not duplicated per-project.
