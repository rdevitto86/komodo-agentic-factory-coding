# komodo-claude-core

Shared Claude Code configuration for all Komodo projects — agents, hooks, and settings symlinked into `~/.claude/` so every project inherits them.

The authoritative description of the system (agent roster, modes, MCP agents, hard rules) lives in [`CLAUDE.md`](CLAUDE.md). This README is just the quick start.

## Structure

```
claude/
├── agents/        # one self-contained folder per agent (agent.md + its standards/skills/modes/docs)
├── hooks/         # global shell hooks triggered by Claude Code events
└── settings.json  # global permissions, plugins, allowed commands, hook registration
```

Each agent owns its own standards, skills, and mode folders — there is no global `standards/` or `skills/` directory. Cross-cutting coding standards live under `claude/agents/swe/`. See `CLAUDE.md` for the encapsulation model and the per-agent **modes** that keep spawned-agent context lean.

## Setup

Run once after cloning:

```bash
bash setup.sh
```

Symlinks everything in `claude/` into `~/.claude/` and removes stale links from the previous layout. Restart Claude Code afterward.

## Agents

Nine Claude subagents (advisor, project-manager, swe, quality-assurance, devops, electrical-engineer, mechatronics, botanist, logistics), plus local MCP agents (`pm`, `qa`, and others) on the komodo bridge. The advisor is the default orchestrator and dispatches specialists with scoped modes. Full roster and triggers: [`CLAUDE.md`](CLAUDE.md).

## Project-level config

Projects keep only project-specific overrides in their own `.claude/settings.json`. Global permissions and agents live here and are not duplicated per-project.
