# komodo-claude

Shared Claude Code configuration for all Komodo projects. Agents, hooks, and settings live here and are symlinked into `~/.claude/` via `setup.sh`. Each agent is a self-contained module that owns its own standards, skills, modes, and docs — there is no global standards or skills directory.

## Token efficiency

Agent usage runs against a shared subscription — treat tokens like money. Prefer MCP agents over Claude agents (MCP runs outside Claude's context window at zero token cost). Pass scoped `MODES:` on every Claude agent spawn. Compact at phase boundaries. Keep delegation lean — summarize before passing context downstream. Full doctrine and all enforcement rules: `~/.claude/agents/advisor/token-efficiency.md`. The advisor is responsible for enforcing this on every dispatch.

## Hard rules

These are global and non-negotiable. They apply to every agent and override any default behavior.

- **Never create git commits or git branches.** Only the user commits, branches, and merges. Do not run `git commit`, `git branch`, or `git checkout -b` under any circumstances — not even when asked to "save", "finalize", or "start on a feature". Always work on the current branch.
- **Error strings must not contain the function name.** Function context belongs in metadata objects or stack traces only — not in the error message string itself.
- **All comments follow `comments.md` exactly — it is the single source of truth for comment rules, every language and every file.** It lives at `claude/agents/swe/comments.md`. No other file defines comment rules or shows comment examples. The hard violations it defines (file/package-level doc blocks, name-leading doc comments, verbose multi-line blocks, commenting a type merely to describe it) are non-negotiable.
- **Never expand scope without permission.** If you discover work outside the current task — a bug, a refactor opportunity, an adjacent improvement — stop. Document it in the nearest `TODO.md` and surface it to the user or advisor. Side work is always declined unless explicitly approved.

## What this repo is

This is the Claude Code configuration layer — not a product codebase. Changes here affect all projects on the next Claude Code session.

## Working files: `TODO.md` and `MEMORY.md`

Two local, git-ignored files that every agent reads and writes. Neither is ever committed.

- **`TODO.md` — task tracker.** Where agents track outstanding work and its progress across many sessions: deferred work, out-of-scope finds, known debt. Items are removed when done, never checked off. Conventions: `~/.claude/agents/project-manager/todo.md`.
- **`MEMORY.md` — session cache.** A continuity checkpoint so progress isn't lost when a session ends, compacts, or is interrupted: what's in flight now, the next steps, decisions made this session, and watch-outs. Working state, not history — pruned and overwritten, not appended forever. Conventions: `~/.claude/agents/project-manager/memory.md`.

The split: `TODO.md` is the durable to-do ledger across sessions; `MEMORY.md` is the resume buffer for the work in flight. Read both at session start and reconcile `MEMORY.md` against the real repo state before trusting it. Both are git-ignored (`.gitignore`).

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

Encapsulation is the organizing principle: an agent's knowledge lives in its own folder so spawned agents don't bleed context into each other. Cross-cutting coding standards (principles, comments, security, PRs, stack, git-flow, logging, readme-maintenance) are owned by `swe`, since all coding routes through it; other agents reference them by `~/.claude/agents/swe/<file>.md` path. The self-contained agents (`quality-assurance`, `project-manager`) inline their rules because they also run as MCP agents with no file access.

## Claude Code agents

Spawned via the `Agent` tool. Defined in `claude/agents/<agent>/agent.md`.

**Default: the `advisor` agent is the entry point for everything.** It is the consigliere and orchestrator — gathers context (user → MCP agents → Claude agents), delegates work autonomously, passes the right modes on each spawn, and insulates the user from operational noise.

| Trigger | Agent | Model | Primary runtime | Fallback | Role |
|---------|-------|-------|-----------------|----------|------|
| `[ADV]` | `advisor` | sonnet | Claude Sonnet | — | **Default. Consigliere, orchestrator, and cross-domain strategist** — advises on strategy/technical/business, decomposes work, dispatches specialists with scoped modes, surfaces only consequential decisions. |
| `[PM]` | `project-manager` | sonnet | MCP `pm` (`analyze_specs`) | Claude Sonnet | Story/work-item management (tracker-agnostic: Trello/TODO.md/JIRA) and business-context assembly for other agents. |
| `[SWE]` | `swe` | sonnet | Claude Sonnet | — | All software — UI, backend, **non-robotics** embedded, system design, architecture. Code review/debugging. All coding routes here except robotics. |
| `[QA]` | `quality-assurance` | sonnet | Claude Sonnet | — | Security review, performance review, test writing. |
| `[OPS]` | `devops` | sonnet | Claude Sonnet | — | CI/CD, infrastructure, deployments, monitoring, incident response. Delegates IaC authoring to `swe`. |
| `[DATA]` | `data-analyst` | sonnet | Claude Sonnet | — | Analytics, BI, KPI/metrics definition and interpretation, ad-hoc data investigation, analytical SQL, dashboards/reporting, A/B experiment analysis. Consumes data; does not author pipelines, ETL, schema, or migrations — that routes to `swe`. |
| `[EE]` | `electrical-engineer` | sonnet | Claude Sonnet | — | Circuit design, schematic review, PCB layout, power systems, component selection, EMC. |
| `[MECH]` | `mechatronics` | sonnet | Claude Sonnet | — | **Robotics end to end — hardware AND the robotics software** (firmware, RTOS, ROS 2 nodes), actuator/sensor interfaces, integration, design reviews. |
| `[CAD]` | `machinist` | sonnet | Claude Sonnet | — | CAD / 3D modeling / CAM — parametric part design, DFM for 3D printing and CNC, tolerances/fits, material selection, toolpath and G-code awareness, model review. Tools: Fusion 360, FreeCAD, SolidWorks, AutoCAD, slicers. |
| `[BOT]` | `botanist` | haiku | Claude Haiku | — | Crop health, plant science, agricultural diagnosis. |
| `[WM]` | `logistics` | haiku | Claude Haiku | — | Warehouse management, inventory control, fulfillment, logistics planning, receiving/shipping, WMS. Trigger is `[WM]` (warehouse-management mnemonic) — agent name is `logistics` but scope is the full logistics domain. |
| — | `lawyer` | sonnet | MCP `lawyer` (`review_document`) | Claude Sonnet | Contract review, compliance, legal research. Not legal advice. |
| — | `customer-servicing` | sonnet | MCP `customer-servicing` (`draft_response`) | Claude Sonnet | Customer response drafting, ticket triage, escalation summaries. |
| `[SEC]` | `cyber-security` | sonnet | Claude Sonnet | — | Offensive + defensive security — threat modeling, pentest, security architecture, red/blue-team. Dev-time security stays with `swe`; per-file review stays with `qa`. |
| — | `marketing` | sonnet | MCP `marketing` (`create_content`) | Claude Sonnet | Campaign strategy, copywriting, brand messaging, and sales-content/proposal drafting (supplementing, not replacing, real sellers). |
| `[TAX]` | `tax` | sonnet | MCP `tax` (`analyze_tax`) | Claude Sonnet | Tax document summarization, exposure/deduction/deadline flagging, first-line due diligence. Not tax advice — escalate to a CPA. |

**Runtime rule:** Claude Sonnet is the universal fallback — if MCP is unreachable, always fall back to the Claude Sonnet subagent. The advisor honors this table at dispatch and never skips MCP for agents where it is the primary runtime.

**Routing notes:**
- **Robotics → `mechatronics`.** It owns the robotics software side (firmware, control loops, ROS nodes). `swe`'s `cpp` mode is for non-robotics embedded only. The advisor leans to mechatronics for anything robotics.
- **Cross-domain/architecture strategy → `advisor`** (the former `architect` role folded in). Deep software/system design → `swe` in `design` mode.
- **CAD/mechanical split:** `machinist` owns part geometry and manufacturability (3D printing, CNC). Finished parts hand off to `mechatronics` for robotics integration and firmware/control. `electrical-engineer` owns PCB layout and electronics-driven enclosure constraints — coordinate with EE when a part must accommodate board mounting or connector cutouts.
- **Security split — three layers:** `swe` owns dev-time security (input validation, authz, secret handling) per `~/.claude/agents/swe/security.md`. `quality-assurance` owns per-file/per-component security review passes. `cyber-security` owns everything else: cross-cutting threat modeling, security architecture, offensive/pentest, red/blue-team, and deep vulnerability assessment.
- **Sales duties** fold into `advisor` (commercial strategy), `marketing` (campaigns, copy, and proposal/sales-content drafting), and `data-analyst` (sales-data analysis).
- **Email is a per-agent mode** (`MODES: email`) on `customer-servicing`, `marketing`, `lawyer`, and `tax`. Each owns its own tone, formatting, and business rules in its `email.md`; Gmail (MCP) is the shared transport. Email is read-first — an agent never sends without explicit per-message user approval, and never bulk-sends through it.

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

Note: "mode" means two different things depending on the agent. For most agents (`swe`, `mechatronics`, `devops`, etc.) a mode is a loadable folder/file bundle activated by keyword. For `quality-assurance`, a mode is the review type (`security` | `performance` | `tests`) — same word, different mechanism.

## MCP bridge

MCP agents run on Qwen3 via the komodo bridge (`~/.komodo/bridge`). Served at `http://localhost:8000/sse`. Start with `docker compose up -d` in `~/.komodo/`. They run fully outside Claude's context window at zero token cost. MCP tool names and roles are in the routing table above (Primary runtime column). Claude subagents in `claude/agents/` are the fallbacks when the bridge is unreachable.

## Standards & skills

There is no global standards or skills directory — each is owned by an agent and lives in its folder, grouped by mode. To find a standard or skill, look in the owning agent's folder (or its mode subfolders). Cross-cutting coding standards live under `claude/agents/swe/`: principles, comments, security, PRs, stack, git-flow, logging, and readme-maintenance.

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
