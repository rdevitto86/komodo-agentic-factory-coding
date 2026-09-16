# Komodo bridge

Prompt files for the local-model bridge (`~/.komodo/bridge`), an MCP server over Ollama. The bridge is text in, text out, single shot: no file access, no tools. That decides what lives here.

| Needs files, shell, or web | Lives in |
|---|---|
| No, a pure text transform | here, `agents/<name>/agent.md` |
| Yes | the harness (`komodo/briefs/`) or a Claude Code agent |

## Roster

| Agent | Tool | Why local |
|---|---|---|
| `completion` | `complete_code` | Inline autocomplete needs the lowest latency in the system |
| `summarizer` | `summarize` | Zero-reasoning compression; a frontier token spent shrinking context defeats the point |

The harness reaches the summarizer directly through `komodo/workers/ollama.py` when the `summarizer` role's provider is `ollama`; the bridge serves interactive sessions over MCP. Both read the same model name from the bridge's compose file, never from `agent.md`, whose `model:` line is decorative.

## Limits

The bridge sets no `num_ctx`, so a large payload truncates server-side without an error. Every caller treats it as best-effort on bounded input, and an unreachable bridge is a skipped step, never a blocker.
