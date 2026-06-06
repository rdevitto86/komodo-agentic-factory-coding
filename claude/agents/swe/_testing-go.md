# Testing Standards (Go)

Applies to all Go services and libraries. JS/TS rules live in [`testing-ts.md`](testing-ts.md).

---

## 0. Approved testing stack

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

---

## 1. Test tier decorators

Unit, component, and integration tests coexist in the same `_test.go` file. Mark each test with a tier decorator — a `testutil` skip-helper called as the first line of the test body. Unit tests need no decorator (they always run).

```go
func TestFoo_PureLogic(t *testing.T) {
}

func TestFoo_WithMocks(t *testing.T) {
    testutil.Component(t)
}

func TestFoo_WithRedis(t *testing.T) {
    testutil.Integration(t)
}

func TestFoo_Contract(t *testing.T) {
    testutil.E2E(t)
}

func TestFoo_Latency(t *testing.T) {
    testutil.Chaos(t)
}
```

The `testutil` helpers call `t.Skip` unless the matching env var is set:

| Decorator | Env var to enable | When it runs |
|-----------|-------------------|--------------|
| _(none)_ | always | Every commit |
| `testutil.Component(t)` | `TEST_COMPONENT=1` | Every commit (opt-in) |
| `testutil.Integration(t)` | `TEST_INTEGRATION=1` | Merge to main |
| `testutil.E2E(t)` | `TEST_E2E=1` | Release tags |
| `testutil.Chaos(t)` | `TEST_CHAOS=1` | Release tags |

---

## 2. File placement and naming

- Tests are colocated with the file under test: `order_service_test.go` next to `order_service.go`.
- Use the `_test` package (black-box) by default. Drop the `_test` suffix only when internal behavior genuinely must be tested.
- Test binaries and fixtures live under `testdata/` (Go ignores this directory for builds).

---

## 3. Structure

- Table-driven tests with `t.Run` subtests for every non-trivial function.
- Each test case is independently runnable — no shared mutable state, no test ordering dependencies.
- Helper functions take `t *testing.T` as the first arg: `newTestServer(t)`, `mustParseTime(t, s)`.
- Call `t.Helper()` inside helpers so failure lines point to the caller.
- Use `t.Cleanup(...)` for teardown; never rely on `defer` inside the test for shared resources.

**Section banners** — unit, component, and integration tests live together in a single `_test.go` file, decorated per §1, with each tier separated by a section banner. The exact banner format is defined in `comments.md`; use it verbatim. Only include sections that are relevant — omit empty ones. E2E tests that span multiple services live in a top-level `e2e/` directory; colocated E2E tests use the `testutil.E2E(t)` decorator (see §1).

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

---

## 4. Mocking and boundaries

- Use `go.uber.org/mock` (mockgen) for interface mocks. Run `mockgen -source=<file>` against API interface files.
- Mock at the interface boundary — never mock concrete types.
- For outbound HTTP calls (connectors, `http/client`), use moxtox as the RoundTripper mock frontend.
- Use `net/http/httptest` for inbound HTTP handler tests. No real network calls in unit or component tests.
- For DB-touching code, use testcontainers-go for real ephemeral instances in integration tests — not dockertest, not heavily-mocked unit tests.

---

## 5. Coverage targets

- >80% on business logic packages.
- 100% on security-critical paths (auth, payments, anything handling PII).
- Coverage is a smoke signal, not a goal — high coverage with weak assertions is worse than honest gaps.

---

## 6. Benchmarks and load testing

- `testing.B` benchmarks live alongside the code they measure: `order_service_bench_test.go` or in the same `_test.go`. Use for SDK hot paths (sanitization, redaction, JWT, ratelimit) where allocation counts and µs-level latency matter.
- For HTTP-serving components (moxtox/server, API endpoints), use k6 load tests — not `testing.B`.
- Benchmark only on stable critical paths; speculative benchmarks rot.
- Compare with `benchstat` against a baseline before claiming a performance change.

---

## 7. Race and timing

- Run `go test -race ./...` in CI. Any test that races is a bug, even if it "usually passes."
- Never `time.Sleep` for synchronization. Use channels, `sync.WaitGroup`, or `context` with deadlines.
- Tests that depend on real wall-clock time must use an injected clock (`clockwork`, custom interface) — see `principles.md` on dependency injection.

---

## 8. Contract and E2E

- Unit, component, and integration tests all live in the same colocated `_test.go` file, separated by section banners (see §3) and decorated per §1.
- E2E flows that span services live under a top-level `e2e/` directory at the repo root and are not governed by colocation rules. Mark them with `testutil.E2E(t)`.
- Use `pact-go` for consumer-driven contract tests. Exchange via file-based pacts — no Pact Broker required. Mark contract tests with `testutil.E2E(t)`; they run on release tags.

---

## 9. Chaos and resilience

- Use Toxiproxy (`github.com/shopify/toxiproxy/v2`) for fault injection tests: latency, jitter, connection drops.
- Spin Toxiproxy via testcontainers-go; wrap the Go client in a `testing/chaos` package.
- Mark chaos tests with `testutil.Chaos(t)`; they run on release tags only.
