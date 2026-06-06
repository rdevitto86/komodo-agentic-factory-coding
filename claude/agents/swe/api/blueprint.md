# API Blueprint

The canonical shape of a Komodo API. This file is the single source of truth for **what artifacts an API must have and which standard governs each one** — language-agnostic. It does not restate rules; it points at the standard that owns them.

Two consumers read this file:

- **Generators** (`/new-service` and future per-language scaffolds) build a new API to this shape.
- **The auditor** (`/api-audit`) checks an existing API against this shape and reports gaps.

Keeping both on one checklist is the point: a service is conformant when it satisfies every applicable row below, and "conformant" means the same thing whether you are generating or auditing.

---

## 1. How language specifics are handled

This blueprint stays generic. Anything language-specific — file extensions, idiomatic layout, test framework, dependency manifest — is **deferred to the language standard and its scaffold skill**, never inlined here. When a row says "defer to language," read the mapped standard for the concrete form.

| Language | Standard | Scaffold skill | Test standard |
|----------|----------|----------------|---------------|
| Go | `~/.claude/agents/swe/go/coding.md` | `/new-service` | `~/.claude/agents/swe/go/coding.md` |
| TypeScript / Node | `~/.claude/agents/swe/ts/coding.md` | _(none yet — audit-only)_ | `~/.claude/agents/swe/ts/coding.md` |
| Python (tooling/data only — not a service language) | `~/.claude/agents/swe/python/coding.md` | _(none — see `~/.claude/agents/swe/stack.md` §2)_ | `~/.claude/agents/swe/python/coding.md` |

Go is the primary service language (`~/.claude/agents/swe/stack.md` §2); its concrete anatomy is fixed in `~/.claude/agents/swe/stack.md` §4. TypeScript and Python APIs have no generator yet — the auditor still applies every language-agnostic row to them, and defers structural rows to the language standard.

---

## 2. Conformance checklist

Each row is a required property of a conformant API. **Governed by** names the standard that defines pass/fail — this file never duplicates that rule. **Applies to** scopes the row; rows marked _all_ apply to every language.

### Structure & surface

| # | Property | Governed by | Applies to |
|---|----------|-------------|------------|
| S1 | Service is named and laid out per the language's standard anatomy | `~/.claude/agents/swe/stack.md` §4 (Go); language standard otherwise | all |
| S2 | A versioned public surface exists (e.g. `pkg/v1/` in Go), separated from internal code | `~/.claude/agents/swe/stack.md` §4 | all |
| S3 | URL paths carry the version (`/v1/...`) and follow REST naming | `~/.claude/agents/swe/api/design.md` §1, §6 | all |
| S4 | Internal-only code is not importable as public API | language standard | all |

### Runtime contract

| # | Property | Governed by | Applies to |
|---|----------|-------------|------------|
| R1 | A `/health` endpoint exists and is wired into the container healthcheck | `~/.claude/agents/swe/stack.md` §4, §9 | all |
| R2 | HTTP methods and status codes match the contract (no GET side effects, no 200-on-error) | `~/.claude/agents/swe/api/design.md` §2, §3 | all |
| R3 | Error responses use the standard `{ error: { code, message, details } }` envelope | `~/.claude/agents/swe/api/design.md` §5 | all |
| R4 | Error and log message strings start with a verb phrase — no name/noun prefix | `~/.claude/agents/swe/principles.md` §1 | all |
| R5 | Request bodies/params are validated; no raw client input reaches the domain layer | `~/.claude/agents/swe/security.md` | all |
| R6 | Auth, CSRF, and idempotency applied per route protection level | `~/.claude/agents/swe/security.md`, `~/.claude/agents/swe/api/design.md` §2 | all |

### Cross-cutting

| # | Property | Governed by | Applies to |
|---|----------|-------------|------------|
| X1 | Dependencies are injected via constructors/options — no package-level singletons or `init()` side effects | `~/.claude/agents/swe/principles.md` §3 | all |
| X2 | Non-trivial logic reuses the forge SDK before a library before custom code | `~/.claude/agents/swe/principles.md` §2, `~/.claude/agents/swe/stack.md` §3 | all |
| X3 | Config read through the SDK `config` surface; secrets via Secrets Manager bootstrap | `~/.claude/agents/swe/stack.md` §6, `~/.claude/agents/swe/security.md` | all |
| X4 | Structured logging through the SDK; `trace_id` propagated | `~/.claude/agents/swe/logging.md`, `~/.claude/agents/devops/observability/standard.md` | all |
| X5 | Comments conform — no doc blocks, name-leading comments, or verbose blocks | `~/.claude/agents/swe/comments.md` | all |

### Documentation & tests

| # | Property | Governed by | Applies to |
|---|----------|-------------|------------|
| D1 | `docs/` skeleton present: `README.md`, `openapi.yaml`, `architecture.md`, `design-decisions.md`, `data-model.md` | `~/.claude/agents/swe/stack.md` §4 | all |
| D2 | Every endpoint has an OpenAPI 3.x entry with parameters, body, responses, and all error codes | `~/.claude/agents/swe/api/design.md` §7 | all |
| D3 | Tests are colocated with the code they cover and follow the language's test naming | `~/.claude/agents/swe/go/coding.md` / `~/.claude/agents/swe/ts/coding.md` | all |
| D4 | Each handler has a test covering success, validation error, not-found, and auth-failure paths | `~/.claude/agents/swe/go/coding.md` / `~/.claude/agents/swe/ts/coding.md` | all |

### Packaging

| # | Property | Governed by | Applies to |
|---|----------|-------------|------------|
| P1 | Multi-stage, non-root container image | `~/.claude/agents/devops/docker/standard.md` | all |
| P2 | `docker-compose.yaml` joins `komodo-network` and exposes the healthcheck | `~/.claude/agents/swe/stack.md` §9 | all |
| P3 | Schema changes ship as paired `.up.sql` / `.down.sql` migrations | `~/.claude/agents/swe/db/sql.md`, `~/.claude/agents/swe/stack.md` §7 | services with a database |

---

## 3. TODO-pointer convention

A generated API ships as a skeleton: structure and wiring in place, behavior stubbed. Every stub left for the developer is marked inline so the gap lives next to the code, never only in a separate doc. Use the language's native comment form (per `~/.claude/agents/swe/comments.md`) with a `TODO:` lead and an imperative pointer:

- A stubbed handler: `TODO: extract and validate params, call the service layer` next to a not-implemented error response.
- An empty model file: `TODO: define domain models for <name>`.
- A placeholder OpenAPI operation: `summary: TODO` plus `# TODO: add parameters, requestBody, responses`.

Each pointer states the **next action**, not a description of the absence. `TODO: validate the order ID` — not `TODO: this is empty`. Larger deferred decisions that don't belong inline go to the nearest `TODO.md` per `~/.claude/agents/project-manager/todo.md`.

---

## 4. Audit procedure

`/api-audit` walks §2 row by row against a target API and reports conformance — report-only, it does not modify the API. For each row it emits **PASS / FAIL / N/A** with a `file:line` pointer and the governing standard. Non-conformances are persisted to the project's `TODO.md` under an audit section per `~/.claude/agents/project-manager/todo.md`. The skill defines the report format and invocation.
