# Skill: /new-service

Scaffold a new Go microservice in this monorepo following established conventions. This is the **Go specialization of `~/.claude/agents/swe/api/blueprint.md`** — that file defines the language-agnostic shape every API must have; this skill produces it for Go. The generated skeleton must satisfy every applicable row of the blueprint's §2 checklist, with stubs marked per its §3 TODO-pointer convention. After scaffolding, `/api-audit` should report all PASS (stubs aside).

## Usage

```
/new-service <name> <domain> [--lambda|--fargate]
```

- `<name>` — short name without prefix/suffix (e.g. `cart`, `loyalty`, `returns`). Full name becomes `komodo-<name>-api`.
- `<domain>` — domain group for port allocation. Must match one of the domain blocks in the Port Allocation table in `CLAUDE.md`.
- `--lambda` or `--fargate` — compute target. Defaults to `--fargate` if omitted.

## Before generating anything

1. Read `CLAUDE.md` for the Port Allocation table. Scan existing `docker-compose.yaml` files in sibling service directories to find which ports in the domain block are already taken. Pick the next available anchor port.
2. Inspect the published forge SDK from Git — `github.com/rdevitto86/komodo-forge-sdk-go` at the version pinned in the generated `go.mod`, never a local checkout. Pull via raw GitHub or `gh`:
   - `http/server/` — exact `srv.Run` signature.
   - `http/middleware/exports.go` — the middleware list.
   - `http/errors/` — `httpErr.SendError` signature.

## Files to generate

Generate all files under `apis/komodo-<name>-api/`. Read each template from `~/.claude/templates/service/` and substitute `<name>` and `<PORT>` throughout.

| File | Template |
|------|----------|
| `go.mod` | `~/.claude/templates/service/go.mod.tmpl` |
| `main.go` (Fargate) | `~/.claude/templates/service/main-fargate.go.tmpl` |
| `internal/handlers/health.go` | `~/.claude/templates/service/health.go.tmpl` |
| `pkg/v1/models/errors.go` | `~/.claude/templates/service/errors.go.tmpl` |
| `pkg/v1/client/client.go` | `~/.claude/templates/service/client.go.tmpl` |
| `pkg/v1/exports.go` | `~/.claude/templates/service/exports.go.tmpl` |
| `pkg/v1/mocks/exports.go` | `~/.claude/templates/service/mocks.go.tmpl` |
| `Dockerfile` | `~/.claude/templates/service/Dockerfile.tmpl` |
| `docker-compose.yaml` | `~/.claude/templates/service/docker-compose.yaml.tmpl` |

**Lambda target:** same as Fargate but move the bootstrap block from `init()` into `main()`. `srv.Run` auto-detects `AWS_LAMBDA_FUNCTION_NAME`.

**`pkg/v1/models/<name>.go`:** Create with `package models` and a `// TODO: define domain models for <name>-api` placeholder.

**`Makefile`:** Copy from any sibling service, substituting `APP_NAME := komodo-<name>-api` and the correct `AWS_SECRET_PREFIX`.

**`.golangci.yaml`:** Copy verbatim from any sibling service. The `wrapcheck.ignore-package-globs` entry must include the forge SDK glob.

**`docs/` skeleton:**

| File | Minimum content |
|------|----------------|
| `README.md` | Port, run commands, env vars table (copy structure from a sibling) |
| `openapi.yaml` | Minimal OpenAPI 3.1 spec with `/health` GET only |
| `architecture.md` | H1 heading + one-line description |
| `design-decisions.md` | H1 heading only |
| `data-model.md` | H1 heading only |

---

## After generating

1. Remind the developer to:
   - Add the service to `infra/local/services.jsonc` under the correct profile group
   - Add a profile entry in `infra/local/docker-compose.yml` if not auto-included
   - Update the port allocation table in root `CLAUDE.md` (and `MEMORY.md` if it exists)
   - Run `go mod tidy` from inside the service directory
2. Print the allocated port and full service name.

---

**After this, you may need:** `/add-route` to register handlers, `/api-audit` to confirm the skeleton conforms to `~/.claude/agents/swe/api/blueprint.md`, `/git-flow` for branch and PR conventions.
