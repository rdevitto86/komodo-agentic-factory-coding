# komodo-agentic-factory-coding

Komodo's code assembly line: one static binary is the conveyor and the devices, markdown is everything a model reads, one guard is the only hook, and a model is a machine mounted per host. `README.md` is the plan and the requirements; `BACKLOG.md` is the work. Everything lands on PR #103.

## Layout

| Path | What |
|---|---|
| `komodo/rules/` | Universal rules and the backlog grammar |
| `komodo/roles/` | One file per role |
| `komodo/standards/`, `komodo/briefs/` | V1 source that TG-03.1 reshapes into skills and role bodies |
| `komodo/facets/` | Komodo's setup skill, appendices, and commands per platform, from TG-03.5 on |
| `templates/project/` | Starters for a new repo |
| `cmd/komodo/`, `internal/`, `bin/` | The binary, from TG-03.2 on |

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
go run ./cmd/komodo gate               # vet, test, doctor, guard table, binaries; from TG-03.2 on
go run ./cmd/komodo lint               # after every backlog edit
go run ./cmd/komodo doctor             # references, portability, drift
```
