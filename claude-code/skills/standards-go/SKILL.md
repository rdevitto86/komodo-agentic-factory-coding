---
name: standards-go
description: Go standards — idioms, domain modelling, concurrency, errors, performance. Load before reading or writing any .go, go.mod, or go.sum file.
user-invocable: false
paths: "**/*.go, **/go.mod, **/go.sum"
---

# Go

Zero comments, zero godoc. Error strings lead with a verb phrase and never name the function.

## Comment discipline

`rules-commenting` carries the shared template contract. This language's exempt machine directives, verified against the guard's own list: `go:build`, `go:generate` (matches the `go:` prefix), `cgo` pragmas, `//nolint:<rule>`. Anything else — including godoc on an exported symbol — prompts for approval.

## Toolchain

- **Version floor is whatever `go.mod` declares.** Read it; never assume a release.
- **`Makefile`'s `verify` target is the merge gate** — `gofmt -l`, `go vet`, `golangci-lint run`, `go test -race -cover`, `go build`, in that order. `context_injector.py` reads this target directly; a repo without it has no gate.
- **Formatting and linting** — `gofmt` and `goimports` on commit, `golangci-lint` as the gate. These are the tools the pre-commit hook runs; `standards-cicd` defines when.
- **Vulnerability scanning** — `govulncheck ./...` is the gate; it reports reachability, so triage by call path, not by CVE score alone. Enable `gosec` in `.golangci.yaml` for the static half. `standards-cicd` defines the gate; the `standards-api-security` skill states the bar.
- **Coverage delta is per package** — Go reports at package granularity, so the pre-push scope is the set of packages containing changed files.
- **Forge SDK** — module path `github.com/rdevitto86/komodo-forge-sdk-go`, subpackaged by concern. Import the published module at a pinned version; never a `replace` directive pointing at a local checkout. Read its package tree before concluding it lacks something.

## Conventions

- **`golangci-lint` must pass.** Config at `.golangci.yaml`. A suppression is a bare `//nolint:<rule>` directive with no appended prose; the reason goes in `BACKLOG.md`, never in a comment. Prefer fixing the finding.
- **Naming**: `req` for request bodies and outgoing `*http.Request`, `res` for response objects. Keep `r *http.Request` / `w http.ResponseWriter`.
- **No `util` / `common` / `helpers` packages.** Split by domain. `internal/` for non-importable code. Minimise exported surface.
- **`init()` does no I/O.** Wire through constructors (`NewServer`, `NewService`) — never package-level globals or `init()` registration.
- **Log once, at the top of the stack.** Never log-and-return.
- **Avoid vague names** — `processData`, `handleStuff`, `doWork`. A name you cannot make specific usually marks a function that should not exist.
- **Bare `{ }` blocks narrow scope, they don't replace extraction.** Use one to isolate a short-lived variable or a lock/defer pair from the rest of the function. Past ~5 lines, it's a function, not a block.
- **Prefer a closure over a new type or extra parameter** when it captures state a caller already has in scope — `sort.Slice`'s comparator, a middleware wrapping `http.Handler`, an `errgroup` task body. Skip it on a hot path: each closure is a heap allocation once it escapes, and a captured loop variable reused across iterations is a classic bug. Measure before choosing a closure over a struct method in code `pprof` already flags.

**A single-call-site function must earn its place** as one of: dependent setup/wiring (`NewX`, option functions), a cohesive subset of functionality, a concurrency unit (goroutine or worker-loop body), or a deliberate abstraction seam (an interface swapped for a fake). Sequencing a handful of statements is not a subset of functionality — it is the call site. A long wiring function is normal Go, not a smell.

## Types and boundaries

- **Named types carry domain meaning** — `type UserID string`, `type TimeoutMS int`. Catches argument-order bugs at compile time.
- **Parse at the edge.** Decode into a request DTO, convert once into the domain type, and let the service layer accept only the domain type. A method taking `map[string]any` or a raw `*http.Request` has no boundary.
- **Fallible constructors return `(T, error)`** — never a zero value plus a `Valid()` the caller may forget.
- **Distinct types over bool flags** and optional fields only meaningful in combination. Two mutually exclusive states are two types or a sealed constant set.
- **Zero values are usable or impossible.** Either the zero value works, or the type is unexported behind a constructor. One that compiles and panics is the worst of both.
- **Explicit JSON struct tags** on every serialised field. `DisallowUnknownFields` only at untrusted external edges.
- **Interfaces stay small and live at the consumer.** One or two methods, declared in the calling package. Never publish an interface beside its only implementation.

## Concurrency, state, resources

- **`ctx context.Context` is the first parameter** of anything doing I/O or blocking work. Never in a struct field, never `nil`.
- **Every blocking call gets a deadline.** `defer cancel()` goes on the line after `WithTimeout`/`WithCancel`. A context without a deadline crossing a network boundary is a defect.
- **`defer` releases in the same scope that acquires** — `rows.Close()`, `resp.Body.Close()`, `mu.Unlock()`.
- **Bound every channel and worker pool.** Explicit capacity, worker counts from config. Define behaviour at capacity — block, drop, or error.
- **`errgroup.WithContext` for fan-out** so the first failure cancels its siblings. A bare `go func()` with no lifecycle owner is a leak.
- **Do not mutate caller-owned slices or maps.** `append` aliasing on a shared backing array corrupts silently.
- **A mutex or `sync/atomic` — never "it is just one field."** Run `go test -race`; a race is a bug even when tests pass.
- **Guard the mutex, not the method.** Never make a network or disk call while holding a lock.

## Errors and resilience

- **Wrap with `%w`, match with `errors.Is` / `errors.As`.** Never string-compare error text. Sentinels for expected conditions, typed errors when the caller needs structure.
- **Never discard an error with `_`** outside a `defer` where the failure is genuinely unactionable.
- **`panic` is for programmer error only.** Recover only at the top-level server or goroutine boundary, and log the stack there.
- **Retries carry exponential backoff, jitter, and a cap.** Only idempotent operations, only genuinely transient errors — never a 4xx or a validation failure.
- **Idempotency keys on state-changing handlers**, so a retry converges instead of double-writing.
- **Graceful shutdown**: catch `SIGTERM`/`SIGINT`, `server.Shutdown(ctx)` with a bounded drain, close pools in reverse dependency order.
- **`crypto/subtle.ConstantTimeCompare`** for tokens, signatures, and secrets — never `==` or `bytes.Equal`.

## Performance

Measure first. An optimisation without a before/after number is unreviewable.

- **Profile with `pprof`** before optimising; benchmark hot paths with `testing.B` and compare via `benchstat`.
- **Pre-size** with `make([]T, 0, n)` and `make(map[K]V, n)` whenever the count is known — the largest low-effort win.
- **`sync.Pool` only on measured hot paths.** It costs clarity everywhere else.
- **`strings.Builder`** for repeated concatenation in a loop. Large structs by pointer, small ones by value.
- **Batch queries.** `EXPLAIN ANALYZE` before adding an index. N+1 is a bug, not a tuning opportunity.

## Evolution

Exported API and schema changes are additive. New optional fields and new functions are safe; renaming, removing, or retyping an exported symbol breaks every service of yours that imports it and needs a version bump. Add to a struct rather than changing a signature; use functional options so a constructor can grow.

## Quick-reference fields

The field set a Go repo's `AGENTS.md` Quick-reference table carries. Every value is read from the repo, never assumed.

| Field | Source on disk |
|---|---|
| Language + floor | `go.mod` |
| Module path | `go.mod` |
| Port(s) | `docker-compose.yaml`, config |
| Contract | `openapi.yaml` (`go-api`) or `tools.md` (`go-mcp`), whichever is present |
| Entrypoint | `cmd/server/main.go` |

Drop a row whose value the repo genuinely lacks. Never add a row for a fact this skill or `standards-sdlc` already states by name.

## Repo layout — `go-api`

```
cmd/server/main.go
internal/
docs/
test/
deploy/
openapi.yaml
docker-compose.yaml
Dockerfile
.dockerignore
.gitignore
Makefile
go.mod
```

One generic entrypoint at `cmd/server/main.go`, or `cmd/main.go` as the alternative — never a `cmd/public` / `cmd/private` split, which is a dead convention. Audience-specific middleware (browser-facing vs service-to-service) is a routing concern inside `internal/`, not a second binary.

**No `db/` unless the user says the service owns one.** Most do not. When it does, add `db/migrations/` and follow the `standards-database` skill's migration naming.

## Seed backlog — `go-api`

Stories `repo-init` splices into `Cross-Cutting` on Create, or appends if missing on Scaffold/Refresh.

- [M] Flesh out `openapi.yaml` beyond the `/health` stub as routes land · S
- [M] Tests: unit + component coverage · S → `make test`

## Repo layout — `go-mcp`

```
cmd/server/main.go
internal/
docs/
test/
deploy/
tools.md
docker-compose.yaml
Dockerfile
.dockerignore
.gitignore
Makefile
go.mod
```

Same shape as `go-api` — one entrypoint at `cmd/server/main.go`, no audience-split binaries — with `tools.md` replacing `openapi.yaml` as the contract file: a running list of registered MCP tools (name, input schema, output shape), not a REST route table. `internal/` holds tool implementations; a tool is a routing concern the same way a handler is in `go-api`, never a second binary.

**No established Go MCP SDK is recorded here yet.** `repo-init`'s Step 5 stops and asks rather than guessing an import path — see that skill for the rule. Once one is confirmed for a real build, it belongs in `reference.md`'s wiring section, not re-decided per repo.

## Seed backlog — `go-mcp`

Stories `repo-init` splices into `Cross-Cutting` on Create, or appends if missing on Scaffold/Refresh.

- [M] Flesh out `tools.md` and the tool registry beyond the `/health` stub as tools land · S
- [M] Tests: unit + component coverage · S → `make test`

## Reference material

- **[reference.md](reference.md)** — a complete comment-free vertical slice: service aggregate, handler, two-tier repository, outbound client, wiring, component test.
- **[testing.md](testing.md)** — the approved test stack, the tier ladder, file placement, coverage floors. Gate/floor definitions there defer to the `standards-sdlc` skill.
