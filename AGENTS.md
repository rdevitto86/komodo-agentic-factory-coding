# komodo-agentic-factory-coding

Komodo's code assembly line: one static binary is the conveyor and devices, markdown is all a model reads, one guard is the only hook, a model is a machine mounted per host. `README.md` is the plan and requirements; `BACKLOG.md` is the work.

## Rules that hold here

- **Nothing outside `internal/mount/` names a host.** No vendor, tool, path, or flag anywhere else.
- **Models read markdown, never Go.** Rules, roles, skills, and policy are the only files a machine sees.
- **One agent hook.** The guard on PreToolUse; nothing else runs in the loop.
- **The gate is local.** `komodo gate` runs pre-commit and pre-push, no model; nothing runs on GitHub.
- **No MCP in 1.0.** MCPs land in a later hot-swap pass.
- **No repo config is required;** nothing refuses to run for a missing file.
- **Standard library only.** Go, no dependencies; `bin/` is gitignored, built by `komodo gate --install`.
- **Versions go alpha, beta, release,** as README defines.
- **A pull request body follows `.github/PULL_REQUEST_TEMPLATE.md`.**
- **A rating follows `docs/scorecard.md`:** `origin/main`, from its last row, with evidence.
- **The prototype is history,** at tag `prototype-final`; nothing returns without a task.

## Commands

```bash
go run ./cmd/komodo gate     # the whole precheck
go run ./cmd/komodo lint     # after a backlog edit
go run ./cmd/komodo doctor   # drift and portability
```
