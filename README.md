# komodo-ai-agents

Shared agent configuration for all Komodo projects — agents, standards, modes, templates, and tool-platform adapters symlinked into `~/.claude/` so every project inherits them.

The authoritative description of the system (agent roster, modes, MCP agents, hard rules) lives in [`CLAUDE.md`](CLAUDE.md). This README is just the quick start.

## Structure

```
agents/            # one self-contained folder per agent (agent.md + its role-mode skills/docs)
standards/         # cross-cutting coding standards (principles, comments, security, stack, git-flow, ...)
modes/             # language blueprints (go, ts, python, cpp, svelte) shared across agents
templates/         # file templates referenced by skills (e.g. templates/service/*.tmpl)
platforms/
├── claude/
│   ├── settings.json  # global permissions, plugins, allowed commands, hook registration
│   └── hooks/         # global shell hooks triggered by Claude Code events
└── komodo-bridge/
    ├── .mcp.json.tmpl   # copy-paste MCP server registration for new projects
    └── agent-roster.md  # bridge roster contract (loadSystemPrompt, expected agents, bind mount)
scripts/
├── validate-refs.sh           # checks that all ~/.claude/{agents,standards,modes,templates}/... references resolve
├── validate-bridge-roster.sh  # checks ~/.komodo/bridge's agent roster matches agents/ in this repo
└── hooks/git/                  # portable git pre-commit hook templates (no-comments + lint)
.gitignore         # editor/OS noise; this config layer keeps no TODO.md/MEMORY.md of its own
```

Cross-cutting coding standards live in top-level `standards/`, language modes in top-level `modes/`; each agent owns only its role-specific skills and mode folders. See `CLAUDE.md` for the encapsulation model and the per-agent **modes** that keep spawned-agent context lean.

## Setup

Run once after cloning:

```bash
bash setup.sh
```

Symlinks `agents/`, `standards/`, `modes/`, `templates/`, `platforms/claude/settings.json`, and `platforms/claude/hooks/` into `~/.claude/` and removes stale links from the previous layout. Also validates cross-agent references (`scripts/validate-refs.sh`); pass `--dry-run` to preview what would be linked without making changes. Restart Claude Code afterward.

## Git hooks (model-agnostic enforcement floor)

`scripts/hooks/git/` holds portable pre-commit checks that catch **any** agent or human in any IDE — not just Claude Code: `pre-commit-no-comments` (enforces `standards/comments.md` on staged changes; shares its rule logic with the Claude hook via `scripts/lib/comment-rules.awk`) and `pre-commit-lint` (`golangci-lint` / `tsc --noEmit` on staged files). Install per repo by writing a `.git/hooks/pre-commit` that invokes both by absolute path and `chmod +x` it — komodo-ecom automates this with `just init-hooks`.

## Agents

Claude subagents (advisor, business-architect, software-engineer, quality-assurance, devops, hardware-engineer, data-analyst, cyber-security, botanist, logistics), plus MCP-primary agents (`pm`, `qa`, lawyer, marketing, customer-servicing, tax-advisor) on the komodo bridge. The advisor is the default orchestrator — it is the entry point for everything and dispatches specialists with scoped modes. Full roster, triggers, and routing notes: [`CLAUDE.md`](CLAUDE.md).

The bridge reads agent system prompts directly from this repo's `agents/` (bind-mounted, not copied); registration template and roster contract: [`platforms/komodo-bridge/`](platforms/komodo-bridge/).

## Project-level config

Projects keep only project-specific overrides in their own `.claude/settings.json`. Global permissions and agents live here and are not duplicated per-project.
