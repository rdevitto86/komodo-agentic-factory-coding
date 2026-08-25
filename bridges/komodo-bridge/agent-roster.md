# Komodo Bridge Agent Roster

How `~/.komodo/bridge` loads agent system prompts from this repo, and why the roster is small and code-only.

## The constraint that decides placement

**A bridge agent is text-in, text-out, single-shot.** `agentDef` in `agents.go` is a system prompt, a model, and a `buildPrompt` function — no file access, no tool calls, no multi-step loop. Whatever it needs, the caller passes as a string argument.

That fact alone decides where an agent lives:

| Needs file/shell/web access to do its job | Lives in |
|---|---|
| No — pure text transform | The bridge (`bridges/komodo-bridge/agents/`) |
| Yes — has to read, search, or browse | Claude only (`claude-code/agents/`) |

`claude-code/agents/engineering.md` reads files and runs searches — it cannot become a bridge agent without the bridge growing a tool-use loop it does not have. It stays Claude-only, not by preference but by architecture.

## The roster

| Bridge agent | Tool | Why it's here, not on Claude |
|---|---|---|
| `completion` | `complete_code` | Inline autocomplete needs the lowest latency in the system — a local call, not a round trip |
| `summarizer` | `summarize` | Zero-reasoning compression. Spending a Claude token to shrink context defeats the point of shrinking it |

## The ceiling, and why `quality-assurance` was retired

**No request sets `num_ctx` or `num_predict`.** `generateRequest` carries exactly `model`, `system`, `prompt`, `stream` — no `options` object. Every call silently inherits whatever context window the Ollama Modelfile defines, so a large payload is truncated server-side with no error reaching the caller.

**That falls hardest on exactly the two tools built to take large inputs.** A review tool that silently drops half its input returns confident findings about code it never saw, which is worse than no review.

`quality-assurance` is therefore gone from `allAgents()`. Its job is now split: deterministic scanners in CI catch what a checklist catches, and `/audit-bugs`/`/audit-security`/`/audit-simplify` on a fresh context handle judgement — model-agnostic by design, so a bridge-only run still has a review path. `summarize` remains, and should be treated as best-effort on bounded input until the bridge sets a context size.

**A bridge call is never a gate.** Any loop step that calls it treats an unreachable server as a skipped optional step, never a blocker — the hybrid is there for cheap help, not to add a dependency that can stall a build.

Each maps to `bridges/komodo-bridge/agents/<name>/agent.md` — flat frontmatter (`name`, `model`), body is the system prompt verbatim.

**`model:` in that frontmatter is decorative.** `loadSystemPrompt` discards the frontmatter block entirely and returns only the body; real selection is `modelFor(<SUFFIX>, <compiled default>)`. Editing `model:` in an `agent.md` changes nothing — set `OLLAMA_MODEL_<SUFFIX>` in compose instead.

## Loading mechanism

`agents.go`'s `loadSystemPrompt(name)` reads `agents/<name>/agent.md` relative to the container's working directory, strips YAML frontmatter, uses the body as the system prompt.

**A missing file calls `log.Fatalf` and takes the whole bridge down.** Every name in `allAgents()` must have a matching file.

## Bind mount

```yaml
# ~/.komodo/docker-compose.yaml
volumes:
  - ${HOME}/komodo/ai/komodo-agentic-tools-code/bridges/komodo-bridge/agents:/app/agents:ro
```

This repo is the single source of truth — agents are never copied into `~/.komodo`. Edits here take effect on the next bridge restart.

## Two agent directories, never merged

| Directory | Consumer | Layout |
|---|---|---|
| `claude-code/agents/` | Claude Code subagents | flat `<name>.md` |
| `bridges/komodo-bridge/agents/` | The bridge | `<name>/agent.md` |

Claude Code requires the flat form; the bridge's loader requires the nested form. A symlink cannot satisfy both.

## Model routing

`agentDef.model` resolves highest-first:

1. **`OLLAMA_MODEL_<SUFFIX>`** — per-agent override, wins when non-empty
2. **Compiled-in default**
3. **`OLLAMA_MODEL`** — global fallback

| agent.md name | Env suffix | Model | Why |
|---|---|---|---|
| `completion` | `OLLAMA_MODEL_COMPLETION` | `qwen3-coder-next:latest` | Code-shaped output, needs to be fast |
| `summarizer` | `OLLAMA_MODEL_SUMMARIZER` | `qwen3:1.7B` | No reasoning required, smallest model wins |

Change a model by editing its `OLLAMA_MODEL_<SUFFIX>` in compose and restarting. No rebuild.

## Where the old roles went

Every agent that is no longer here was absorbed, not dropped. Nothing needs recreating.

| Former agent | Now |
|---|---|
| `advisor` | The default session |
| `software-engineer` | `engineering` subagent |
| `business-architect`, `marketing`, `lawyer`, `tax-advisor`, `customer-servicing`, `logistics`, `hardware-engineer` | Dropped — no business-domain agent in this repo; revisit if a business/office repo gets built |

The advisory roles had no reason to run off Claude — they were never code review. The three that remain are here because each needs a *different* model than the one calling it.

## Adding a bridge agent

Add it here only if it is a pure text transform. If it needs to read a file or browse the web, it belongs in `claude-code/agents/` instead.

1. Create `bridges/komodo-bridge/agents/<name>/agent.md`.
2. Register it in `allAgents()` with a `modelFor("<SUFFIX>", "<default>")`.
3. Set `OLLAMA_MODEL_<SUFFIX>` in compose.
4. Restart the bridge.

Skipping step 1 crashes the bridge on next start.

## Registering the bridge with a project

Copy `.mcp.json.tmpl` to `.mcp.json` at the project root.
