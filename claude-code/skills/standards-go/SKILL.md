---
name: standards-go
description: Go standards — idioms, domain modelling, concurrency, errors, performance. Load before reading or writing any .go, go.mod, or go.sum file.
user-invocable: false
paths: "**/*.go, **/go.mod, **/go.sum"
---

# Go

Comments go through `comments.py apply`, never `Edit`/`Write` — see Comment discipline below. Error strings lead with a verb phrase and never name the function.

## Comment discipline

`standards-comments` owns the rules and loads on every `.go` file alongside this skill — read it there, not here. In short: every function gets one line saying what it does, exported or not; everything else gets nothing unless the code cannot say it; `comments.py apply` is the only write path. A godoc-shaped one-sentence comment opening with the declared name is correct on unexported declarations too.

## Toolchain

- **Version floor is whatever `go.mod` declares.** Read it; never assume a release.
- **`Makefile`'s `verify` target is the merge gate** — `gofmt -l`, `go vet`, `golangci-lint run`, `go test -race -cover`, `go build`, in that order. `context_injector.py` reads this target directly; a repo without it has no gate.
- **Formatting and linting** — `gofmt` and `goimports` on commit, `golangci-lint` as the gate. These are the tools the pre-commit hook runs; `standards-cicd` defines when.
- **Vulnerability scanning** — `govulncheck ./...` is the gate; it reports reachability, so triage by call path, not by CVE score alone. Enable `gosec` in `.golangci.yaml` for the static half. `standards-cicd` defines the gate; the `standards-api-security` skill states the bar.
- **Outdated dependencies** — `go list -u -m all` flags modules that are outdated, no CVE required to surface. Advisory only, never a merge gate; bump with `go get -u <module>` then `go mod tidy`, one module at a time rather than a blanket `go get -u ./...`.
- **Internal dependency graph** — `go list -deps ./...` for package edges, `go mod graph` for module ones. `assess-change-risk` states when to run it and what its output does to the tier.
- **Coverage delta is per package** — Go reports at package granularity, so the pre-push scope is the set of packages containing changed files.
- **Forge SDK** — module path `github.com/rdevitto86/komodo-forge-sdk-go`, subpackaged by concern. Import the published module at a pinned version; never a `replace` directive pointing at a local checkout. Read its package tree before concluding it lacks something.
- **A requested repo-wide sweep (magic numbers, casing, literal-flattening, or similar) must actually cover the whole repo** — `test/`, `cmd/`, and any `//go:build`-gated file, the latter re-run with its matching `-tags` so the gated code is compiled and checked, not skipped by default constraints. Confirm completeness with a fresh, non-cached run (`go build`, `go vet`, `golangci-lint run`, `go test -count=1 ./...`) — a cached `go test` can report stale results from before the sweep.

## Conventions

- **`golangci-lint` must pass.** Config at `.golangci.yaml`. A suppression is a bare `//nolint:<rule>` directive with no appended prose; the reason goes in `BACKLOG.md`, never in a comment. Prefer fixing the finding.
- **Naming**: `req` for request bodies and outgoing `*http.Request`, `res` for response objects. Keep `r *http.Request` / `w http.ResponseWriter`.
- **No `util` / `common` / `helpers` packages.** Split by domain. `internal/` for non-importable code. Minimise exported surface.
- **`init()` does no I/O.** Wire through constructors (`NewServer`, `NewService`) — never package-level globals or `init()` registration.
- **Log once, at the top of the stack.** Never log-and-return.
- **Avoid vague names** — `processData`, `handleStuff`, `doWork`. A name you cannot make specific usually marks a function that shouldn't exist.
- **Bare `{ }` blocks narrow scope, they don't replace extraction.** Use one to isolate a short-lived variable or a lock/defer pair from the rest of the function. Past ~5 lines, it's a function, not a block.
- **Prefer a closure over a new type or extra parameter** when it captures state a caller already has in scope — `sort.Slice`'s comparator, a middleware wrapping `http.Handler`, an `errgroup` task body. Skip it on a hot path: each closure is a heap allocation once it escapes, and a captured loop variable reused across iterations is a classic bug. Measure before choosing a closure over a struct method in code `pprof` already flags.
- **A `const` is zero-cost at any scope** — the compiler resolves it, so scope it as narrowly as the use allows. A `var` whose initializer runs code (`errors.New(...)`, `regexp.MustCompile(...)`, a struct literal holding a func field or needing setup) costs an allocation or computation *every time that declaration executes* — scope it to how often it should be constructed, not to how many call sites reference it: package-level runs once at init, function-local per call, loop-body per iteration.
- **A sentinel error, or any value compared by `==`/`errors.Is`/`errors.As`, stays a single, stable package-level `var`, never a function-local one, regardless of call-site count.** Moving it into a function body creates a new instance per call and silently breaks every identity comparison downstream. Deleting an exported sentinel to do this is also a breaking API change per Evolution below.
- **Wrap a long statement by collapsing one level at a time, never straight to one-arg-per-line.** 120 cols is a hard lint gate (`lll` in `.golangci.yaml`); a ~90-col soft threshold governs the separate grouped-vs-one-item-per-line judgment, which `lll` cannot make, so it stays a reviewed convention. Try the statement on one line first; if it doesn't fit, move the wrapped content to its own indented line(s), trailing comma, closing paren/brace on its own line at the original indent — a call may stay grouped here if it fits ~90 cols, but struct literal fields never do, always one per line once wrapped. Only if that grouped form is still too long (or over ~90 cols), break to one item per line. Apply identically to calls, struct literals, multi-return signatures (wrap the return tuple, not just the params), and multi-arg logger calls (`logger.Info` with several `logger.Attr(...)` args).
- **A numeric literal, a short repeated string, or a spec-level value (a header name, status string, config key) in a function body standing for a size, TTL, count, threshold, or fixed identifier gets extracted to a named const.** Package-level when more than one function in the package shares it; a function-local const block at the top of the function when scoped to one constructor. Put it immediately above or inside the function that first needs it, not a distant top-of-file block unless one already exists for that category. A literal repeated across 2+ files or functions is the strongest signal to extract first; if it already exists as a generated model/enum constant, use that. **A const used by more than one package lives in the single most spec-relevant domain package; every other package imports it — never a copy per consumer.** **Two or more consts introduced together go in one `const ( ... )` block**, never consecutive standalone statements — a block reads as one decision, separate statements as unrelated ones. Never introduce a new `SCREAMING_SNAKE_CASE` const: exported use MixedCaps, unexported lowerCamelCase. An existing one is grandfathered legacy and a rename-sweep candidate, never a pattern to continue even in the same file; an explicitly-requested repo-wide casing sweep renames every exported `SCREAMING_SNAKE_CASE` identifier the repo itself owns — definition and every call site, tests included — never one owned by an external/SDK package a local file merely references. A magnitude not obvious from the const's own name (`maxUploadBytes = 1 << 20`) is a `write-comments` `NOTE:` candidate, never an inline comment — `comments.py` has no exception, by design.
- **No blank line between two consecutive early-return guard clauses.** Exactly one blank line between the last guard clause in a sequence and the happy-path logic that follows it, and one after a multi-line composite-literal construction or call.
- **A composite literal skips an embedded field's own nested literal once `go.mod`'s floor reaches the release that added the rewrite (Go 1.27)** — write `T{x: 1}`, not `T{U: U{x: 1}}`, when `x` is promoted from embedded `U`. `gopls`'s `embedlit` analyzer performs the rewrite.

**A single-call-site function defaults to a full inline into the caller's body** when it is a pure, small, incidental value computation with no domain significance (a random-string generator, an error-to-display mapper). A closure still costs an allocation a fully-inlined statement doesn't, so reserve it for when a full inline would awkwardly restructure the caller — the value needs `:=` ahead of use, or inlining breaks up an otherwise-linear statement. It earns a named, un-inlined place instead as one of: dependent setup/wiring (`NewX`, option functions), a cohesive subset of functionality, a concurrency unit (goroutine or worker-loop body), or a deliberate abstraction seam (an interface swapped for a fake). Sequencing a handful of statements is not a subset of functionality — it is the call site; a long wiring function is normal Go, not a smell. **Security- and audit-significant logic always stays a named function regardless of size or call-site count** — a constant-time compare, an auth check, a revocation/ban predicate — greppable-during-review outweighs one fewer symbol; never inline one to stay under a complexity-linter budget. A named pipeline/validation step stays named for the same reason. If the helper carried its own unit test, confirm those cases are still reachable through the caller's tests before deleting it.

## Types and boundaries

- **Named types carry domain meaning** — `type UserID string`, `type TimeoutMS int`. Catches argument-order bugs at compile time.
- **Parse at the edge.** Decode into a request DTO, convert once into the domain type, and let the service layer accept only the domain type. A method taking `map[string]any` or a raw `*http.Request` has no boundary.
- **Fallible constructors return `(T, error)`** — never a zero value plus a `Valid()` the caller may forget.
- **Distinct types over bool flags** and optional fields only meaningful in combination. Two mutually exclusive states are two types or a sealed constant set.
- **Zero values are usable or impossible.** Either the zero value works, or the type is unexported behind a constructor. One that compiles and panics is the worst of both.
- **Explicit JSON struct tags** on every serialised field. `DisallowUnknownFields` only at untrusted external edges.
- **Interfaces stay small and live at the consumer.** One or two methods, declared in the calling package. Never publish an interface beside its only implementation. This is Dependency Inversion applied to Go: the caller depends on a small interface it declares and owns, not on a concrete type from the implementing package — the implementation depends on the abstraction's shape, never the reverse.

## Concurrency, state, resources

- **`ctx context.Context` is the first parameter** of anything doing I/O or blocking work. Never in a struct field, never `nil`.
- **Every blocking call gets a deadline.** `defer cancel()` goes on the line after `WithTimeout`/`WithCancel`. A context without a deadline crossing a network boundary is a defect.
- **`defer` releases in the same scope that acquires** — `rows.Close()`, `resp.Body.Close()`, `mu.Unlock()`.
- **Bound every channel and worker pool.** Explicit capacity, worker counts from config. Define behaviour at capacity — block, drop, or error.
- **`errgroup.WithContext` for fan-out** so the first failure cancels its siblings. A bare `go func()` with no lifecycle owner is a leak.
- **Do not mutate caller-owned slices or maps.** `append` aliasing on a shared backing array corrupts silently.
- **A mutex or `sync/atomic` — never "it is just one field."** Run `go test -race`; a race is a bug even when tests pass.
- **Guard the mutex, not the method.** Never make a network or disk call while holding a lock.

## Logging

- **Testable logging is a narrow `Logger` interface field on a `Deps`-style struct, nil-checked, never a mandatory constructor argument.** Define the smallest interface the struct actually calls (`Info(msg string, args ...any)`, or similar), accept it as an optional field, and nil-check at every call site or wrap it once behind a helper that no-ops on `nil`. When unset, default to a thin adapter over the package or SDK's own global logging funcs — never assume an existing global logger is untouchable just because one exists. Log output is then assertable by injecting a fake `Logger`, without every constructor taking a logger it doesn't need.

## Errors and resilience

- **Wrap with `%w`, match with `errors.Is` / `errors.As`.** Never string-compare error text. Sentinels for expected conditions, typed errors when the caller needs structure.
- **Prefer `errors.AsType[T]` over `errors.As` for new code once `go.mod`'s floor reaches the release that introduced it (Go 1.26)** — `v, ok := errors.AsType[*MyErr](err)` returns the typed value with no pointer target, no reflection. Not deprecating `errors.As`; don't mass-migrate existing calls. `gopls`'s `errorsastype` analyzer flags eligible ones.
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

Exported API and schema changes are additive. New optional fields and new functions are safe; renaming, removing, or retyping an exported symbol breaks every importing service and needs a version bump. Add to a struct rather than changing a signature; use functional options so a constructor can grow.

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

Stories `git-repo-init` splices into `Cross-Cutting` on Create, or appends if missing on Scaffold/Refresh.

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

Same shape as `go-api` — one entrypoint at `cmd/server/main.go`, no audience-split binaries — with `tools.md` replacing `openapi.yaml` as the contract file: a running list of registered MCP tools (name, input schema, output shape), not a REST route table. `internal/` holds tool implementations; a tool is a routing concern like a handler in `go-api`, never a second binary.

**No established Go MCP SDK is recorded here yet.** `git-repo-init`'s Step 5 stops and asks rather than guessing an import path — see that skill for the rule. Once one is confirmed for a real build, it belongs in `reference.md`'s wiring section, not re-decided per repo.

## Seed backlog — `go-mcp`

Stories `git-repo-init` splices into `Cross-Cutting` on Create, or appends if missing on Scaffold/Refresh.

- [M] Flesh out `tools.md` and the tool registry beyond the `/health` stub as tools land · S
- [M] Tests: unit + component coverage · S → `make test`

## Reference material

- **[reference.md](reference.md)** — a complete comment-free vertical slice: service aggregate, handler, two-tier repository, outbound client, wiring, component test.
- **[testing.md](testing.md)** — the approved test stack, the tier ladder, file placement, coverage floors. Gate/floor definitions there defer to the `standards-sdlc` skill.
