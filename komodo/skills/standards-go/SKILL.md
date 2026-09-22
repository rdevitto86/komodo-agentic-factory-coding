---
name: standards-go
description: Go: godoc, error wrapping, interfaces at the consumer, and the standard library first.
globs: ["**/*.go"]
---

# Go

## Comments
- godoc on every exported identifier: name first, one sentence. `// Load reads the config and applies defaults.`
- A private function gets a one-line comment when its body is longer than a screen or its behaviour is not obvious from its name.
- A `//nolint:<rule>` directive is bare; the reason goes in the backlog, not the comment.

## Conventions
- `golangci-lint` must pass with the repo's `.golangci.yaml`. Fix the finding before suppressing it.
- `req` for request bodies and outgoing `*http.Request`, `res` for responses, `r *http.Request` / `w http.ResponseWriter` in handlers.
- No `util`, `common`, or `helpers` packages. Split by domain. `internal/` for non-importable code. Minimise the exported surface.
- `init()` does no I/O. Wire through constructors (`NewServer`, `NewService`), never package-level globals.
- Log once, at the top of the stack. Never log-and-return.
- No vague names (`processData`, `handleStuff`). A name you cannot make specific marks a function that should not exist.
- Extract a literal that stands for a size, TTL, count, threshold, header name, or config key into a named const. Package-level when shared across functions, function-local otherwise. Two or more introduced together go in one `const ( ... )` block. MixedCaps, never `SCREAMING_SNAKE_CASE`.
- A sentinel error or any value compared with `errors.Is`/`errors.As` is a single package-level `var`, never function-local.
- Wrap a long line by collapsing one level at a time: content on its own indented lines, trailing comma, closing paren at the original indent. 120 columns is the lint gate.
- No blank line between consecutive early-return guards; one blank line between the last guard and the happy path.
- A single-call-site pure helper with no domain meaning inlines into its caller. Constructors, concurrency bodies, abstraction seams, and any security or audit check stay named regardless of size.

## Types and boundaries
- Named types carry domain meaning: `type UserID string`, `type TimeoutMS int`.
- Parse at the edge. Decode into a DTO, convert once, and let the service layer accept only the domain type.
- Fallible constructors return `(T, error)`, never a zero value plus a `Valid()`.
- Distinct types over bool flags and over optional fields that only mean something in combination.
- Zero values are usable or impossible. A type that compiles and panics is the worst of both.
- Explicit JSON struct tags on every serialised field. `DisallowUnknownFields` only at untrusted external edges.
- Interfaces are small and live at the consumer. Never publish an interface beside its only implementation.

## Concurrency and resources
- `ctx context.Context` is the first parameter of anything that does I/O or blocks. Never in a struct field, never nil.
- Every blocking call has a deadline. `defer cancel()` on the line after `WithTimeout`/`WithCancel`.
- `defer` releases in the scope that acquires: `rows.Close()`, `resp.Body.Close()`, `mu.Unlock()`.
- Bound every channel and worker pool; define behaviour at capacity.
- `errgroup.WithContext` for fan-out. A bare `go func()` with no owner is a leak.
- Never mutate caller-owned slices or maps.
- A mutex or `sync/atomic` for every shared field. `go test -race` is part of done.
- Never hold a lock across a network or disk call.

## Errors
- Wrap with `%w`, match with `errors.Is`/`errors.As`. Never string-compare error text.
- Never discard an error with `_` outside a `defer` where failure is unactionable.
- `panic` is for programmer error only; recover only at the server or goroutine boundary and log the stack.
- Retries carry exponential backoff, jitter, and a cap, and apply only to idempotent operations on transient errors.
- Idempotency keys on state-changing handlers.
- Graceful shutdown: catch `SIGTERM`/`SIGINT`, `server.Shutdown(ctx)` with a bounded drain, close pools in reverse order.
- `crypto/subtle.ConstantTimeCompare` for tokens, signatures, and secrets.

## Performance
- Measure first with `pprof` and `testing.B`; an optimisation without a before/after number is unreviewable.
- Pre-size with `make([]T, 0, n)` and `make(map[K]V, n)` when the count is known.
- `strings.Builder` for concatenation in a loop. `sync.Pool` only on a measured hot path.
- Batch queries. N+1 is a bug.

## Evolution
Exported API and schema changes are additive. Renaming, removing, or retyping an exported symbol is a breaking change and a version bump. Grow a constructor with functional options.

## Security
- `html/template` for anything rendered to a browser, never `text/template`.
- `os/exec` with an argument slice, never `sh -c` with input.
- `database/sql` placeholders, never string-built queries.
- Decode untrusted input into a concrete, field-limited struct, never `any`.
- `filepath.Clean` plus a root prefix check on any user-supplied path segment.
- `crypto/rand` for tokens, keys, and nonces.
- `InsecureSkipVerify: true` never reaches a committed default.

## Testing
- Table-driven tests with `t.Run` per case; `t.Parallel()` where cases share no state.
- Test the package's public behaviour through its exported surface; a private helper's test is a smell.
- `-race` and `-count=1` in the verify target. No sleeps for synchronisation; use channels or `testing/synctest`.
