# Go Standards

High-level Go idioms for Komodo. Cross-cutting doctrine — hard rules, error-string format, design/DI principles, code-reuse priority, comments — lives in `~/.claude/agents/swe/principles.md` and `~/.claude/agents/swe/comments.md`; this file does not restate it. Reuse `komodo-forge-sdk-go` before writing custom Go (`~/.claude/agents/swe/principles.md` §2). Project-level configs may extend these but must not contradict them.

---

## 1. Formatting & tooling

- All code formatted with `gofmt` or `goimports` — no exceptions
- `golangci-lint` must pass before merge; config lives in `.golangci.yaml` at repo root
- No disabling linter rules without a comment explaining why
- Run `go vet ./...` as part of CI
- The `gopls` LSP is the standard editor integration — formatting, lint diagnostics, refs, and rename. Agents should use the LSP tool when available before falling back to `go build` / `go vet` for quick checks; IDEs auto-discover it

---

## 2. Error handling

- Error message format (verb-leading, no function/noun prefix) follows `~/.claude/agents/swe/principles.md` §1 — the single source of truth
- Always handle error return values — never `_` an error
- Wrap with `%w` to preserve the chain: `fmt.Errorf("failed to fetch user: %w", err)`
- Return errors up the call stack; don't log and return. Log once, at the top of the stack where propagation stops
- No `panic` except for truly unrecoverable initialization failures (missing required config at startup)
- Sentinel errors (`var ErrNotFound = errors.New(...)`) for errors callers need to match; `errors.As`/`errors.Is` for inspection

---

## 3. Naming

- Comments and doc-comment requirements: governed entirely by `~/.claude/agents/swe/comments.md` (the single source of truth; it deliberately overrides Go's godoc naming convention)
- Interfaces named by behavior: `Reader`, `Writer`, `Handler` — not `IReader`, `ReaderInterface`
- Avoid stutter: `user.Service` not `user.UserService`
- Unexported identifiers: camelCase, no underscores
- Acronyms: consistent casing — `userID`, `httpURL`, `parseJSON` (all caps or all lower, never mixed)
- Test helper functions: `newTestServer(t)`, `mustParseTime(t, s)` — take `t *testing.T` as first arg
- HTTP handler variables: `req` for request objects (e.g. decoded body structs, outgoing `*http.Request`), `res` for response objects (e.g. `*http.Response`, response body structs) — Go handler signatures keep `r *http.Request` and `w http.ResponseWriter` by language convention

---

## 4. Package design

- One package per directory
- Package name = what it provides, singular, lowercase: `order`, `payment`, `middleware`
- Avoid `util`, `common`, `helpers`, `misc` — if you can't name it, split it differently
- Internal packages (`internal/`) for code that must not be imported outside the module
- Minimize exported surface — unexport everything that doesn't need to be consumed externally

---

## 5. Testing

Full Go testing standard — colocation, table-driven structure, mocking at interface boundaries, coverage targets, race/timing, and integration conventions. JS/TS rules live in `~/.claude/agents/swe/ts/coding.md`.

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

All tiers coexist in the same colocated `_test.go` file. Tiers form a single ordered ladder, each one including every tier below it:

```
unit < component < integration < e2e < chaos
```

The active tier is selected by one env var, `TEST_TIER`, read by `testutil` skip-helpers from the SDK. There is no per-tier boolean and no build tag.

- **unit is always-on** — the fast, hermetic default (pure logic). It carries no marker and runs on every invocation.
- **component, integration, e2e, and chaos are opt-in.** Gate each with the matching `testutil` helper as the first line of the test body. Component uses in-process mocks (`httptest`, moxtox, mockgen); integration, e2e, and chaos touch real infrastructure or span services.

```go
func TestFoo_PureLogic(t *testing.T) {
    // unit — always runs
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

func TestFoo_FaultInjection(t *testing.T) {
    testutil.Chaos(t)
}
```

Selection is cumulative — setting a tier runs it and everything below it. `-short` is honored as an alias for unit-only and overrides `TEST_TIER`; the default (no flags, unset, or unrecognized `TEST_TIER`) is unit.

| Invocation | Tiers that run | Stage |
|------------|----------------|-------|
| `go test -short ./...` | unit | quick inner loop |
| `go test ./...` | unit | every commit |
| `TEST_TIER=component go test ./...` | unit + component | pre-push / PR |
| `TEST_TIER=integration go test ./...` | … + integration | merge to main |
| `TEST_TIER=e2e go test ./...` | … + e2e | release tags |
| `TEST_TIER=chaos go test ./...` | … + chaos | release tags |

**Identifying component tests.** Gate each component test with `testutil.Component(t)` as the first line of the body, and group them under the `// ── Component Tests ──` section banner (see §5.3). The marker is the source of truth for tier membership; the banner aids readability.

The skip-helpers (`Component`, `Integration`, `E2E`, `Chaos`) live in the SDK `testutil` package — a single ordered comparison against `TEST_TIER`, not independent env vars. Do not redefine them per service.

### 5.2 File placement and naming

- Tests are colocated with the file under test: `order_service_test.go` next to `order_service.go`.
- Use the `_test` package (black-box) by default. Drop the `_test` suffix only when internal behavior genuinely must be tested.
- Test binaries and fixtures live under `testdata/` (Go ignores this directory for builds).

### 5.3 Structure

- Table-driven tests with `t.Run` subtests for every non-trivial function.
- Each test case is independently runnable — no shared mutable state, no test ordering dependencies.
- Helper functions take `t *testing.T` as the first arg: `newTestServer(t)`, `mustParseTime(t, s)`.
- Call `t.Helper()` inside helpers so failure lines point to the caller.
- Use `t.Cleanup(...)` for teardown; never rely on `defer` inside the test for shared resources.

**Section banners** — unit, component, and integration tests live together in a single `_test.go` file, gated per §5.1, with each tier separated by a section banner. The exact banner format is defined in `~/.claude/agents/swe/comments.md`; use it verbatim. Only include sections that are relevant — omit empty ones. E2E tests that span multiple services live in a top-level `e2e/` directory; colocated E2E tests use the `testutil.E2E(t)` helper.

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

- `testing.B` benchmarks live alongside the code they measure: `order_service_bench_test.go` or in the same `_test.go`. Use for SDK hot paths (sanitization, redaction, JWT, ratelimit) where allocation counts and µs-level latency matter.
- For HTTP-serving components (moxtox/server, API endpoints), use k6 load tests — not `testing.B`.
- Benchmark only on stable critical paths; speculative benchmarks rot.
- Compare with `benchstat` against a baseline before claiming a performance change.

### 5.7 Race and timing

- Run `go test -race ./...` in CI. Any test that races is a bug, even if it "usually passes."
- Never `time.Sleep` for synchronization. Use channels, `sync.WaitGroup`, or `context` with deadlines.
- Tests that depend on real wall-clock time must use an injected clock (`clockwork`, custom interface) — see `~/.claude/agents/swe/principles.md` on dependency injection.

### 5.8 Contract and E2E

- E2E flows that span services live under a top-level `e2e/` directory at the repo root and are not governed by colocation rules. Mark them with `testutil.E2E(t)`.
- Use `pact-go` for consumer-driven contract tests. Exchange via file-based pacts — no Pact Broker required. Mark contract tests with `testutil.E2E(t)`; they run on release tags.

### 5.9 Chaos and resilience

- Use Toxiproxy (`github.com/shopify/toxiproxy/v2`) for fault injection tests: latency, jitter, connection drops.
- Spin Toxiproxy via testcontainers-go; wrap the Go client in a `testing/chaos` package.
- Mark chaos tests with `testutil.Chaos(t)`; they run on release tags only.

---

## 6. Design patterns

Follow the design doctrine in `~/.claude/agents/swe/principles.md` §3–4 (dependency injection, accept interfaces / return concrete types, explicit wiring, composition, testability). Go-specific application:

- Assemble the dependency graph with constructor functions (`NewServer(...)`, `NewService(...)`) — not package-level globals, registries, or `init()` registration
- `init()` must not perform I/O — no network calls, file opens, or DB connections; init failures produce unclear errors and untestable startup sequences

---

## 7. Concurrency

- Never share mutable state without synchronization
- Prefer channels for coordination between goroutines; mutexes for protecting shared state
- Document goroutine lifetimes — who starts it, what stops it, what happens on error
- Always propagate and respect `context.Context` cancellation
- Use `sync.WaitGroup` or `errgroup` for goroutine lifecycle management
- `go func()` with no done signal is almost always wrong — capture and handle the goroutine

---

## 8. Performance

- Profile before optimizing — `pprof` is built in, use it
- Avoid premature allocation: prefer stack allocation, reuse slices with `[:0]`, use `strings.Builder`
- `sync.Pool` for frequently allocated short-lived objects on hot paths
- Benchmark critical paths with `testing.B` — see §5.6 for placement and the `benchstat` workflow
- Database queries: use `EXPLAIN ANALYZE` before adding an index; N+1 queries are a bug
