# Komodo Bridge Agent Roster

Reference for how `~/.komodo/bridge` loads agent system prompts from this repo.

## Loading mechanism

`agents.go`'s `loadSystemPrompt(name string)` reads `agents/<name>/agent.md` relative to the bridge's CWD, strips the YAML frontmatter, and uses the body as that agent's system prompt. Missing file → `log.Fatalf`, which crashes the whole bridge.

## Expected roster

The bridge's `allAgents()` registers exactly these 7 names, 1:1 with this repo's `agents/` directory:

- `business-architect`
- `quality-assurance`
- `lawyer`
- `customer-servicing`
- `marketing`
- `summarizer`
- `tax-advisor`

Each name maps to `agents/<name>/agent.md` in this repo.

## Source of truth

`~/.komodo/`'s docker-compose bind-mounts `${HOME}/komodo-ai-agents/agents` into the container at `/app/agents` (read-only). Agents are not duplicated or copied into `~/.komodo` — this repo is the single source of truth. Edits to `agents/<name>/agent.md` here take effect on the next bridge restart.

## Model routing

Each agent in `agents.go` carries a default model in its `agentDef.model` field, set via `modelFor("<SUFFIX>", "<default>")`. Resolution precedence, highest first:

1. **`OLLAMA_MODEL_<SUFFIX>`** — per-agent override env var. If set and non-empty, wins outright.
2. **Compiled-in default** (the table below) — used when the per-agent env var is unset or empty.
3. **`OLLAMA_MODEL`** — global fallback, applied inside `OllamaClient.Generate` only if the resolved model string is empty (i.e. every agent has a default, so this tier is a safety net for future agents added without one).

| Agent (Go func) | agent.md name | Env var suffix | Default model |
|---|---|---|---|
| `businessArchitectAgent` | business-architect | `OLLAMA_MODEL_BA` | `deepseek-r1:14b` |
| `qaAgent` | quality-assurance | `OLLAMA_MODEL_QA` | `qwen3-coder-next:latest` |
| `lawyerAgent` | lawyer | `OLLAMA_MODEL_LAWYER` | `deepseek-r1:14b` |
| `taxAdvisorAgent` | tax-advisor | `OLLAMA_MODEL_TAX` | `deepseek-r1:14b` |
| `customerServicingAgent` | customer-servicing | `OLLAMA_MODEL_CUSTOMER_SERVICING` | `qwen3:4B` |
| `marketingAgent` | marketing | `OLLAMA_MODEL_MARKETING` | `qwen3:4B` |
| `summarizerAgent` | summarizer | `OLLAMA_MODEL_SUMMARIZER` | `qwen3:1.7B` |

`~/.komodo/docker-compose.yaml` sets all 7 override vars explicitly under `komodo-bridge`'s `environment`, alongside the pre-existing global `OLLAMA_MODEL`. They currently match the compiled-in defaults — kept explicit so the routing is visible and editable from compose without touching Go source. To change an agent's model, edit its `OLLAMA_MODEL_<SUFFIX>` value in compose and restart the bridge; no rebuild required.

## Drift check

`scripts/validate-bridge-roster.sh` extracts every `loadSystemPrompt("<name>")` call from `~/.komodo/bridge/agents.go` and confirms `agents/<name>/agent.md` exists in this repo. No-ops if `~/.komodo/bridge` isn't present.

## New project setup

Copy `platforms/komodo-bridge/.mcp.json.tmpl` to `.mcp.json` at the new project's root to register the bridge as an MCP server.
