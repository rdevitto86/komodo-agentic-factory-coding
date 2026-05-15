# komodo-claude

Shared Claude Code configuration for all Komodo projects. Settings, skills, agents, hooks, and standards live here and are symlinked into `~/.claude/` via `setup.sh`.

## Hard rules

- **Never create git commits.** Only the user commits and merges code. Do not run `git commit` under any circumstances, even when asked to "save" or "finalize" work.
- **Error strings must not contain the function name.** Function context belongs in metadata objects or stack traces only — not in the error message string itself.
- **Doc comments must not reference the function/method name.** Write `// Returns the user for the given ID` not `// GetUser returns the user for the given ID`.

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
| `[SWE]` | `swe` | sonnet | Implementation, code review, debugging, refactoring, CI/CD, security, performance. |
| `[TEST]` | `swe-qa` | sonnet | Focused test writing for a single file/component. Fallback — prefer MCP `qa` agent. |
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

These are invoked as MCP tools, not Claude subagents — they run fully outside Claude's context window.

| Agent | MCP tool | Role |
|-------|----------|------|
| `pm` | `analyze_specs` | Task breakdown, sprint planning, delivery risk, stakeholder communication. |
| `qa` | `generate_test_cases` | Test planning, QA review, bug triage, release gates. |
| `lawyer` | `review_document` | Contract review, compliance, legal research. Not legal advice. |
| `customer-servicing` | `draft_response` | Customer response drafting, ticket triage, escalation summaries. |
| `marketing` | `create_content` | Campaign strategy, copywriting, brand messaging. |
| `sales` | `draft_sales_content` | Lead qualification, proposals, negotiation, CRM. |

## Key skills

| Skill | When to use |
|-------|-------------|
| `/git-flow` | Branch naming, commit conventions, PR process |
| `/new-service` | Scaffold a Go microservice |
| `/new-migration` | Create a database schema migration |
| `/new-tf-module` | Scaffold a Terraform module |
| `/new-page` | Scaffold a SvelteKit 5 page |
| `/new-component` | Scaffold a Svelte 5 component |

See `claude/skills/` for the full list.

## Standards

Engineering and operational standards in `claude/standards/`. Agents reference these by name — do not rename files without updating agent prompts.

| File | Covers |
|------|--------|
| `security.md` | Secrets, input validation, auth, OWASP, incident response |
| `pull-requests.md` | PR size, descriptions, review duties, merge criteria |
| `api-design.md` | URL conventions, HTTP methods, status codes, versioning, OpenAPI |
| `go.md` | Formatting, error handling, naming, testing, concurrency |
| `typescript.md` | Type safety, naming, async patterns, module conventions |
| `python.md` | Versions, typing, error handling, async, testing, design patterns |
| `testing.md` | JS/TS test file naming (`.x.test.ts`), colocation, single-file structure, SvelteKit and Vue conventions |
| `testing-go.md` | Go test colocation, table-driven structure, mocking at interface boundaries, coverage, race/timing |
| `comments.md` | Inline comments stay short, doc comments stay concise; no verbose narration or restating code |
| `sql.md` | Schema conventions, migrations, indexing, query safety |
| `logging.md` | Log levels, required fields, what never to log, correlation |
| `token-efficiency.md` | MCP-first delegation, compaction cadence, lean context passing, model selection |

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
