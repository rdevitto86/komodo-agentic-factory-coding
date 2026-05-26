# Testing Standards (Go)

Applies to all Go services and libraries. JS/TS rules live in [`testing-ts.md`](testing-ts.md).

---

## 1. File placement and naming

- Tests are colocated with the file under test: `order_service_test.go` next to `order_service.go`.
- Use the `_test` package (black-box) by default. Drop the `_test` suffix only when internal behavior genuinely must be tested.
- Test binaries and fixtures live under `testdata/` (Go ignores this directory for builds).

---

## 2. Structure

- Table-driven tests with `t.Run` subtests for every non-trivial function.
- Each test case is independently runnable — no shared mutable state, no test ordering dependencies.
- Helper functions take `t *testing.T` as the first arg: `newTestServer(t)`, `mustParseTime(t, s)`.
- Call `t.Helper()` inside helpers so failure lines point to the caller.
- Use `t.Cleanup(...)` for teardown; never rely on `defer` inside the test for shared resources.

**Section banner format** — unit, component, and integration tests live together in a single `_test.go` file. Separate each section with this exact banner comment (72 chars total, box-drawing dash `─` U+2500):
```
// ── Unit Tests ──────────────────────────────────────────────────────────
// ── Component Tests ─────────────────────────────────────────────────────
// ── Integration Tests ───────────────────────────────────────────────────
```

```go
// cache_client_test.go

// ── Unit Tests ──────────────────────────────────────────────────────────

func TestCacheClient_VerifyOTP(t *testing.T) { ... }

// ── Component Tests ─────────────────────────────────────────────────────

func TestCacheClient_StoreAndRetrieve(t *testing.T) { ... }

// ── Integration Tests ───────────────────────────────────────────────────

func TestCacheClient_Redis(t *testing.T) { ... }
```

Only include sections that are relevant — omit empty ones. E2E tests are **not** colocated; they live in a top-level `e2e/` directory and may use a `//go:build e2e` tag.

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

## 3. Mocking and boundaries

- No mocking frameworks. Define minimal interfaces at the point of consumption and use test doubles that implement them.
- Mock at the interface boundary — never mock concrete types.
- Use `net/http/httptest` for HTTP handler tests. No real network calls in unit tests.
- For DB-touching code, prefer integration tests against a real ephemeral instance (`testcontainers-go`, `dockertest`) over heavily-mocked unit tests.

---

## 4. Coverage targets

- >80% on business logic packages.
- 100% on security-critical paths (auth, payments, anything handling PII).
- Coverage is a smoke signal, not a goal — high coverage with weak assertions is worse than honest gaps.

---

## 5. Benchmarks

- `testing.B` benchmarks live alongside the code they measure: `order_service_bench_test.go` or in the same `_test.go`.
- Benchmark only on stable critical paths; speculative benchmarks rot.
- Compare with `benchstat` against a baseline before claiming a performance change.

---

## 6. Race and timing

- Run `go test -race ./...` in CI. Any test that races is a bug, even if it "usually passes."
- Never `time.Sleep` for synchronization. Use channels, `sync.WaitGroup`, or `context` with deadlines.
- Tests that depend on real wall-clock time must use an injected clock (`clockwork`, custom interface) — see `principles.md` on dependency injection.

---

## 7. E2E

- Unit, component, and integration tests all live in the same colocated `_test.go` file, separated by section banners (see §2).
- E2E flows that span services live under a top-level `e2e/` directory at the repo root and are not governed by colocation rules. Gate them with `//go:build e2e`.
