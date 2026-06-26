# Komodo Context

Org-wide stack invariants every agent can assume without rediscovery. This file answers "what is the Komodo stack and how is it shaped" so agents stop re-deriving it per task. It does **not** restate engineering rules — those live in `~/.claude/standards/principles.md` and the language/topic standards.

Per-repo specifics (exact port, run/test/build/deploy commands, CI pipeline) live in each project's `CLAUDE.md`. If a project `CLAUDE.md` conflicts with this file, the project file wins.

---

## 1. Repository shape

- **Multi-repo.** Repos live under `~/komodo/<domain>/` grouped by business domain. Each service, SDK, UI, and tool is its own git repository.

| Domain | Path | Contents |
|--------|------|----------|
| `commerce/` | storefront-ui, cart, order, order-reservations, search, shop-items, shop-inventory, shop-promotions, user | Customer-facing commerce |
| `platform/` | auth, events, features, insights, ssr-engine-svelte, statistics | Shared platform services |
| `payments/` | payments | Payment processing |
| `fulfillment/` | address, shipping | Logistics and geo |
| `customer/` | communications, loyalty, support, customer-servicing-ui | Post-sale CX |
| `dashboards/` | api-dashboard, feature-toggles-dashboard | Internal tooling |
| `sdk/` | forge-sdk-go, forge-sdk-ts | Shared libraries |
| `ai/` | ai-agents (this repo), ai-guardrails | Agent config and AI services |
| `agtech/` | agtech-ahr | Agriculture vertical |
| `inventory/` | inventory-management-engine | Warehouse/inventory |

- **Service naming.** A backend service is `komodo-<name>-api` (e.g. `komodo-cart-api`). The short `<name>` is used for ports, secret prefixes, and migration paths.

## 2. Languages and runtimes

- **Backend:** Go 1.26. Primary language for all services.
- **Frontend:** TypeScript + SvelteKit 5 (runes). See `~/.claude/modes/svelte/coding.md` and `~/.claude/modes/ts/coding.md`. Some projects use Vue 3 + Composition API instead — see `~/.claude/modes/vue/coding.md`. The project's own `CLAUDE.md` is authoritative on which frontend framework is in use, per the "project file wins" note above.
- **Python:** tooling and data/ML work only — not a service language. See `~/.claude/modes/python/coding.md`.

## 3. Forge SDKs — reuse before building

`komodo-forge-sdk-go` / `komodo-forge-sdk-ts` are the first stop for any non-trivial logic (see `~/.claude/standards/principles.md` §2 for the full reuse order). The Go SDK provides HTTP server (`http/server`), middleware (`http/middleware`), error responses (`http/errors`), config (`config`), structured logging (`logging/runtime`), and AWS Secrets Manager bootstrap (`aws/secrets-manager`). Do not reimplement what the SDK covers; flag gaps for upstream extraction.

The SDKs are published and consumed as external dependencies — **always import the published packages, never a local-machine copy.** Go import path is `github.com/rdevitto86/komodo-forge-sdk-go` (e.g. `github.com/rdevitto86/komodo-forge-sdk-go/http/server`); the TS package is `@komodo-forge-sdk/typescript`. Do not add a local `replace` directive or a relative/sibling import to point at a checkout on disk unless the user explicitly asks for it (e.g. while developing the SDK itself). The same applies to *reading* SDK source: when an agent needs to confirm a signature, pull it from Git at the pinned version (raw GitHub or `gh`), never from an assumed local path such as `apis/komodo-forge-sdk-go/`.

## 4. Service anatomy

A standard Go service is laid out as:

```
komodo-<name>-api/
├── cmd/
│   ├── public/main.go      # Customer-facing entrypoint
│   └── private/main.go     # Service-to-service entrypoint
├── internal/               # Handlers, domain logic, DB adapters
│   └── clients/<svc>client/ # Generated downstream clients (oapi-codegen)
├── openapi.yaml            # Contract source of truth (OpenAPI 3.1)
├── docs/
│   ├── README.md           # Routes, port, env, commands
│   ├── architecture.md
│   ├── design-decisions.md
│   └── data-model.md
└── docker-compose.yaml     # Joins external komodo-network
```

**Entrypoint convention** — services split binaries by audience via `cmd/`:

| Dir | Audience | Middleware stack |
|-----|----------|-----------------|
| `cmd/public/` | Browser / customer-facing | RequestID, Telemetry, RateLimiter, CORS, SecurityHeaders, Auth, CSRF, Normalization, Sanitization |
| `cmd/private/` | Service-to-service | RequestID, Telemetry, Auth, Scope |

A service may be public-only (cart, support), private-only (event-bus, communications), or both (auth, user, order). No flat `main.go` at service root. Rust services use `src/bin/public.rs`/`private.rs`. Lambda: same `cmd/` convention, one binary per function.

**OpenAPI and codegen** — `openapi.yaml` at the service root is the contract source of truth. Each consumer generates downstream clients into its own `internal/clients/<service>client/` via `oapi-codegen`, sourcing the provider's `openapi.yaml` by relative path. No shared client package. Error code ranges: `komodo-forge-sdk-go/http/errors/ranges.go`; read `codes.go` before defining new codes.

API conventions (versioning, status codes, error format) are governed by `~/.claude/agents/software-engineer/api/design.md`.

## 5. Compute and deployment

- **Cloud:** AWS.
- **Default compute:** ECS Fargate. Lambda is opt-in; `srv.Run` auto-detects the Lambda runtime via `AWS_LAMBDA_FUNCTION_NAME`, so the same service binary targets either without code changes.
- **Containers:** multi-stage, non-root images per `~/.claude/agents/devops/docker/standard.md`.

## 6. Configuration and secrets

- Runtime config is read through the forge SDK `config` package (`config.GetConfigValue`), never hardcoded.
- Secrets load from **AWS Secrets Manager** via the SDK bootstrap, scoped per service by `AWS_SECRET_PREFIX`. See `~/.claude/standards/security.md` for handling rules.

## 7. Data and migrations

- **Database:** PostgreSQL. Conventions (UUID primary keys via `gen_random_uuid()`, `TIMESTAMPTZ` timestamps) are in `~/.claude/agents/software-engineer/db/sql.md`.
- **Migrations:** live under the service's `db/migrations/` as `<sequence>_<description>.up.sql` / `.down.sql`. The migration tool is per-service (e.g. `golang-migrate`, `goose`) — confirm via the service `go.mod` before generating.

## 8. Infrastructure as code

- **Terraform.** Reusable modules in `infra/modules/`, per-environment instantiation in `infra/environments/`, AWS provider.
- Module types: `service` (ECS Fargate / Lambda), `queue` (SQS), `storage` (S3 / DynamoDB), `network` (VPC / subnets), `shared` (reusable).

## 9. Local development

- Each service ships a `docker-compose.yaml` joined to the external `komodo-network`, exposing a `/health` endpoint for the container healthcheck.
- Ports are allocated in domain-grouped blocks of 10 — anchor is `public`, anchor+1 is `private`, remaining slots are reserved. 7000 is reserved.

| Port | Service | Domain |
|------|---------|--------|
| 7001 | ui (storefront) | Frontend |
| 7002 | events | Platform |
| 7003 | ssr-engine-svelte | Platform |
| 7011/7012 | auth pub/priv | Platform |
| 7022 | features | Platform |
| 7023 | ai-guardrails | AI |
| 7031 | address | Fulfillment |
| 7041–7045 | shop-items / search / cart / inventory / promotions | Commerce |
| 7051/7052 | user pub/priv | Commerce |
| 7061–7064 | order pub/priv / reservations / shipping | Orders + Fulfillment |
| 7071 | payments | Payments |
| 7081 | communications | Customer |
| 7091 | loyalty | Customer |
| 7101 | support | Customer |
| 7111/7112, 7114 | statistics pub/priv, insights | Platform |

## 10. Observability

Structured logging through the SDK `logging/runtime` package per `~/.claude/standards/logging.md`; metrics, traces, and `trace_id` propagation per `~/.claude/agents/devops/observability/standard.md`.
