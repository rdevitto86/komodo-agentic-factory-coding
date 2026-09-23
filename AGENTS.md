# komodo-agentic-factory-coding

Komodo's code assembly line: one static binary is the conveyor and the devices, markdown is everything a model reads, one guard is the only hook, and a model is a machine mounted per host. `README.md` is the plan and the requirements; `BACKLOG.md` is the work. Everything lands on PR #103.

## Layout

| Path | What |
|---|---|
| `komodo/AGENTS.md`, `komodo/rules/` | Universal rules and the backlog grammar |
| `komodo/roles/` | One file per role |
| `komodo/skills/` | `run`, `review`, `backlog`, `respond`, and one `standards-<x>` per language or domain |
| `komodo/policy.json` | The four denials and the critical refs |
| `komodo/facets/` | Komodo's setup skill, appendices, and commands per platform |
| `templates/project/` | Starters for a new repo |
| `cmd/komodo/`, `internal/`, `bin/` | The binary |

## Rules that hold here

- **Nothing outside `internal/mount/` names a host.** No vendor, host tool, host path, or host flag anywhere else.
- **Models read markdown, never Go.** Rules, roles, skills, and policy are the only files a machine sees.
- **One agent hook.** The guard on PreToolUse. Nothing else runs in the loop.
- **The gate is local.** `komodo gate` runs before every commit and push, mechanically, with no model. Nothing runs on GitHub.
- **No MCP in V2.** Machines, skills, and external dependencies are the swappable parts; MCPs come in a later hot-swap pass.
- **No repo config is required.** Nothing refuses to run because a file is missing.
- **Standard library only.** Go with no dependencies; prebuilt binaries under `bin/` with a manifest.
- **V1 is history.** The tag `v1-final` holds it; nothing is copied back without a task naming it. Run state was never committed and is gone.

## Commands

```bash
go run ./cmd/komodo gate               # vet, test, doctor, guard table, binaries
go run ./cmd/komodo lint               # after every backlog edit
go run ./cmd/komodo doctor             # references, portability, drift
```
