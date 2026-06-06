# Komodo Context

Org-wide stack invariants every agent can assume without rediscovery. This file answers "what is the Komodo stack and how is it shaped" so agents stop re-deriving it per task. It does **not** restate engineering rules — those live in `~/.claude/agents/swe/principles.md` and the language/topic standards.

Per-repo specifics (exact port table, run/test/build/deploy commands, CI pipeline) live in each project's `CLAUDE.md`. If a project `CLAUDE.md` conflicts with this file, the project file wins.

---

## 1. Repository shape

- **Monorepo.** Backend services under `apis/`, infrastructure under `infra/`, frontend in its own SvelteKit project. The forge SDKs (`komodo-forge-sdk-go`, `komodo-forge-sdk-ts`) sit alongside services in `apis/`.
- **Service naming.** A backend service is `komodo-<name>-api` (e.g. `komodo-cart-api`). The short `<name>` is used for ports, secret prefixes, and migration paths.

## 2. Languages and runtimes

- **Backend:** Go 1.26. Primary language for all services.
- **Frontend:** TypeScript + SvelteKit 5 (runes). See `~/.claude/agents/swe/svelte/coding.md` and `~/.claude/agents/swe/ts/coding.md`.
- **Python:** tooling and data/ML work only — not a service language. See `~/.claude/agents/swe/python/coding.md`.

## 3. Forge SDKs — reuse before building

`komodo-forge-sdk-go` / `komodo-forge-sdk-ts` are the first stop for any non-trivial logic (see `~/.claude/agents/swe/principles.md` §2 for the full reuse order). The Go SDK provides HTTP server (`http/server`), middleware (`http/middleware`), error responses (`http/errors`), config (`config`), structured logging (`logging/runtime`), and AWS Secrets Manager bootstrap (`aws/secrets-manager`). Do not reimplement what the SDK covers; flag gaps for upstream extraction.

The SDKs are published and consumed as external dependencies — **always import the published packages, never a local-machine copy.** Go import path is `github.com/rdevitto86/komodo-forge-sdk-go` (e.g. `github.com/rdevitto86/komodo-forge-sdk-go/http/server`); the TS package is `@komodo-forge-sdk/typescript`. Do not add a local `replace` directive or a relative/sibling import to point at a checkout on disk unless the user explicitly asks for it (e.g. while developing the SDK itself). The same applies to *reading* SDK source: when an agent needs to confirm a signature, pull it from Git at the pinned version (raw GitHub or `gh`), never from an assumed local path such as `apis/komodo-forge-sdk-go/`.

## 4. Service anatomy

A standard Go service is laid out as:

- `main.go` — bootstrap (logger, secrets) then `srv.Run`.
- `internal/handlers/` — HTTP handlers, including `health.go`.
- `pkg/v1/` — the versioned public surface: `models/`, `client/`, `mocks/`, and `exports.go`.
- `docs/` — `README.md`, `openapi.yaml` (OpenAPI 3.1), `architecture.md`, `design-decisions.md`, `data-model.md`.

API conventions (versioning, status codes, error format) are governed by `~/.claude/agents/swe/api/design.md`.

## 5. Compute and deployment

- **Cloud:** AWS.
- **Default compute:** ECS Fargate. Lambda is opt-in; `srv.Run` auto-detects the Lambda runtime via `AWS_LAMBDA_FUNCTION_NAME`, so the same service binary targets either without code changes.
- **Containers:** multi-stage, non-root images per `~/.claude/agents/devops/docker/standard.md`.

## 6. Configuration and secrets

- Runtime config is read through the forge SDK `config` package (`config.GetConfigValue`), never hardcoded.
- Secrets load from **AWS Secrets Manager** via the SDK bootstrap, scoped per service by `AWS_SECRET_PREFIX`. See `~/.claude/agents/swe/security.md` for handling rules.

## 7. Data and migrations

- **Database:** PostgreSQL. Conventions (UUID primary keys via `gen_random_uuid()`, `TIMESTAMPTZ` timestamps) are in `~/.claude/agents/swe/db/sql.md`.
- **Migrations:** live under the service's `db/migrations/` as `<sequence>_<description>.up.sql` / `.down.sql`. The migration tool is per-service (e.g. `golang-migrate`, `goose`) — confirm via the service `go.mod` before generating.

## 8. Infrastructure as code

- **Terraform.** Reusable modules in `infra/modules/`, per-environment instantiation in `infra/environments/`, AWS provider.
- Module types: `service` (ECS Fargate / Lambda), `queue` (SQS), `storage` (S3 / DynamoDB), `network` (VPC / subnets), `shared` (reusable).

## 9. Local development

- Each service ships a `docker-compose.yaml` joined to the external `komodo-network`, exposing a `/health` endpoint for the container healthcheck.
- Ports are allocated in domain-grouped blocks; the authoritative port table lives in the project `CLAUDE.md`.

## 10. Observability

Structured logging through the SDK `logging/runtime` package per `~/.claude/agents/swe/logging.md`; metrics, traces, and `trace_id` propagation per `~/.claude/agents/devops/observability/standard.md`.
