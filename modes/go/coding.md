# Go Standards

High-level Go idioms for Komodo. Cross-cutting doctrine — hard rules, error-string format, design/DI principles, code-reuse priority, comments — lives in `~/.claude/standards/principles.md` and `~/.claude/standards/comments.md`; this file does not restate it. Reuse `komodo-forge-sdk-go` before writing custom Go (`~/.claude/standards/principles.md` §2). Project-level configs may extend these but must not contradict them.

---

## 1. Komodo Go conventions

Idiomatic Go — formatting, `%w` wrapping, sentinel errors, naming, package layout, profiling — is assumed. Only the Komodo-specific or non-obvious points:

- `golangci-lint` must pass; config at `.golangci.yaml` (repo root), and its `wrapcheck.ignore-package-globs` must include the forge SDK glob. No disabling a rule without a comment saying why. Prefer the `gopls` LSP for quick checks over `go build`/`go vet`.
- Error-string format: `~/.claude/standards/principles.md` §1. Comment/doc rules: `comments.md` — zero godoc, zero comments anywhere, no exceptions. Log once at the top of the stack — never log-and-return.
- HTTP handlers: `req` for request bodies / outgoing `*http.Request`, `res` for response objects; keep `r *http.Request` / `w http.ResponseWriter` by Go convention.
- Test helpers take `t *testing.T` first and call `t.Helper()`.
- Avoid `util`/`common`/`helpers` packages — split by domain; `internal/` for non-importable code; minimize exported surface.
- `init()` does no I/O; wire dependencies through constructors (`NewServer`, `NewService`), never package-level globals or `init()` registration.
- Concurrency: propagate `context.Context` cancellation, manage goroutine lifecycles with `errgroup`/`WaitGroup`, never orphan a `go func()`.

---

## 5. Testing

Full Go testing standard — colocation, table-driven structure, mocking at interface boundaries, coverage targets, race/timing, and integration conventions. JS/TS rules live in `~/.claude/modes/ts/coding.md`.

### 5.0 Approved testing stack

| Tier | Framework | Notes |
|------|-----------|-------|
| Unit | `testing` + `github.com/stretchr/testify` | De facto standard; `assert` + `require` packages |
| Component / Mocking | `go.uber.org/mock` (mockgen) | Interface mock generation; `mockgen -source` against API interfaces |
| Component / HTTP Mocking | moxtox (RoundTripper frontend) | SDK's own tool; use for any package making outbound HTTP calls (connectors, `http/client`) |
| Integration | `github.com/testcontainers/testcontainers-go` | LocalStack, Redis, Postgres, OpenSearch, GCP emulators |
| Performance | Native `testing.B` benchmarks | For SDK hot paths (sanitization, redaction, JWT, ratelimit) |
| Performance / HTTP Load | k6 | For HTTP-serving components (moxtox/server, API endpoints) only |
| Contract / E2E | `github.com/pact-foundation/pact-go` | Consumer-driven contracts; file-based pact exchange; no Pact Broker required |
| Chaos / Resilience | Toxiproxy (`github.com/shopify/toxiproxy/v2`) | Via testcontainers-go; wraps the Go client in `testing/chaos` package |

**Notable rationale:**
- moxtox over WireMock — no JVM, same engine in-process and out-of-process
- testcontainers-go over dockertest — actively maintained, better LocalStack and GCP emulator modules
- pact-go over Specmatic — Specmatic requires Java; pact-go is idiomatic Go with no JVM dependency
- Toxiproxy over Chaos Mesh — Chaos Mesh requires a k8s cluster; Toxiproxy is a lightweight TCP proxy, right-sized for library testing
- `testing.B` over k6 for SDK internals — k6 is HTTP-centric; native benchmarks are the right tool for µs-level latency and allocation counts on middleware hot paths

### 5.1 Test tiers

Tiers form a single ordered ladder, each one including every tier below it:

```
unit < component < integration < e2e < chaos
```

Each test file now belongs to exactly one tier — tier membership is determined by which folder the file lives in (§5.2), not by an in-file marker. `TEST_TIER` and the `testutil` skip-helpers still exist, but their job has narrowed: they no longer separate tiers within a file (folders do that), they control which folders a **default, unscoped `go test ./...` sweep** touches.

- **unit is always-on** — the fast, hermetic default (pure logic), colocated with its source, runs on every invocation with no gate.
- **component, integration, e2e, and chaos live in their own folders under `test/`** (§5.2). Running `go test ./test/integration/...` directly already scopes to that tier — no gate needed for a targeted run. The `testutil` helper still matters for `go test ./...`, which walks every package including `test/...`: without a gate, a bare `go test ./...` would attempt to spin up real infrastructure. Keep one gate call per test file, first line of the body, naming the tier the folder already implies:

```go
// test/component/order_service_test.go
func TestOrderService_WithMocks(t *testing.T) {
    testutil.Component(t)
}

// test/integration/order_test.go
func TestOrder_WithPostgres(t *testing.T) {
    testutil.Integration(t)
}

// test/e2e/checkout_test.go
func TestCheckout_Contract(t *testing.T) {
    testutil.E2E(t)
}

// test/chaos/order_test.go
func TestOrder_FaultInjection(t *testing.T) {
    testutil.Chaos(t)
}
```

Selection is cumulative — setting a tier runs it and everything below it. `-short` is honored as an alias for unit-only and overrides `TEST_TIER`; the default (no flags, unset, or unrecognized `TEST_TIER`) is unit.

| Invocation | Tiers that run | Stage |
|------------|----------------|-------|
| `go test -short ./...` | unit | quick inner loop |
| `go test ./...` | unit (others self-skip via the gate) | every commit |
| `go test ./test/component/...` | component only, ungated by choice of path | local/CI targeted run |
| `TEST_TIER=component go test ./...` | unit + component | pre-push / PR |
| `TEST_TIER=integration go test ./...` | … + integration | merge to main |
| `TEST_TIER=e2e go test ./...` | … + e2e | release tags |
| `TEST_TIER=chaos go test ./...` | … + chaos | release tags |

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
2. If exporting a hook would leak internals into the public API surface for no reason beyond testing, keep that specific benchmark colocated as `<file>_bench_test.go` in the internal package, and note in the file (via `TODO.md`, not a comment) why it's an exception. If it needs to be visually separated from the unit tests in that file, use a plain single-line comment (e.g. `// Benchmarks`), not a banner — this is not a structural `Setup`/`Helpers` section. Treat this as a documented carve-out, not a default — most perf work still belongs in `test/perf/`.

### 5.3 Structure

- Table-driven tests with `t.Run` subtests for every non-trivial function.
- Each test case is independently runnable — no shared mutable state, no test ordering dependencies.
- Helper functions take `t *testing.T` as the first arg: `newTestServer(t)`, `mustParseTime(t, s)`.
- Call `t.Helper()` inside helpers so failure lines point to the caller.
- Use `t.Cleanup(...)` for teardown; never rely on `defer` inside the test for shared resources.

**Section banners** are reserved for `Setup` and `Helpers` only — not tiers (each file has exactly one tier by virtue of its folder or, for unit tests, its colocation with the source) and not any other label. Format is defined in `~/.claude/standards/comments.md`; use it verbatim. Only include sections that are relevant — omit empty ones. Any other section label is a plain comment, not a banner.

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
            // ...
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

### 5.5 Coverage targets

- >80% on business logic packages.
- 100% on security-critical paths (auth, payments, anything handling PII).
- Coverage is a smoke signal, not a goal — high coverage with weak assertions is worse than honest gaps.

### 5.6 Benchmarks and load testing

- `testing.B` benchmarks live in `test/perf/`, flat by feature (`test/perf/order_bench_test.go`), black-box (`package foo_test`) by default. Use for SDK hot paths (sanitization, redaction, JWT, ratelimit) where allocation counts and µs-level latency matter. See §5.2 for the documented exception when a benchmark genuinely needs unexported access.
- For HTTP-serving components (moxtox/server, API endpoints), use k6 load tests under `test/perf/` — not `testing.B`.
- Benchmark only on stable critical paths; speculative benchmarks rot.
- Compare with `benchstat` against a baseline before claiming a performance change.

### 5.7 Race and timing

- Run `go test -race ./...` in CI. Any test that races is a bug, even if it "usually passes."
- Never `time.Sleep` for synchronization. Use channels, `sync.WaitGroup`, or `context` with deadlines.
- Tests that depend on real wall-clock time must use an injected clock (`clockwork`, custom interface) — see `~/.claude/standards/principles.md` on dependency injection.

### 5.8 Contract and E2E

- E2E flows that span services live under `test/e2e/` (or `tests/e2e/`), flat by feature, not under a bare top-level `e2e/`. Mark them with `testutil.E2E(t)`.
- Use `pact-go` for consumer-driven contract tests, also under `test/e2e/`. Exchange via file-based pacts — no Pact Broker required. Mark contract tests with `testutil.E2E(t)`; they run on release tags.

### 5.9 Chaos and resilience

- Use Toxiproxy (`github.com/shopify/toxiproxy/v2`) for fault injection tests: latency, jitter, connection drops.
- Spin Toxiproxy via testcontainers-go; place chaos tests flat by feature under `test/chaos/` (the Go client wrapper itself can live in an internal `testing/chaos` helper package, imported by files under `test/chaos/`).
- Mark chaos tests with `testutil.Chaos(t)`; they run on release tags only.

---

## 6. Design & performance

Design doctrine (DI, accept-interfaces/return-concrete, composition, testability) is in `~/.claude/standards/principles.md` §3–4. Performance: profile with `pprof` before optimizing; `sync.Pool` for hot-path short-lived allocations; benchmark critical paths with `testing.B` (§5.6, `benchstat` workflow); `EXPLAIN ANALYZE` before adding an index — N+1 is a bug.
