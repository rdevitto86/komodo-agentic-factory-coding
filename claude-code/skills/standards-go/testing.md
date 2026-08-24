# Go testing

Full Go testing standard — colocation, table-driven structure, mocking at interface boundaries, coverage targets, race/timing, and integration conventions. JS/TS rules live in the `standards-typescript` skill.

## Approved testing stack

| Tier | Framework | Notes |
|------|-----------|-------|
| Unit | `testing` + `github.com/stretchr/testify` | De facto standard; `assert` + `require` packages |
| Component / Mocking | `go.uber.org/mock` (mockgen) | Interface mock generation; `mockgen -source` against API interfaces |
| Component / HTTP Mocking | moxtox (RoundTripper frontend) | SDK's own tool; use for any package making outbound HTTP calls (connectors, `http/client`) |
| Contract | `github.com/pact-foundation/pact-go` | Consumer-driven contracts; file-based pact exchange; no Pact Broker required. Runs on the PR |
| Integration | `github.com/testcontainers/testcontainers-go` under `-mock`; deployed endpoints under `-live` | LocalStack, Redis, Postgres, OpenSearch, GCP emulators — created and destroyed by the test run |
| Smoke | `testing` + `testify` | Extended health check per service, proving endpoints are ready to test. Serial, seconds, no fixtures |
| Performance | Native `testing.B` benchmarks | For SDK hot paths (sanitization, redaction, JWT, ratelimit) |
| Performance / HTTP Load | k6 | For HTTP-serving components (moxtox/server, API endpoints) only |
| Live dependency (e2e) | `testing` + `testify` | Thin happy-path flows against deployed STG services |
| Chaos / Resilience | Toxiproxy (`github.com/shopify/toxiproxy/v2`) | Via testcontainers-go; wraps the Go client in `testing/chaos` package |
| Chaos / Mutation | gremlins (`github.com/go-gremlins/gremlins`) | Verifies the suite detects injected defects; STG pipeline only |

**Notable rationale:**
- moxtox over WireMock — no JVM, same engine in-process and out-of-process
- testcontainers-go over dockertest — actively maintained, better LocalStack and GCP emulator modules
- pact-go over Specmatic — Specmatic requires Java; pact-go is idiomatic Go with no JVM dependency
- Toxiproxy over Chaos Mesh — Chaos Mesh requires a k8s cluster; Toxiproxy is a lightweight TCP proxy, right-sized for library testing
- `testing.B` over k6 for SDK internals — k6 is HTTP-centric; native benchmarks are the right tool for µs-level latency and allocation counts on middleware hot paths
- gremlins over go-mutesting/ooze for mutation — the `go-mutesting` lineage (original and the avito-tech fork) went inactive in late 2025; gremlins is maintained, sized for microservice-scale modules, and designed to run as a CI quality gate

## Test tiers

The hermetic tiers form a single ordered ladder, each one including every tier below it. The environment-scoped tiers do not — they are selected one at a time.

```
unit < component < contract        smoke   integration   e2e   chaos
└──── CI stage, merge gate ─┘      └──── STG-scoped, selected alone ────┘
```

`unit`, `component`, and `contract` gate the merge and are hermetic — the CI stage never reaches a deployed environment. `smoke`, `integration`, `e2e`, and `chaos` are environment-scoped and never accumulate: asking for `e2e` runs `e2e`, not the ladder beneath it. Performance sits outside both — see the note under the table below. Component is **discretionary** : write them where they earn their keep, not for every change; unit, contract, and integration are not optional.

**Every environment-scoped tier runs locally too.** `-mock` is the default target; `-live` points at STG. Same tests, same seeding, same teardown as the pipeline.

Each test file belongs to exactly one tier. **The gate call decides it where one is present; the folder decides it otherwise.** A colocated file with no gate call is unit. `TEST_TIER` and the `testutil` skip-helpers still exist, but their job has narrowed: they no longer separate tiers within a file (folders do that), they control which folders a **default, unscoped `go test ./...` sweep** touches.

- **unit is always-on** — the fast, hermetic default (pure logic), colocated with its source, runs on every invocation with no gate.
- **every other tier lives in its own folder under `test/`** (see **File placement and naming**). Running `go test ./test/integration/...` directly already scopes to that tier — no gate needed for a targeted run. The `testutil` helper still matters for `go test ./...`, which walks every package including `test/...`: without a gate, a bare `go test ./...` would attempt to spin up real infrastructure. Keep one gate call per test file, first line of the body, naming the tier the folder already implies:

| File | Gate call, first line of the body |
|---|---|
| `test/component/order_service_test.go` | `testutil.Component(t)` |
| `test/contract/order_pact_test.go` | `testutil.Contract(t)` |
| `test/integration/order_test.go` | `testutil.Integration(t)` |
| `test/smoke/order_test.go` | `testutil.Smoke(t)` |
| `test/e2e/checkout_test.go` | `testutil.E2E(t)` |
| `test/chaos/order_test.go` | `testutil.Chaos(t)` |

Selection is cumulative **through the hermetic ladder only** — setting `component` or `contract` runs that tier and everything below it. `smoke`, `integration`, `e2e`, and `chaos` are exact matches: they run alone, because each is scoped to an environment the others are not. `-short` is honored as an alias for unit-only and overrides `TEST_TIER`; the default (no flags, unset, or unrecognized `TEST_TIER`) is unit.

| Invocation | Tiers that run | Stage |
|------------|----------------|-------|
| `go test -short ./...` | unit | quick inner loop |
| `go test ./...` | unit (others self-skip via the gate) | every commit |
| `go test ./test/component/...` | component only, ungated by choice of path | local targeted run |
| `TEST_TIER=component go test ./...` | unit + component | local dev loop |
| `TEST_TIER=contract go test ./...` | unit + component + contract | **CI stage — blocks the merge to main** |
| `TEST_TIER=smoke go test ./test/smoke/...` | smoke only | first sub-stage of a STG deploy |
| `TEST_TIER=integration go test ./test/integration/...` | integration only | STG, after the flip |
| `TEST_TIER=e2e go test ./test/e2e/...` | live-dependency only | STG prerelease |
| `TEST_TIER=chaos go test ./test/chaos/...` | chaos only | scheduled or on demand |

`TEST_TIER=contract` is the merge gate: unit + component + contract in one invocation, no deployed dependency anywhere in it.

**Target selection is a separate axis from tier selection.** `TEST_TARGET=mock` (the default) resolves dependencies to LocalStack or an HTTP interceptor; `TEST_TARGET=live` resolves them to STG endpoints with real STG credentials. The same command runs in both modes:

```
TEST_TIER=integration go test ./test/integration/...
TEST_TIER=integration TEST_TARGET=live go test ./test/integration/...
```

**A resolved target on a PROD host aborts the run.** That check lives in the shared test setup, not in each test.

Tier definitions and coverage floors are owned by the `standards-sdlc` skill; stage ordering, express releases, and failure handling by the `standards-cicd` skill. This section only covers the Go mechanics.

**Performance is not `TEST_TIER`-gated.** `testing.B` benchmarks don't run under `go test` without `-bench`, and k6 isn't a Go test at all, so both are invoked by their own STG pipeline job (`go test -bench=. ./test/perf/...`) rather than through the ladder. There is no `testutil.Perf` helper and none should be added.

The skip-helpers (`Component`, `Contract`, `Integration`, `Smoke`, `E2E`, `Chaos`) live in the SDK `testutil` package — an ordered comparison against `TEST_TIER` for the hermetic tiers and an exact match for the deployed ones, not independent env vars. Do not redefine them per service.

## File placement and naming

- **Unit tests stay colocated**: `order_service_test.go` next to `order_service.go`. Use the `_test` package (black-box) by default. Drop the `_test` suffix only when internal behavior genuinely must be tested.
- **Every non-unit tier moves to a top-level `test/`** — canonical for Go, never `tests/`. One subfolder per tier:

```
test/
├── component/
├── contract/
├── integration/
├── smoke/
├── e2e/
├── perf/
└── chaos/
```

- **Component tests may stay colocated when they need unexported access.** `test/component/` is the default. When the test must reach private fields, name it `<name>_component_test.go` beside its source and keep `testutil.Component(t)` as the first line of the body — there the gate call marks the tier, not the folder.
- **Tiers needing their own fixtures get `helpers.go` or `setup.go`** in that tier folder — a plain file, not `_test.go`, so every file in the tier can import it.
- **Flat by feature inside each tier folder** — do not mirror the source package tree. `test/integration/order_test.go`, not `test/integration/internal/order/service_test.go`. Name the file after the feature/domain under test, not the source file path.
- Test binaries and fixtures live under `testdata/` (Go ignores this directory for builds); a `test/testdata/` (or `test/<tier>/testdata/`) works the same way for the moved-out tiers.

**SDK-internal micro-benchmark exception.** `testing.B` benchmarks for SDK hot paths (sanitization, redaction, JWT, ratelimit — see **Benchmarks and load testing**) sometimes need same-package access to unexported internals for allocation-level measurement. A black-box `test/perf/` file in package `foo_test` cannot reach those unexported symbols. Two options, in order of preference:
1. Export a minimal test hook (a thin wrapper or exported constructor) so the benchmark can live black-box in `test/perf/` like everything else — preferred, keeps the layout uniform.
2. If exporting a hook would leak internals into the public API surface for no reason beyond testing, keep that specific benchmark colocated as `<file>_bench_test.go` in the internal package, and record why it's an exception in `BACKLOG.md` — not in a comment. The `_bench_test.go` filename is the separation; no comment marks the section. Treat this as a documented carve-out, not a default — most perf work still belongs in `test/perf/`.

## Structure

- **One suite per module under test**, table-driven with `t.Run` subtests. Scenario titles, one level of subtests, never a subtest inside a subtest — see the `standards-sdlc` skill.
- Each test case is independently runnable — no shared mutable state, no test ordering dependencies.
- Helper functions take `t *testing.T` as the first arg: `newTestServer(t)`, `mustParseTime(t, s)`.
- Call `t.Helper()` inside helpers so failure lines point to the caller.
- Use `t.Cleanup(...)` for teardown; never rely on `defer` inside the test for shared resources.

- Helpers and setup funcs sit at the **bottom of the file, unexported**, under the one permitted banner. Placement across files follows the usage table in the `standards-sdlc` skill.

```go
// --- Helpers ----------------------------------------------------
```

**That banner is the only one.** No `Setup` banner, no tier banner — a file's tier comes from its folder or its gate call, never from a comment. Every other grouping gets no comment at all: name it through identifiers or split the file.

**A one-or-two-line description above a `func Test…` is allowed and optional.** Use it for a corner case or an invariant the name cannot carry, never to paraphrase the identifier. See the `standards-sdlc` skill.

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

## Mocking and boundaries

- Use `go.uber.org/mock` (mockgen) for interface mocks. Run `mockgen -source=<file>` against API interface files.
- Mock at the interface boundary — never mock concrete types.
- For outbound HTTP calls (connectors, `http/client`), use moxtox as the RoundTripper mock frontend.
- Use `net/http/httptest` for inbound HTTP handler tests. No real network calls in unit or component tests.
- For DB-touching code, use testcontainers-go for real ephemeral instances under `-mock` — not dockertest, not heavily-mocked unit tests.
- **The CI stage is hermetic and stays that way.** unit, component, and contract never reach a deployed endpoint under any target setting. LocalStack also stands in for secret fetching there, so a PR runner needs no real credentials.
- **Destroy every container the run created**, via `t.Cleanup`, including on failure. A leaked container is a failing test even when the assertions passed.

## Fixture cost and isolation

Policy is the `standards-sdlc` skill's; these are the Go mechanics for it.

- **Go has no lifecycle hooks and needs none** — an explicit `newTestContext(t)` at the top of the case is already the idiom. `TestMain` is for one-time, package-wide setup only; per-test work never belongs there, because every case in the package pays for it whether it runs or not.
- **Inject the cheap work factor.** `bcrypt.MinCost` (4) in tests against the production cost from config — the constructor takes the cost, the hashing code never inspects an env var to pick one.
- **Discard log output.** `slog.New(slog.NewTextHandler(io.Discard, nil))` for the injected logger, or a `bytes.Buffer` dumped through `t.Log` only when the case fails.
- **Shorten client timeouts.** A retry or connection-failure case sets `http.Client{Timeout: 100 * time.Millisecond}`; the 30-second production default parks a worker for the whole run.
- **One container per package, one transaction per test.** Create the testcontainers instance in `TestMain`, then give each case a `BeginTx` handle rolled back in `t.Cleanup`. Rollback is near-free and leaves nothing behind; truncate-and-reseed costs hundreds of milliseconds per case and survives a panic as orphan rows.

## Coverage targets

Floors are set by the `standards-sdlc` skill and repeated here only as the numbers you code against:

- **100% is the target on all new code**; **85% is the hard minimum** that fails the build.
- **100% required** in SDKs and common libraries (`komodo-forge-sdk-go`, shared internal libs) — no exceptions.
- 100% on security-critical paths regardless of package (auth, payments, anything handling PII).
- Measured on unit tests, which are mandatory for all code.
- Coverage is a floor, not a goal — high coverage with weak assertions is worse than honest gaps, and a number reached by asserting nothing fails review.

## Benchmarks and load testing

- `testing.B` benchmarks live in `test/perf/`, flat by feature (`test/perf/order_bench_test.go`), black-box (`package foo_test`) by default. Use for SDK hot paths (sanitization, redaction, JWT, ratelimit) where allocation counts and µs-level latency matter. See **File placement and naming** for the documented exception when a benchmark genuinely needs unexported access.
- For HTTP-serving components (moxtox/server, API endpoints), use k6 load tests under `test/perf/` — not `testing.B`.
- Benchmark only on stable critical paths; speculative benchmarks rot.
- Compare with `benchstat` against a baseline before claiming a performance change.

## Race and timing

- Run `go test -race ./...` in CI. Any test that races is a bug, even if it "usually passes."
- Never `time.Sleep` for synchronization. Use channels, `sync.WaitGroup`, or `context` with deadlines.
- **Poll for a side effect you cannot receive a signal for.** When the thing you are waiting on is external — a row landing, a worker draining a queue — `require.Eventually` with a tight tick and a hard deadline replaces the sleep. It returns the instant the condition holds instead of always paying the full delay.

```go
require.Eventually(t, func() bool {
    return worker.IsDone()
}, time.Second, 10*time.Millisecond)
```

- Tests that depend on real wall-clock time must use an injected clock (`clockwork`, custom interface) rather than the system clock.

## Contract

- `pact-go`, under `test/contract/`, marked `testutil.Contract(t)`. Exchange via file-based pacts — no Pact Broker required.
- **Both sides run on the PR.** Consumer pacts are generated and the provider verifies them in the same merge-gating invocation, so a contract break never reaches a deploy.
- Hermetic: the provider is verified in-process against the pact file, never against a deployed instance.

## Smoke

- Under `test/smoke/`, marked `testutil.Smoke(t)`, run as the first sub-stage of a STG deploy and the gate on everything after it. Runs locally against either target.
- **Extended health check.** Is the service up, are its dependencies reachable, is the build the one just deployed. No fixtures, no data setup. Express releases assert basic functionality here too, since no other STG tier runs on that path.
- **Fast-fail** — the stage exists to stop a doomed release in seconds, so report the first hard-dependency failure immediately rather than completing the sweep.
- **Serial, and still seconds.** A smoke suite is a handful of health pings; running them in sequence costs nothing and keeps the load off a service that was deployed moments ago.

## Live dependency (e2e)

The tier is named **live dependency** in the `standards-sdlc` skill; `e2e` stays the on-disk token (`test/e2e/`, `testutil.E2E(t)`) so nothing needs renaming.

- These call **real deployed services and real data** in STG, QA, or UAT — always `-live`, never mocked. A developer may run them locally to reproduce a failure; the STG run is the authority.
- **Happy paths only.** They answer "is it working end to end," not "is every branch correct" — depth belongs in the merge-gating tiers.
- Flows that span services live under `test/e2e/`, flat by feature, not under a bare top-level `e2e/`. Mark them with `testutil.E2E(t)`.
- **Seed what you need, tear down what you seeded**, namespaced per run. Never depend on data another run left behind.
- **Classify each dependency hard or soft.** A hard dependency down fails the test; a soft one skips it with an explicit outage reason — see the `standards-cicd` skill. Never let an unreachable dependency produce a pass.

## Parallel and serial

- `t.Parallel()` is **preferred for unit, component, and contract — never mandatory.** Reach for it when the package's tests already own their fixtures; leaving a suite serial is a legitimate outcome, not a gap to be filled.
- **Never add it in bulk.** `t.Parallel()` pauses the test, and the runner resumes every paused test in the package at once. Any package-level `var` a test writes, or any setter that reaches one, starts interleaving — and `t.Setenv` panics outright in a parallel test because the environment is process-wide. Convert one package at a time and confirm with `go test -race -count=2 ./<pkg>/`.
- **Omit `t.Parallel()` in smoke, integration, e2e, and chaos.** Those tiers call real deployed infrastructure; a parallel fan-out loads the environment under test, turning a capacity problem into a phantom test failure. On a just-flipped deployment that load arrives at the worst possible moment.
- **Opting a deployed-tier suite back into parallel takes three things**: its own data namespace per run, a dependency no sibling touches, and a reason worth the risk. Two out of three means serial.
- **`t.Setenv` over `os.Setenv`.** It restores on cleanup and panics in a parallel test, so an env-dependent case is forced to declare itself serial rather than corrupting a sibling.
- **Serial is a declared requirement, never an accident.** A test that must not run alongside others says so at the top of the test — omit `t.Parallel()` and name the shared resource in the test's identifier (`TestOrderReindex_SerialSharedIndex`), not in a comment.
- Never rely on file order, alphabetical ordering, or `-p 1` to get correctness.

## Chaos

STG pipeline only, never a merge gate. Three kinds of work share this tier :

- **Fault injection / resilience** — Toxiproxy (`github.com/shopify/toxiproxy/v2`) for latency, jitter, and connection drops. Spin it via testcontainers-go; the Go client wrapper can live in an internal `testing/chaos` helper package imported by files under `test/chaos/`.
- **Mutation testing** — gremlins (see **Approved testing stack**); verifies the suite actually detects injected defects, catching the high-coverage/weak-assertion suites **Coverage targets** warns about.
- **Advanced security and performance probing** — deeper than the per-file review pass; treat offensive probing as its own scheduled exercise.

Place all three flat by feature under `test/chaos/` and mark them `testutil.Chaos(t)`.

## Testing references

- **Audit testcontainers-go lifecycle on any failure in that tier.** A red integration or component suite is as often a leaked, unready, or cross-test-contaminated container as it is a real regression — check provisioning order, that any suite opted into parallel owns an isolated container and data namespace (see **Parallel and serial**), and that `t.Cleanup` actually ran, before trusting the assertion failure itself.
- **Martyr CI Mode.** When a soft dependency (the hard/soft split under **Live dependency (e2e)**) is unreachable from the CI runner, generate a mock for it from the same interface testcontainers-go or moxtox would have wrapped, rather than skip the test outright — CI is the environment built to absorb this cost so STG doesn't.

---
