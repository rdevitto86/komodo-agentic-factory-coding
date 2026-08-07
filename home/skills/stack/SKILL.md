---
name: stack
description: Komodo stack facts: repo layout, service naming, SDK imports, service anatomy, ports, deployment.
user-invocable: false
---

# Stack

Stack invariants so they never need rediscovering. Not engineering rules — those live in `coding-principles`. A project's own `AGENTS.md` wins over this file.

## Repository shape

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
| `ai/` | agentic-config, ai-guardrails | Agent config and AI services |
| `agtech/` | agtech-ahr | Agriculture vertical |
| `inventory/` | inventory-management-engine | Warehouse/inventory |

- **Service naming.** A backend service is `komodo-<name>-api` (e.g. `komodo-cart-api`). The short `<name>` is used for ports, secret prefixes, and migration paths.

## Languages and runtimes

- **Backend:** Go 1.26. Primary language for all services.
- **Frontend:** TypeScript + SvelteKit 5 (runes). See the `svelte` and `typescript` skills. Some projects use Vue 3 + Composition API instead — see the `vue` skill. The project's own `CLAUDE.md` is authoritative on which frontend framework is in use, per the "project file wins" note above.
- **Python:** tooling and data/ML work only — not a service language. See the `python` skill.

## Forge SDKs — reuse before building

`komodo-forge-sdk-go` / `komodo-forge-sdk-ts` are the first stop for any non-trivial logic (see the `coding-principles` skill for the full reuse order). The Go SDK provides HTTP server (`http/server`), middleware (`http/middleware`), error responses (`http/errors`), config (`config`), structured logging (`logging/runtime`), and AWS Secrets Manager bootstrap (`aws/secrets-manager`). Do not reimplement what the SDK covers; flag gaps for upstream extraction.

The SDKs are published and consumed as external dependencies — **always import the published packages, never a local-machine copy.** Go import path is `github.com/rdevitto86/komodo-forge-sdk-go` (e.g. `github.com/rdevitto86/komodo-forge-sdk-go/http/server`); the TS package is `@komodo-forge-sdk/typescript`. Do not add a local `replace` directive or a relative/sibling import to point at a checkout on disk unless the user explicitly asks for it (e.g. while developing the SDK itself). The same applies to *reading* SDK source: when an agent needs to confirm a signature, pull it from Git at the pinned version (raw GitHub or `gh`), never from an assumed local path such as `apis/komodo-forge-sdk-go/`.

## Service anatomy

A standard Go service is laid out as:

```
komodo-<name>-api/
├── cmd/server/main.go
├── internal/
│   └── clients/<svc>client/
├── docs/
│   ├── README.md
│   ├── architecture.md
│   ├── design-decisions.md
│   └── data-model.md
├── test/
├── deploy/
├── openapi.yaml
└── docker-compose.yaml
```

**Standard folders** — `cmd/`, `internal/`, `docs/`, `test/`, `deploy/`. A service adds `db/migrations/` only when it owns a database.

**Entrypoint convention** — one binary per service, `cmd/server/main.go` by default. `cmd/main.go` is the accepted alternative. No flat `main.go` at the service root.

Audience is a routing concern inside `internal/`, not a second binary: browser-facing routes take RequestID, Telemetry, RateLimiter, CORS, SecurityHeaders, Auth, CSRF, Normalization, Sanitization; service-to-service routes take RequestID, Telemetry, Auth, Scope. Rust services use `src/main.rs`. Lambda: one binary per function under `cmd/<function>/`.

**OpenAPI and codegen** — `openapi.yaml` at the service root is the contract source of truth. Each consumer generates downstream clients into its own `internal/clients/<service>client/` via `oapi-codegen`, sourcing the provider's `openapi.yaml` by relative path. No shared client package. Error code ranges: `komodo-forge-sdk-go/http/errors/ranges.go`; read `codes.go` before defining new codes.

API conventions (versioning, status codes, error format) are governed by the `api-design` skill.

## Compute and deployment

- **Cloud:** AWS.
- **Default compute:** ECS Fargate. Lambda is opt-in; `srv.Run` auto-detects the Lambda runtime via `AWS_LAMBDA_FUNCTION_NAME`, so the same service binary targets either without code changes.
- **Containers:** multi-stage, non-root images per multi-stage non-root images.

## Configuration and secrets

- Runtime config is read through the forge SDK `config` package (`config.GetConfigValue`), never hardcoded.
- Secrets load from **AWS Secrets Manager** via the SDK bootstrap, scoped per service by `AWS_SECRET_PREFIX`. See the `security` skill for handling rules.

## Data and migrations

- **Database:** PostgreSQL. Conventions (UUID primary keys via `gen_random_uuid()`, `TIMESTAMPTZ` timestamps) are in the `sql` skill.
- **Migrations:** live under the service's `db/migrations/` as `<sequence>_<description>.up.sql` / `.down.sql`. The migration tool is per-service (e.g. `golang-migrate`, `goose`) — confirm via the service `go.mod` before generating.

## Infrastructure as code

- **Terraform.** Reusable modules in `infra/modules/`, per-environment instantiation in `infra/environments/`, AWS provider.
- Module types: `service` (ECS Fargate / Lambda), `queue` (SQS), `storage` (S3 / DynamoDB), `network` (VPC / subnets), `shared` (reusable).

## Local development

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

## Observability

Structured logging through the SDK `logging/runtime` package per the `logging` skill; metrics, traces, and `trace_id` propagation per the `logging` skill.
