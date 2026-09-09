---
name: standards-go
description: Go standards — idioms, domain modelling, concurrency, errors, performance. Load before reading or writing any .go, go.mod, or go.sum file.
user-invocable: false
paths: "**/*.go, **/go.mod, **/go.sum"
---

# Go

No comment or godoc authored directly in a session — see Comment discipline below. Error strings lead with a verb phrase and never name the function.

## Comment discipline

The real rules live in `claude-code/hooks/comments.py` and its shared rules module `claude-code/hooks/lib/comment_rules.py`, not here. `comments.py check` reports what is missing or malformed; comments are only ever added via the `write-comments` skill.

## Toolchain

- **Version floor is whatever `go.mod` declares.** Read it; never assume a release.
- **`Makefile`'s `verify` target is the merge gate** — `gofmt -l`, `go vet`, `golangci-lint run`, `go test -race -cover`, `go build`, in that order. `context_injector.py` reads this target directly; a repo without it has no gate.
- **Formatting and linting** — `gofmt` and `goimports` on commit, `golangci-lint` as the gate. These are the tools the pre-commit hook runs; `standards-cicd` defines when.
- **Vulnerability scanning** — `govulncheck ./...` is the gate; it reports reachability, so triage by call path, not by CVE score alone. Enable `gosec` in `.golangci.yaml` for the static half. `standards-cicd` defines the gate; the `standards-api-security` skill states the bar.
- **Outdated dependencies** — `go list -u -m all` flags modules that are outdated, no CVE required to surface. Advisory only, never a merge gate; bump with `go get -u <module>` then `go mod tidy`, one module at a time rather than a blanket `go get -u ./...`.
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
- **Wrap a long statement by collapsing one level at a time, never straight to one-arg-per-line.** Soft target ~120 cols. Try the whole statement on one line first; if it doesn't fit, move the wrapped content to its own indented line(s) with a trailing comma and the closing paren/brace on its own line at the original indent — a function call may stay grouped at this step if it fits, but struct literal fields never do, always one field per line once wrapped. Only if that grouped form is still too long, break to one item per line. Apply identically to calls, struct literals, multi-return signatures (wrap the return tuple, not just the params), and multi-arg logger calls (`logger.Info` with several `logger.Attr(...)` args).
- **A numeric literal in a function body standing for a size, TTL, count, or threshold gets extracted to a named const.** Package-level when shared by more than one function in the package; a function-local const block at the top of the function when scoped to one constructor. Never introduce a new `SCREAMING_SNAKE_CASE` const, exported or not — exported consts use MixedCaps, unexported use lowerCamelCase; an existing `SCREAMING_SNAKE_CASE` const already in the package is grandfathered legacy and a rename-sweep candidate, never a pattern to continue even in the same file. An explicitly-requested repo-wide casing sweep renames every exported `SCREAMING_SNAKE_CASE` identifier the repo itself owns — definition and every call site, tests included — never one owned by an external/SDK package just because a local file references it. Place the new const immediately above or inside the function that first needs it, not in a distant top-of-file block unless one already exists for that category. A literal repeated across 2+ files or functions is the strongest signal to extract first. If the value already exists as a generated model/enum constant, use that instead of adding a parallel one. A magnitude that isn't obvious from the const's own name (`maxUploadBytes = 1 << 20`) is a `write-comments` `NOTE:` candidate, never an inline comment written directly — `comments.py` has no exception for this, by design.
- **No blank line between two consecutive early-return guard clauses.** Exactly one blank line between the last guard clause in a sequence and the happy-path logic that follows it, and one after a multi-line composite-literal construction or call, before the next statement.

**A single-call-site function must earn its place** as one of: dependent setup/wiring (`NewX`, option functions), a cohesive subset of functionality, a concurrency unit (goroutine or worker-loop body), or a deliberate abstraction seam (an interface swapped for a fake). Sequencing a handful of statements is not a subset of functionality — it is the call site. A long wiring function is normal Go, not a smell. The exception runs the other way: inline a single-call-site helper into a closure at its call site when it is a pure, incidental value computation with no domain/business significance (a random-string generator, an error-to-display mapper) — never one kept to stay under a complexity-linter budget, one with security significance, or a named pipeline/validation step, all of which stay named functions even at one call site. If the helper carried its own unit test, confirm those cases are still reachable through the caller's tests before deleting it.

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

## Security standards

Language-specific insecure-usage patterns for `/assess-security` to pull from, beyond `standards-api-security`'s generic OWASP checklist.

- **`html/template` for anything rendered to a browser, never `text/template`.** `text/template` performs no contextual escaping — interpolating request-derived data into it is stored/reflected XSS.
- **`os/exec` with an argument slice, never a shell string.** `exec.Command("sh", "-c", userInput)` is command injection; build `exec.Command(bin, arg1, arg2)` instead.
- **`database/sql` placeholders (`?`/`$1`), never string-built queries.** `fmt.Sprintf` into a query string is SQL injection regardless of how the value was validated upstream.
- **`encoding/gob` and `encoding/json` into `interface{}`/`any` from an untrusted source is an insecure-deserialization surface** — decode into a concrete, field-limited struct instead.
- **`filepath.Clean` plus a prefix check after `filepath.Join`** on any user-supplied path segment — `Join` alone does not stop `../` traversal out of the intended root.
- **`crypto/rand`, never `math/rand`, for a token, key, or nonce.** `math/rand` is seeded and predictable.
- **`InsecureSkipVerify: true` on a `tls.Config` disables certificate validation** — a debug-only flag that must never reach a committed default.

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
