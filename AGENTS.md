# komodo-agentic-factory-coding

Komodo's code assembly line: one static binary is the conveyor and devices, markdown is all a model reads, one guard is the only hook, a model is a machine mounted per host. `README.md` is the plan and requirements; `BACKLOG.md` is the work.

## Rules that hold here

- **Nothing outside `internal/mount/` names a host.** No vendor, tool, path, or flag anywhere else.
- **Models read markdown, never Go.** Rules, roles, skills, and policy are the only files a machine sees.
- **One agent hook.** The guard on PreToolUse; nothing else runs in the loop.
- **The gate is local.** `komodo gate` runs pre-commit and pre-push, no model; nothing runs on GitHub.
- **No MCP in 1.0.** Machines, skills, and dependencies are swappable parts; MCPs land in a later hot-swap pass.
- **No repo config is required.** Nothing refuses to run for a missing file.
- **Standard library only.** Go, no dependencies; `bin/` is gitignored, built by `komodo gate --install`.
- **Versions are alpha, beta, then release.** A beta becomes `x.y.z` only once both proofs are in the changelog; README's Versions section defines each stage.
- **The prototype is history.** Tag `prototype-final` holds it, released as `1.0.0-alpha.1`–`.4`; nothing returns without a named task; run state was never committed, and is gone.

## Commands

```bash
go run ./cmd/komodo gate               # vet, test, doctor, guard table, comments
go run ./cmd/komodo lint               # after every backlog edit
go run ./cmd/komodo doctor             # references, portability, drift
```
