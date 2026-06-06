# komodo-claude

Shared Claude Code configuration for all Komodo projects. Agents, hooks, and settings live here and are symlinked into `~/.claude/` via `setup.sh`. Each agent is a self-contained module that owns its own standards, skills, modes, and docs — there is no global standards or skills directory.

## Hard rules

These are global and non-negotiable. They apply to every agent and override any default behavior.

- **Never create git commits or git branches.** Only the user commits, branches, and merges. Do not run `git commit`, `git branch`, or `git checkout -b` under any circumstances — not even when asked to "save", "finalize", or "start on a feature". Always work on the current branch.
- **Error strings must not contain the function name.** Function context belongs in metadata objects or stack traces only — not in the error message string itself.
- **All comments follow `comments.md` exactly — it is the single source of truth for comment rules, every language and every file.** It lives at `claude/agents/swe/comments.md`. No other file defines comment rules or shows comment examples. The hard violations it defines (file/package-level doc blocks, name-leading doc comments, verbose multi-line blocks, commenting a type merely to describe it) are non-negotiable.
- **Never expand scope without permission.** If you discover work outside the current task — a bug, a refactor opportunity, an adjacent improvement — stop. Document it in the nearest `TODO.md` and surface it to the user or advisor. Side work is always declined unless explicitly approved.

## What this repo is

This is the Claude Code configuration layer — not a product codebase. Changes here affect all projects on the next Claude Code session.

## Directory layout

```
claude/
├── agents/              # Claude Code subagents; each folder is a self-contained module
│   └── <agent>/
│       ├── agent.md     # the directive (frontmatter name = agent identity)
│       ├── <standard>.md# always-on standards owned by this agent
│       └── <mode>/      # mode folder: standards + skills + docs/ for one mode
│                        # Local MCP agents live in ~/.komodo/bridge/ — not here
├── hooks/               # Shell scripts triggered automatically by Claude Code events (global)
└── settings.json        # Global permissions, allowed commands, plugins, hook registration
```

Encapsulation is the organizing principle: an agent's knowledge lives in its own folder so spawned agents don't bleed context into each other. Cross-cutting coding standards (principles, comments, security, PRs, stack, git-flow, logging) are owned by `swe`, since all coding routes through it; other agents reference them by `~/.claude/agents/swe/<file>.md` path. The self-contained agents (`quality-assurance`, `project-manager`) inline their rules because they also run as MCP agents with no file access.

## Claude Code agents

Spawned via the `Agent` tool. Defined in `claude/agents/<agent>/agent.md`.

**Default: the `advisor` agent is the entry point for everything.** It is the consigliere and orchestrator — gathers context (user → MCP agents → Claude agents), delegates work autonomously, passes the right modes on each spawn, and insulates the user from operational noise.

| Trigger | Agent | Model | Role |
|---------|-------|-------|------|
| `[ADV]` | `advisor` | sonnet | **Default. Consigliere, orchestrator, and cross-domain strategist** — advises on strategy/technical/business, decomposes work, dispatches specialists with scoped modes, surfaces only consequential decisions. |
| `[PM]` | `project-manager` | sonnet | Story/work-item management (tracker-agnostic: Trello/TODO.md/JIRA) and business-context assembly for other agents. **Primary: MCP `pm`**; Claude subagent fallback. |
| `[SWE]` | `swe` | sonnet | All software — UI, backend, **non-robotics** embedded, system design, architecture. Code review/debugging. All coding routes here except robotics. |
| `[QA]` | `quality-assurance` | sonnet | Security review, performance review, test writing. **Primary: MCP `qa`**; Claude subagent fallback. |
| `[OPS]` | `devops` | sonnet | CI/CD, infrastructure, deployments, monitoring, incident response. Delegates IaC authoring to `swe`. |
| `[EE]` | `electrical-engineer` | sonnet | Circuit design, schematic review, PCB layout, power systems, component selection, EMC. |
| `[MECH]` | `mechatronics` | sonnet | **Robotics end to end — hardware AND the robotics software** (firmware, RTOS, ROS 2 nodes), actuator/sensor interfaces, integration, design reviews. |
| `[BOT]` | `botanist` | haiku | Crop health, plant science, agricultural diagnosis. |
| `[WM]` | `logistics` | haiku | Inventory control, logistics, warehouse operations. |

**Routing notes:**
- **Robotics → `mechatronics`.** It owns the robotics software side (firmware, control loops, ROS nodes). `swe`'s `cpp` mode is for non-robotics embedded only. The advisor leans to mechatronics for anything robotics.
- **Cross-domain/architecture strategy → `advisor`** (the former `architect` role folded in). Deep software/system design → `swe` in `design` mode.

**Model tiers:** `haiku` (simple/lookup/drafting) · `sonnet` (complex technical work, default) · `opus` (highest reasoning).

**Invoking agents:**
- **Shorthand prefix:** `[SWE] add pagination to the orders endpoint`. Carry modes in the prefix: `[SWE: go, api]`.
- **Natural language:** "use the swe agent to…", "have the advisor look at…"
- **Orchestration:** the advisor decomposes work and dispatches specialists in parallel natively — no routing skill required.

## Modes

Each agent's knowledge is split into **modes** — keyword-activated bundles (a folder, or always-on root files). A spawned agent loads **only** the active modes and ignores the rest, keeping context lean and preventing bleed.

- **Activation:** pass a `MODES:` line on spawn (e.g. `MODES: go, api`) or carry it in the trigger (`[SWE: go, api]`). The advisor scopes modes to exactly what the task needs.
- **Inference:** with no `MODES:` line, the agent infers from the working tree and states what it enabled.
- **Mode tables** live in each `agent.md`. Example — `swe`: `go`, `ts`, `python`, `svelte`, `cpp`, `api`, `db`, `infra`, `design`.
- **Docs are mode-gated:** a mode folder may hold a `docs/` of targeted context that loads with the mode; project-specific context lives in the target repo's `/docs/`.

## Local MCP agents

Run on Qwen3 via the komodo bridge (`~/.komodo/bridge`). Served at `http://localhost:8000/sse`. Start with `docker compose up -d` in `~/.komodo/`. They run fully outside Claude's context window. **Always prefer MCP agents over Claude agents when an MCP agent can cover the task.**

`pm` and `qa` are the primary MCP agents — invoke them first.

| Agent | MCP tool | Role |
|-------|----------|------|
| `pm` ⭐ | `analyze_specs` | Story/work-item management (Trello/TODO.md/JIRA), task breakdown, sprint planning, delivery risk, and business-context assembly for other agents. |
| `qa` ⭐ | `generate_test_cases` | Security review, performance review, test writing, bug triage, release gates. |
| `lawyer` | `review_document` | Contract review, compliance, legal research. Not legal advice. |
| `customer-servicing` | `draft_response` | Customer response drafting, ticket triage, escalation summaries. |
| `marketing` | `create_content` | Campaign strategy, copywriting, brand messaging. |
| `sales` | `draft_sales_content` | Lead qualification, proposals, negotiation, CRM. |

The `pm` and `qa` Claude subagents in `claude/agents/` are the fallbacks when the MCP bridge is unavailable.

## Standards & skills

There is no global standards or skills directory — each is owned by an agent and lives in its folder, grouped by mode. To find a standard or skill, look in the owning agent's folder (or its mode subfolders). Cross-cutting coding standards live under `claude/agents/swe/`.

## Hooks (auto-run, global)

Hooks are registered globally in `settings.json` and fire on events — they cannot be scoped per-agent, but event + content matching already separates concerns (a PM agent never triggers the lint hook because it never edits code).

| Hook | Trigger | What it does |
|------|---------|--------------|
| `post-edit-lint.sh` | After Edit or Write | Runs `golangci-lint` (Go) or `tsc --noEmit` (TS/Svelte) |
| `stop-summary.sh` | Session end | Shows git diff summary if uncommitted changes exist |
| `post-pr-trello.sh` | After `gh pr create` | Surfaces PR URL and Trello card reminder |

## Setup

Run once after cloning:

```bash
bash setup.sh
```

Symlinks `claude/` contents into `~/.claude/` and removes stale links from the old layout. Restart Claude Code after running.
