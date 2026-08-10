# Komodo Bridge Agent Roster

How `~/.komodo/bridge` loads agent system prompts from this repo, and why the roster is small and code-only.

## The constraint that decides placement

**A bridge agent is text-in, text-out, single-shot.** `agentDef` in `agents.go` is a system prompt, a model, and a `buildPrompt` function — no file access, no tool calls, no multi-step loop. Whatever it needs, the caller passes as a string argument.

That fact alone decides where an agent lives:

| Needs file/shell/web access to do its job | Lives in |
|---|---|
| No — pure text transform | The bridge (`platforms/komodo-bridge/agents/`) |
| Yes — has to read, search, or browse | Claude only (`home/agents/`) |

`home/agents/engineering.md` and `home/agents/business.md` read files and run searches — they cannot become bridge agents without the bridge growing a tool-use loop it does not have. They stay Claude-only, not by preference but by architecture.

## The roster

| Bridge agent | Tool | Why it's here, not on Claude |
|---|---|---|
| `completion` | `complete_code` | Inline autocomplete needs the lowest latency in the system — a local call, not a round trip |
| `quality-assurance` | `review_code` | Independent review only means something on a **different** model from the one that wrote the code. Claude reviewing Claude's own work is not a second set of eyes |
| `summarizer` | `summarize` | Zero-reasoning compression. Spending a Claude token to shrink context defeats the point of shrinking it |

Each maps to `platforms/komodo-bridge/agents/<name>/agent.md` — flat frontmatter (`name`, `model`), body is the system prompt verbatim.

## Loading mechanism

`agents.go`'s `loadSystemPrompt(name)` reads `agents/<name>/agent.md` relative to the container's working directory, strips YAML frontmatter, uses the body as the system prompt.

**A missing file calls `log.Fatalf` and takes the whole bridge down.** Every name in `allAgents()` must have a matching file.

## Bind mount

```yaml
# ~/.komodo/docker-compose.yaml
volumes:
  - ${HOME}/komodo/ai/komodo-agentic-config/platforms/komodo-bridge/agents:/app/agents:ro
```

This repo is the single source of truth — agents are never copied into `~/.komodo`. Edits here take effect on the next bridge restart.

## Two agent directories, never merged

| Directory | Consumer | Layout |
|---|---|---|
| `home/agents/` | Claude Code subagents | flat `<name>.md` |
| `platforms/komodo-bridge/agents/` | The bridge | `<name>/agent.md` |

Claude Code requires the flat form; the bridge's loader requires the nested form. A symlink cannot satisfy both.

## Model routing

`agentDef.model` resolves highest-first:

1. **`OLLAMA_MODEL_<SUFFIX>`** — per-agent override, wins when non-empty
2. **Compiled-in default**
3. **`OLLAMA_MODEL`** — global fallback

| agent.md name | Env suffix | Model | Why |
|---|---|---|---|
| `completion` | `OLLAMA_MODEL_COMPLETION` | `qwen3-coder-next:latest` | Code-shaped output, needs to be fast |
| `quality-assurance` | `OLLAMA_MODEL_QA` | `qwen3-coder-next:latest` | Same family as completion — both read code |
| `summarizer` | `OLLAMA_MODEL_SUMMARIZER` | `qwen3:1.7B` | No reasoning required, smallest model wins |

Change a model by editing its `OLLAMA_MODEL_<SUFFIX>` in compose and restarting. No rebuild.

## Where the old roles went

Every agent that is no longer here was absorbed, not dropped. Nothing needs recreating.

| Former agent | Now |
|---|---|
| `advisor` | The default session |
| `software-engineer` | `engineering` subagent |
| `business-architect`, `marketing` | `business` subagent, `backlog` skill |
| `lawyer`, `tax-advisor` | `business` subagent |
| `customer-servicing`, `logistics` | `business` subagent |
| `hardware-engineer` | `business` subagent |

The advisory roles had no reason to run off Claude — they were never code review. The three that remain are here because each needs a *different* model than the one calling it.

## Adding a bridge agent

Add it here only if it is a pure text transform. If it needs to read a file or browse the web, it belongs in `home/agents/` instead.

1. Create `platforms/komodo-bridge/agents/<name>/agent.md`.
2. Register it in `allAgents()` with a `modelFor("<SUFFIX>", "<default>")`.
3. Set `OLLAMA_MODEL_<SUFFIX>` in compose.
4. Restart the bridge.

Skipping step 1 crashes the bridge on next start.

## Registering the bridge with a project

Copy `.mcp.json.tmpl` to `.mcp.json` at the project root.
