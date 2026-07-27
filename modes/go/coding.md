# Go Standards

High-level Go idioms for Komodo. Cross-cutting doctrine — hard rules, error-string format, design/DI principles, code-reuse priority, comments — lives in `~/.claude/standards/principles.md` and `~/.claude/standards/comments.md`; this file does not restate it. Reuse `komodo-forge-sdk-go` before writing custom Go (`~/.claude/standards/principles.md` §2). Project-level configs may extend these but must not contradict them.

---

## 1. Komodo Go conventions

Idiomatic Go — formatting, `%w` wrapping, sentinel errors, naming, package layout, profiling — is assumed. Only the Komodo-specific or non-obvious points:

- `golangci-lint` must pass; config at `.golangci.yaml` (repo root), and its `wrapcheck.ignore-package-globs` must include the forge SDK glob. Disabling a rule needs a justification, and it does **not** go in a trailing comment — the bare `//nolint:<rule>` directive stands alone (a directive is exempt; appended prose is not), and the reason goes in the PR description or `TODO.md`. Prefer fixing the finding over suppressing it. Prefer the `gopls` LSP for quick checks over `go build`/`go vet`.
- Error-string format: `~/.claude/standards/principles.md` §1. Comment/doc rules: `comments.md` — zero godoc, zero comments anywhere, no exceptions. Log once at the top of the stack — never log-and-return.
- HTTP handlers: `req` for request bodies / outgoing `*http.Request`, `res` for response objects; keep `r *http.Request` / `w http.ResponseWriter` by Go convention.
- Test helpers take `t *testing.T` first and call `t.Helper()`.
- Avoid `util`/`common`/`helpers` packages — split by domain; `internal/` for non-importable code; minimize exported surface.
- `init()` does no I/O; wire dependencies through constructors (`NewServer`, `NewService`), never package-level globals or `init()` registration.
- Concurrency: propagate `context.Context` cancellation, manage goroutine lifecycles with `errgroup`/`WaitGroup`, never orphan a `go func()`.
- **Single-call-site functions must pass one of the four tests in `~/.claude/standards/principles.md` §3.3** — dependent setup/wiring, a specific subset of functionality, a concurrency unit, or a deliberate abstraction seam. In Go that means `NewX`/`newX` constructors and option functions qualify as setup; a goroutine or worker-loop body qualifies as a concurrency unit; an interface implementation swapped for a fake qualifies as a seam. A run of statements given a name so `main` or a constructor reads shorter does not qualify — inline it. `func` count is not a quality metric, and a long wiring function is normal Go, not a smell to be broken up.
- Avoid stuttering and vague names (`processData`, `handleStuff`, `doWork`) — a name you can't make specific is usually a function that shouldn't exist.

---

## 2. Types, boundaries & domain modeling

Go expression of `~/.claude/standards/principles.md` §3.2. Not restated here — only how it lands in Go:

- **Named types for domain meaning**: `type UserID string`, `type TimeoutMS int`. Go's implicit conversions between named types and their underlying primitives are narrow enough that this actually catches argument-order bugs at compile time.
- **Parse at the edge into a domain struct.** Decode into a request DTO, convert once into the domain type, and let the service layer take only the domain type. A service method accepting `map[string]any` or a raw `*http.Request` has no boundary.
- **Constructors that can fail return `(T, error)`** — not a zero value plus a separate `Valid()` the caller may forget. If a type has an invariant, the only way to build it is the constructor that checks it.
- **Prefer distinct types over bool flags and optional fields** that are only meaningful in combination. Two mutually exclusive states are two types or a sealed set of constants, not two nullable pointers.
- **Zero values must be usable or impossible.** Either the zero value is a valid, working default (Go's idiom), or the type is unexported with a constructor so a zero value can't escape. A zero value that compiles but panics is the worst of both.
- **JSON**: explicit struct tags on every serialized field; never rely on field-name defaults. Use `DisallowUnknownFields` at untrusted external edges only — internal and versioned payloads tolerate unknown fields (`principles.md` §11).
- **Interfaces stay small and live at the consumer.** One- and two-method interfaces defined in the package that calls them; do not publish an interface next to its only implementation.

---

## 3. Concurrency, state & resources

- **`context.Context` is the first parameter** of any function doing I/O or blocking work, named `ctx`. Never store it in a struct field, and never pass `nil` — `context.TODO()` marks genuinely unresolved plumbing.
- **Every blocking call gets a deadline** via `context.WithTimeout`, and every `WithCancel`/`WithTimeout` gets its `defer cancel()` on the next line. A context without a deadline crossing a network boundary is a defect.
- **`defer` for release, in the same scope as acquire** — `rows.Close()`, `resp.Body.Close()`, `mu.Unlock()`, file handles. Never rely on a later branch to clean up.
- **Bound every channel and worker pool.** Buffered channels get an explicit capacity chosen for a reason; worker counts come from config, not a literal in the loop. Define the behavior at capacity — block, drop, or error — rather than discovering it in production.
- **`errgroup.WithContext` for concurrent fan-out**, so the first failure cancels its siblings. A bare `go func()` with no lifecycle owner is a leak; `sync.WaitGroup` is fine when there are no errors to collect.
- **Don't mutate caller-owned slices or maps.** Append to a fresh slice or document ownership transfer at the boundary. `append` aliasing on a shared backing array is a recurring source of silent corruption.
- **`sync/atomic` or a mutex — never "it's just one field."** Run `go test -race` in CI; a race is a bug even when tests pass (§5.7).
- **Guard the mutex, not the method.** Keep critical sections narrow and never make a network or disk call while holding a lock.

---

## 4. Resilience & error handling

- **Wrap with `%w` and match with `errors.Is`/`errors.As`** — never string-compare error text. Sentinel errors (`var ErrNotFound = errors.New(...)`) for expected conditions; typed errors when the caller needs structured detail.
- **Never discard an error with `_`** outside a `defer` where the failure is genuinely unactionable. No silent zero-value returns in place of a real error (`principles.md` §9).
- **`panic` is for programmer error only** — never for control flow, never across a package boundary. Recover only at the top-level server/goroutine boundary, and log the stack there.
- **Retries carry exponential backoff and jitter, and a cap.** Retry only idempotent operations, and only on errors that are actually transient — never on a 4xx or a validation failure.
- **Idempotency keys on state-changing handlers**, so a client retry converges on the same end state rather than a duplicate write.
- **Graceful shutdown is part of the service**: catch `SIGTERM`/`SIGINT`, call `server.Shutdown(ctx)` with a bounded drain timeout, and close pools in reverse dependency order.
- **Use `crypto/subtle.ConstantTimeCompare`** for tokens, signatures, and secret comparison — never `==` or `bytes.Equal`.

---

## 5. Testing

Full Go testing standard — colocation, table-driven structure, mocking at interface boundaries, coverage targets, race/timing, and integration conventions. JS/TS rules live in `~/.claude/modes/ts/coding.md`.

### 5.0 Approved testing stack

| Tier | Framework | Notes |
|------|-----------|-------|
| Unit | `testing` + `github.com/stretchr/testify` | De facto standard; `assert` + `require` packages |
| Component / Mocking | `go.uber.org/mock` (mockgen) | Interface mock generation; `mockgen -source` against API interfaces |
| Component / HTTP Mocking | moxtox (RoundTripper frontend) | SDK's own tool; use for any package making outbound HTTP calls (connectors, `http/client`) |
| Integration | `github.com/testcontainers/testcontainers-go` | LocalStack, Redis, Postgres, OpenSearch, GCP emulators — created and destroyed by the test run, never a deployed instance |
| Performance | Native `testing.B` benchmarks | For SDK hot paths (sanitization, redaction, JWT, ratelimit) |
| Performance / HTTP Load | k6 | For HTTP-serving components (moxtox/server, API endpoints) only |
| Live dependency (e2e) / Contract | `github.com/pact-foundation/pact-go` | Consumer-driven contracts; file-based pact exchange; no Pact Broker required |
| Chaos / Resilience | Toxiproxy (`github.com/shopify/toxiproxy/v2`) | Via testcontainers-go; wraps the Go client in `testing/chaos` package |
| Chaos / Mutation | gremlins (`github.com/go-gremlins/gremlins`) | Verifies the suite detects injected defects; STG pipeline only |

**Notable rationale:**
- moxtox over WireMock — no JVM, same engine in-process and out-of-process
- testcontainers-go over dockertest — actively maintained, better LocalStack and GCP emulator modules
- pact-go over Specmatic — Specmatic requires Java; pact-go is idiomatic Go with no JVM dependency
- Toxiproxy over Chaos Mesh — Chaos Mesh requires a k8s cluster; Toxiproxy is a lightweight TCP proxy, right-sized for library testing
- `testing.B` over k6 for SDK internals — k6 is HTTP-centric; native benchmarks are the right tool for µs-level latency and allocation counts on middleware hot paths
- gremlins over go-mutesting/ooze for mutation — the `go-mutesting` lineage (original and the avito-tech fork) went inactive in late 2025; gremlins is maintained, sized for microservice-scale modules, and designed to run as a CI quality gate

### 5.1 Test tiers

Tiers form a single ordered ladder, each one including every tier below it:

```
unit < component < integration < e2e < chaos
        └──── DEV layer ────┘   └── STG layer ──┘
         blocks merge to main    post-deploy only
```

The first three are the DEV layer and gate the merge; `e2e` (live dependency) and `chaos` need a deployed environment. Performance sits outside the ladder entirely — see the note under the table below. Component is **discretionary** (`testing.md` §2.2): write them where they earn their keep, not for every change; unit and integration are not optional.

Each test file now belongs to exactly one tier — tier membership is determined by which folder the file lives in (§5.2), not by an in-file marker. `TEST_TIER` and the `testutil` skip-helpers still exist, but their job has narrowed: they no longer separate tiers within a file (folders do that), they control which folders a **default, unscoped `go test ./...` sweep** touches.

- **unit is always-on** — the fast, hermetic default (pure logic), colocated with its source, runs on every invocation with no gate.
- **component, integration, e2e, and chaos live in their own folders under `test/`** (§5.2). Running `go test ./test/integration/...` directly already scopes to that tier — no gate needed for a targeted run. The `testutil` helper still matters for `go test ./...`, which walks every package including `test/...`: without a gate, a bare `go test ./...` would attempt to spin up real infrastructure. Keep one gate call per test file, first line of the body, naming the tier the folder already implies:

| File | Gate call, first line of the body |
|---|---|
| `test/component/order_service_test.go` | `testutil.Component(t)` |
| `test/integration/order_test.go` | `testutil.Integration(t)` |
| `test/e2e/checkout_test.go` | `testutil.E2E(t)` |
| `test/chaos/order_test.go` | `testutil.Chaos(t)` |

Selection is cumulative — setting a tier runs it and everything below it. `-short` is honored as an alias for unit-only and overrides `TEST_TIER`; the default (no flags, unset, or unrecognized `TEST_TIER`) is unit.

| Invocation | Tiers that run | Stage |
|------------|----------------|-------|
| `go test -short ./...` | unit | quick inner loop |
| `go test ./...` | unit (others self-skip via the gate) | every commit |
| `go test ./test/component/...` | component only, ungated by choice of path | local/CI targeted run |
| `TEST_TIER=component go test ./...` | unit + component | local dev loop |
| `TEST_TIER=integration go test ./...` | … + integration | **PR build — blocks the merge to main** |
| `TEST_TIER=e2e go test ./...` | … + live-dependency (e2e) | STG / QA / UAT deploy |
| `TEST_TIER=chaos go test ./...` | … + chaos | STG pipeline |

`TEST_TIER=integration` is the DEV-layer gate: it runs the full merge-blocking set (unit + component + integration) in one invocation. Everything above it needs a deployed environment. Tier definitions, coverage floors, and the environment/pipeline mapping are owned by `~/.claude/standards/testing.md` — this section only covers the Go mechanics.

**Performance is not `TEST_TIER`-gated.** `testing.B` benchmarks don't run under `go test` without `-bench`, and k6 isn't a Go test at all, so both are invoked by their own STG pipeline job (`go test -bench=. ./test/perf/...`) rather than through the ladder. There is no `testutil.Perf` helper and none should be added.

The skip-helpers (`Component`, `Integration`, `E2E`, `Chaos`) live in the SDK `testutil` package — a single ordered comparison against `TEST_TIER`, not independent env vars. Do not redefine them per service.

### 5.2 File placement and naming

- **Unit tests stay colocated**: `order_service_test.go` next to `order_service.go`. Use the `_test` package (black-box) by default. Drop the `_test` suffix only when internal behavior genuinely must be tested.
- **Component, integration, e2e, and chaos tests move to a top-level test root** — `test/` or `tests/`; pick one spelling per repo and stay consistent within it, neither spelling is canonical org-wide. One subfolder per tier:

```
test/
├── component/
├── integration/
├── perf/
├── chaos/
└── e2e/
```

- **Flat by feature inside each tier folder** — do not mirror the source package tree. `test/integration/order_test.go`, not `test/integration/internal/order/service_test.go`. Name the file after the feature/domain under test, not the source file path.
- Test binaries and fixtures live under `testdata/` (Go ignores this directory for builds); a `test/testdata/` (or `test/<tier>/testdata/`) works the same way for the moved-out tiers.

**SDK-internal micro-benchmark exception.** `testing.B` benchmarks for SDK hot paths (sanitization, redaction, JWT, ratelimit — §5.6) sometimes need same-package access to unexported internals for allocation-level measurement. A black-box `test/perf/` file in package `foo_test` cannot reach those unexported symbols. Two options, in order of preference:
1. Export a minimal test hook (a thin wrapper or exported constructor) so the benchmark can live black-box in `test/perf/` like everything else — preferred, keeps the layout uniform.
2. If exporting a hook would leak internals into the public API surface for no reason beyond testing, keep that specific benchmark colocated as `<file>_bench_test.go` in the internal package, and record why it's an exception in `TODO.md` — not in a comment. The `_bench_test.go` filename is the separation; no comment marks the section. Treat this as a documented carve-out, not a default — most perf work still belongs in `test/perf/`.

### 5.3 Structure

- Table-driven tests with `t.Run` subtests for every non-trivial function.
- Each test case is independently runnable — no shared mutable state, no test ordering dependencies.
- Helper functions take `t *testing.T` as the first arg: `newTestServer(t)`, `mustParseTime(t, s)`.
- Call `t.Helper()` inside helpers so failure lines point to the caller.
- Use `t.Cleanup(...)` for teardown; never rely on `defer` inside the test for shared resources.

**Section banners** are reserved for `Setup` and `Helpers` only — not tiers (each file has exactly one tier by virtue of its folder or, for unit tests, its colocation with the source) and not any other label. Format is defined in `~/.claude/standards/comments.md`; use it verbatim. Only include sections that are relevant — omit empty ones. Every other grouping gets no comment at all, plain or banner — name it through identifiers or a separate file.

```go
func TestOrderService_Cancel(t *testing.T) {
    cases := []struct {
        name      string
        order     Order
        wantErr   error
    }{
        {"already shipped", Order{Status: Shipped}, ErrNotCancelable},
        {"pending", Order{Status: Pending}, nil},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            err := svc.Cancel(t.Context(), tc.order)
            require.ErrorIs(t, err, tc.wantErr)
        })
    }
}
```

### 5.4 Mocking and boundaries

- Use `go.uber.org/mock` (mockgen) for interface mocks. Run `mockgen -source=<file>` against API interface files.
- Mock at the interface boundary — never mock concrete types.
- For outbound HTTP calls (connectors, `http/client`), use moxtox as the RoundTripper mock frontend.
- Use `net/http/httptest` for inbound HTTP handler tests. No real network calls in unit or component tests.
- For DB-touching code, use testcontainers-go for real ephemeral instances in integration tests — not dockertest, not heavily-mocked unit tests.
- **Ephemeral is not live.** A container the integration run creates and destroys is DEV-layer and merge-gating; a deployed instance shared with anything else is a live dependency and belongs in `test/e2e/` (§5.8). The distinction is ownership of the lifecycle, not whether the engine is real.

### 5.5 Coverage targets

Floors are set by `~/.claude/standards/testing.md` §3 and repeated here only as the numbers you code against:

- **90% minimum on new code**, 100% preferred.
- **100% required** in SDKs and common libraries (`komodo-forge-sdk-go`, shared internal libs) — no exceptions.
- 100% on security-critical paths regardless of package (auth, payments, anything handling PII).
- Measured on unit tests, which are mandatory for all code.
- Coverage is a floor, not a goal — high coverage with weak assertions is worse than honest gaps, and a number reached by asserting nothing fails review.

### 5.6 Benchmarks and load testing

- `testing.B` benchmarks live in `test/perf/`, flat by feature (`test/perf/order_bench_test.go`), black-box (`package foo_test`) by default. Use for SDK hot paths (sanitization, redaction, JWT, ratelimit) where allocation counts and µs-level latency matter. See §5.2 for the documented exception when a benchmark genuinely needs unexported access.
- For HTTP-serving components (moxtox/server, API endpoints), use k6 load tests under `test/perf/` — not `testing.B`.
- Benchmark only on stable critical paths; speculative benchmarks rot.
- Compare with `benchstat` against a baseline before claiming a performance change.

### 5.7 Race and timing

- Run `go test -race ./...` in CI. Any test that races is a bug, even if it "usually passes."
- Never `time.Sleep` for synchronization. Use channels, `sync.WaitGroup`, or `context` with deadlines.
- Tests that depend on real wall-clock time must use an injected clock (`clockwork`, custom interface) — see `~/.claude/standards/principles.md` on dependency injection.

### 5.8 Live dependency (e2e) and contract

The tier is named **live dependency** in `testing.md` §2.4; `e2e` stays the on-disk token (`test/e2e/`, `testutil.E2E(t)`) so nothing needs renaming.

- These call **real deployed services and real data** in STG, QA, or UAT — never locally, and never against a container the test spun up itself (that's integration, §5.4).
- **Keep them thin and happy-path.** They answer "is it working and are the live resources up," not "is every branch correct" — depth belongs in the merge-gating DEV layer.
- Flows that span services live under `test/e2e/` (or `tests/e2e/`), flat by feature, not under a bare top-level `e2e/`. Mark them with `testutil.E2E(t)`.
- Use `pact-go` for consumer-driven contract tests, also under `test/e2e/`, marked `testutil.E2E(t)`. Exchange via file-based pacts — no Pact Broker required.

### 5.9 Chaos

STG pipeline only, never a merge gate. Three kinds of work share this tier (`testing.md` §2.6):

- **Fault injection / resilience** — Toxiproxy (`github.com/shopify/toxiproxy/v2`) for latency, jitter, and connection drops. Spin it via testcontainers-go; the Go client wrapper can live in an internal `testing/chaos` helper package imported by files under `test/chaos/`.
- **Mutation testing** — gremlins (§5.0); verifies the suite actually detects injected defects, catching the high-coverage/weak-assertion suites §5.5 warns about.
- **Advanced security and performance probing** — deeper than the per-file review pass; coordinate with `cyber-security` for offensive work.

Place all three flat by feature under `test/chaos/` and mark them `testutil.Chaos(t)`.

---

## 6. Design & performance

Design doctrine (DI, accept-interfaces/return-concrete, composition, testability, decomposition) is in `~/.claude/standards/principles.md` §3–4; the Go-specific application is §§2–4 above.

**Performance** — measure first, always (`principles.md` §8):

- Profile with `pprof` before optimizing; benchmark critical paths with `testing.B` (§5.6, `benchstat` workflow). An optimization without a before/after number is unreviewable.
- Pre-size with `make([]T, 0, n)` and `make(map[K]V, n)` whenever the count is known — the dominant, lowest-effort allocation win in Go.
- `sync.Pool` for hot-path short-lived allocations only; it costs clarity and is wrong anywhere else.
- `strings.Builder` for repeated concatenation in a loop; pass large structs by pointer, small ones by value.
- Batch queries and calls — `EXPLAIN ANALYZE` before adding an index, and treat N+1 as a bug, not a tuning opportunity.
- Prefer flat slices of structs over slices of pointers on measured hot paths, for cache locality; elsewhere prefer whatever reads clearest.

**Evolution** (`principles.md` §11): exported API and schema changes are additive — new optional fields and new functions are safe; renaming, removing, or retyping an exported symbol is a breaking change needing a version bump. Add to a struct rather than changing a function signature; use functional options so a constructor can grow without breaking callers.
