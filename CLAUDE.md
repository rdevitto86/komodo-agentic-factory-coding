# komodo-claude

Shared Claude Code configuration for all Komodo projects. Settings, skills, agents, hooks, and standards live here and are symlinked into `~/.claude/` via `setup.sh`.

## Hard rules

- **Never create git commits or git branches.** Only the user commits, branches, and merges. Do not run `git commit`, `git branch`, or `git checkout -b` under any circumstances — not even when asked to "save", "finalize", or "start on a feature". Always work on the current branch.
- **Error strings must not contain the function name.** Function context belongs in metadata objects or stack traces only — not in the error message string itself.
- **All comments follow `comments.md` exactly — it is the single source of truth for comment rules, every language and every file.** No other file defines comment rules or shows comment examples. The hard violations it defines (file/package-level doc blocks, name-leading doc comments, verbose multi-line blocks, commenting a type merely to describe it) are non-negotiable.
- **Never expand scope without permission.** If you discover work outside the current task — a bug, a refactor opportunity, an adjacent improvement — stop. Document it in the nearest `TODO.md` and surface it to the user or advisor. Side work is always declined unless explicitly approved.

## What this repo is

This is not a product codebase. It is the Claude Code configuration layer — agents, skills, standards, and hooks that are shared across every Komodo project. Changes here affect all projects on the next Claude Code session.

## Directory layout

```
claude/
├── agents/          # Claude Code subagents (spawned via Agent tool)
│                    # Local MCP agents live in ~/.komodo/bridge/ — not here
├── skills/          # User-invocable slash commands (/feature-workflow, /dispatch, etc.)
├── standards/       # Engineering and operational standards referenced by agents and skills
├── hooks/           # Shell scripts triggered automatically by Claude Code events
└── settings.json    # Global permissions, allowed commands, plugins, hook registration
```

## Claude Code agents

Spawned by Claude via the `Agent` tool. Defined in `claude/agents/`.

**Default: the `advisor` agent is the entry point for everything.** It is the consigliere and orchestrator — gathers context (user → MCP agents → Claude agents), delegates work autonomously, and insulates the user from operational noise. The user focuses on high-level decisions, external relationships, and core work. The advisor handles the rest.

| Trigger | Agent | Model | Role |
|---------|-------|-------|------|
| `[ADV]` | `advisor` | sonnet | **Default. Consigliere and orchestrator** — advises on strategy/technical/business, auto-dispatches specialist agents, only surfaces decisions that are consequential. |
| `[ARCH]` | `architect` | opus | Cross-business domain strategy — software, product, commercial, ops, legal. Escalated to by the advisor for formal architecture decisions. |
| `[SWE]` | `swe` | sonnet | Implementation and code design. Secondary: code/architecture review. |
| `[QA]` | `quality-assurance` | sonnet | Security review, performance review, and test writing. **Primary: MCP `qa` agent.** Claude subagent only when MCP is unavailable. |
| `[OPS]` | `devops` | sonnet | CI/CD, infrastructure, deployments, monitoring, incident response. |
| `[EMB]` | `swe-embedded` | sonnet | Embedded systems — RTOS, bare-metal C/C++, device drivers, safety-critical firmware. |
| `[EE]` | `electronics` | sonnet | Circuit design, schematic review, PCB layout, power systems, component selection, EMC. |
| `[MECH]` | `mechatronics` | sonnet | Embedded firmware, actuator/sensor interfaces, hardware-software integration. |
| `[BOT]` | `botanist` | haiku | Crop health, plant science, agricultural diagnosis. |
| `[WM]` | `warehouse-manager` | haiku | Inventory control, logistics, warehouse operations. |

**Model tiers:**
- `haiku` — simple/lookup/drafting tasks
- `sonnet` — complex technical work (default)
- `opus` — highest reasoning demand (architecture)

**Invoking agents:**
- **Shorthand prefix:** start your message with the trigger (e.g. `[SWE] add pagination to the orders endpoint`)
- **Natural language:** "use the swe agent to...", "have the advisor look at..."
- **Skills:** `/dispatch` for parallel multi-agent routing, `/feature-workflow` for structured design → implementation flow

## Local MCP agents

Run on Qwen3 via the komodo bridge (`~/.komodo/bridge`). Served at `http://localhost:8000/sse`. Start with `docker compose up -d` in `~/.komodo/`.

These are invoked as MCP tools, not Claude subagents — they run fully outside Claude's context window. **Always prefer MCP agents over Claude agents when an MCP agent can cover the task.**

`pm` and `qa` are the primary MCP agents — invoke them first for planning and QA work. Claude subagents are the fallback when MCP is unavailable.

| Agent | MCP tool | Role |
|-------|----------|------|
| `pm` ⭐ | `analyze_specs` | Task breakdown, sprint planning, delivery risk, stakeholder communication. |
| `qa` ⭐ | `generate_test_cases` | Security review, performance review, test writing, bug triage, release gates. |
| `lawyer` | `review_document` | Contract review, compliance, legal research. Not legal advice. |
| `customer-servicing` | `draft_response` | Customer response drafting, ticket triage, escalation summaries. |
| `marketing` | `create_content` | Campaign strategy, copywriting, brand messaging. |
| `sales` | `draft_sales_content` | Lead qualification, proposals, negotiation, CRM. |

## Key skills

| Skill | When to use |
|-------|-------------|
| `/git-flow` | Branch naming, commit conventions, PR process |
| `/new-service` | Scaffold a Go microservice |
| `/add-route` | Add a handler + test + OpenAPI stub to an existing Go service |
| `/new-middleware` | Scaffold HTTP middleware in the forge SDK |
| `/new-migration` | Create a database schema migration |
| `/new-tf-module` | Scaffold a Terraform module |
| `/new-page` | Scaffold a SvelteKit 5 page |
| `/new-component` | Scaffold a Svelte 5 component |
| `/new-bom` | Generate or review a Bill of Materials |
| `/circuit-review` | Review an electrical circuit design |
| `/design-review` | Review a mechanical design |
| `/new-ros-node` | Scaffold a ROS 2 node (C++ or Python) |
| `/crop-analysis` | Generate a crop health report |
| `/stock-report` | Generate an inventory report |

## Standards

Engineering and operational standards in `claude/standards/`. Agents reference these by name — do not rename files without updating agent prompts.

| File | Covers |
|------|--------|
| `principles.md` | Hard rules, code reuse priority, DI/testability design doctrine |
| `comments.md` | Single source of truth for all comment rules — every language, every file |
| `security.md` | Secrets, input validation, auth, OWASP, incident response |
| `pull-requests.md` | PR size, descriptions, review duties, merge criteria |
| `api-design.md` | URL conventions, HTTP methods, status codes, versioning, OpenAPI |
| `go.md` | Formatting, error handling, naming, testing, concurrency |
| `typescript.md` | Type safety, naming, async patterns, module conventions |
| `python.md` | Versions, typing, error handling, async, testing, design patterns |
| `svelte.md` | Svelte 5 runes, component structure, SvelteKit conventions, accessibility |
| `testing-ts.md` | JS/TS test file naming (`.x.test.ts`), colocation, single-file structure, SvelteKit and Vue conventions |
| `testing-go.md` | Go test colocation, table-driven structure, mocking at interface boundaries, coverage, race/timing |
| `sql.md` | Schema conventions, migrations, indexing, query safety |
| `logging.md` | Log levels, required fields, what never to log, correlation |
| `observability.md` | Metrics, distributed traces, trace_id propagation, health checks, alerting |
| `docker.md` | Multi-stage builds, non-root images, layer caching, secrets, image scanning |
| `token-efficiency.md` | MCP-first delegation, compaction cadence, lean context passing, model selection |
| `todo.md` | Item format, section headers, no-date rule for audits, what belongs in TODO.md |

## Hooks (auto-run)

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

Symlinks `claude/` contents into `~/.claude/`. Restart Claude Code after running.
