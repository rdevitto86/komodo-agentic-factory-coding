---
name: tech-stack
description: Komodo stack facts: repo layout, service naming, SDK imports, service anatomy, ports, deployment.
user-invocable: false
---

# Tech stack

Stack conventions so they never need rediscovering. Not engineering rules — those live in `coding-principles`. A project's own `AGENTS.md` wins over this file.

**This file records rules, never inventory.** No repo lists, no port assignments, no package manifests. Anything that changes when a repo is added or renamed belongs on disk, not here — `ls ~/komodo/` is always more current than a table.

## Repository shape

- **Multi-repo.** Repos live under `~/komodo/<domain>/`, grouped by business domain. Each service, SDK, UI, and tool is its own git repository. Domains are business-shaped, not layer-shaped — a domain directory names a part of the business, never a tier.
- **Service naming.** A backend service is `komodo-<name>-api`. The short `<name>` is the key used for its ports, secret prefix, and migration path — those three always agree.
- **Non-service repos** drop the `-api` suffix and name their kind: `-ui`, `-cli`, `-sdk-<lang>`, `-engine`, `-dashboard`.
- **Discover, don't recall.** To find a repo, list the domain directory. To confirm what a repo is, read its `AGENTS.md`.

## Languages and runtimes

- **Backend:** Go. Primary language for all services.
- **Frontend:** TypeScript, two frameworks chosen by workload — **Svelte** for lightweight, performance-minded, interactive pages; **Vue** for dashboards and feature-rich sites. A server-rendering engine exists for each.
- **Python:** tooling and data/ML work only — never a service language.

**Versions live in the language skills and in each repo's manifest**, never here. Load `go`, `typescript`, `svelte`, `vue`, `python`, or `uiux` for the current baseline; read `go.mod` / `package.json` / `pyproject.toml` for what a given repo actually pins.

**Framework choice is per-repo.** The project's own `CLAUDE.md` is authoritative, per the "project file wins" note above.

## Forge SDKs — reuse before building

**A shared SDK exists per language and is the first stop for any non-trivial logic** — see `coding-principles` for the full reuse order. It covers the cross-cutting service concerns: HTTP serving, middleware, error responses, config resolution, structured logging, and secret loading.

**Never conclude the SDK lacks something from memory.** Read its package tree and its exported surface at the version the repo pins. A gap confirmed that way gets flagged for upstream extraction; a gap assumed becomes a duplicate implementation.

**Always import the published package, never a local checkout.** No `replace` directive, no relative or sibling import pointing at a working copy on disk, unless the user is developing the SDK itself and says so. This applies to *reading* SDK source too — pull it from the registry or from Git at the pinned version, never from an assumed path on disk.

**The exact import paths belong to the language skills** — `go` and `typescript` name them.

## Service anatomy

**Standard folders** — `cmd/`, `internal/`, `docs/`, `test/`, `deploy/`, plus the service's contract file and its container definitions at the root. A service adds a migrations directory only when it owns a database. The matching language skill carries the concrete tree, its `AGENTS.md` field set, and the `test/` tier scheme its testing reference defines.

**Entrypoint convention** — one binary per service, under `cmd/`. No flat entrypoint at the service root, and never a public/private binary split.

**Audience is a routing concern, not a second binary.** Browser-facing routes carry the full perimeter chain — request ID, telemetry, rate limiting, CORS, security headers, auth, CSRF, normalisation, sanitisation. Service-to-service routes carry request ID, telemetry, auth, and scope only. Both live inside the same binary.

**OpenAPI is the contract source of truth**, one spec file at the service root. Each consumer generates its own client into its own internal package from the provider's spec. Never a shared client package, never a hand-written one.

**Error codes come from a reserved range per service.** The SDK owns the range allocation and the existing codes — read them there before defining a new one, rather than picking a number.

## API wire contract

Generic REST practice is assumed and not restated here. These are the four shapes that must be **byte-identical across every service**, because a consumer written against one service parses all of them.

**Error envelope:**

```json
{ "error": { "code": "MACHINE_READABLE_CODE", "message": "safe for display", "details": {} } }
```

`code` is SCREAMING_SNAKE and stable across versions. `message` carries no internal detail. `details` is optional, for field-level validation. Never expose stack traces, internal service names, or query strings.

**Paginated list:**

```json
{ "data": [], "pagination": { "cursor": "...", "hasMore": true } }
```

**Payload conventions** — JSON only, `camelCase` fields, ISO 8601 UTC timestamps, IDs as strings.

**Two status codes carry a house meaning** beyond the RFC: `422` is a validation failure on a well-formed request; `409` is a state conflict or optimistic-lock failure.

**Versioning** — version in the path (`/v1/users`). A new version is required for removing or retyping a field, removing an endpoint, or changing auth; additive changes are not breaking. Deprecated versions send `Deprecation` and `Sunset` headers with at least 90 days notice.

## Compute and deployment

- **Cloud:** AWS.
- **Default compute:** ECS Fargate. Lambda is opt-in; `srv.Run` auto-detects the Lambda runtime via `AWS_LAMBDA_FUNCTION_NAME`, so the same service binary targets either without code changes.
- **Containers:** multi-stage, non-root images per multi-stage non-root images.

## Configuration and secrets

- Runtime config is read through the forge SDK `config` package (`config.GetConfigValue`), never hardcoded.
- Secrets load from **AWS Secrets Manager** via the SDK bootstrap, scoped per service by `AWS_SECRET_PREFIX`. See the `security` skill for handling rules.

## Data and migrations

- **A service owns its data.** No shared database between services, no cross-service reads at the storage layer — that is what the API contract is for.
- **Relational is the default;** a document or key-value store is chosen for a specific access pattern, not as a default. Conventions for every engine are in the `database` skill.
- **Migrations live in the service repo** under its own migrations directory, one sequence per service. The migration tool is per-service — confirm from the repo's manifest before generating anything.

## Infrastructure as code

- **AWS CDK, TypeScript.** Shared infrastructure lives in its own platform repo. See the `cdk` skill for stack layout and safety rules.
- Construct types: `service` (ECS Fargate / Lambda), `queue` (SQS), `storage` (S3 / DynamoDB), `network` (VPC / subnets), `shared` (reusable).

## Ports — the dual-port strategy

**A service that serves two audiences binds two ports, never one port with a path prefix.** The separation is what lets the perimeter and the mesh carry different middleware, different auth, and different exposure — a prefix on a shared listener collapses all three.

| Port | Audience | Reached by |
|---|---|---|
| Anchor | Public | Browser, external client, through the edge |
| Anchor + 1 | Private | Sibling services, inside the mesh |

**Allocation is by rule, not by registry.** Ports come in domain-grouped blocks of 10: the anchor is public, anchor+1 is private, the remaining 8 slots are reserved for that domain's growth. 7000 is reserved and never assigned.

**Never look up a port from this file.** The authoritative value for any service is its own `docker-compose.yaml` and its repo `AGENTS.md` Quick-reference row. Read those.

**Single-audience services bind the anchor only.** The private slot stays reserved rather than being reused by a neighbour.

## Headless containers vs serverless functions

Both target the same service binary — `srv.Run` detects the Lambda runtime via `AWS_LAMBDA_FUNCTION_NAME` — but the operational shape differs, and the difference drives layout and testing.

| Aspect | Fargate container | Lambda function |
|---|---|---|
| Entrypoint | One binary, `cmd/server/` | One binary per function, `cmd/<function>/` |
| Ports | Binds anchor (+ private) | None — event or function URL |
| Health | `/health` for the container check | No check; the platform owns liveness |
| Lifetime | Long-lived, warm state | Per-invocation, cold-start sensitive |

**Fargate is the default.** Lambda is opt-in, chosen for spiky or event-driven work where paying for an idle container is the larger cost.

**Anything initialised per-request on Lambda is a cold-start bill.** Clients, pools, and config load once outside the handler. The same code on Fargate hides the mistake because the container amortises it.

**Local development runs the container shape only.** Each service ships a `docker-compose.yaml` joined to an external shared network, exposing `/health` for the healthcheck. A Lambda-targeted service still runs as a container locally.

## Observability

**Logs, metrics, and traces all go through the shared SDK**, never a hand-rolled logger or a direct vendor client — that is what keeps the trace ID propagating across service boundaries. Standards for all three signals are in the `observability` skill; the language skill names the SDK package.
