---
name: sdlc
description: SDLC standard shared across every language — test tiers, merge/release gates, coverage floors. Load before reading or writing any test file.
user-invocable: false
---

# SDLC

Defines the tiers and what each one is for, language-agnostically. Every language skill (`go`, `typescript`, `python`, …) owns its own tooling, folder mechanics, and repo-scaffold layout, and must not contradict this. When each tier runs, and what happens when it fails, is owned by the `cicd` skill.

**This file names no framework.** The approved stack per tier lives in the language skill's testing reference — `go/testing.md` and `typescript/testing.md`; `python` carries its own inline. Read the one matching the repo before choosing a test tool.

## Where each tier runs

| Tier | DEV (local) | CI (PR) | STG (post-merge) |
|---|---|---|---|
| unit | ✓ | **gate** | — |
| component | ✓ | **gate** | — |
| contract | ✓ | **gate** | — |
| smoke | ✓ | — | ✓ first, fast-fail |
| integration | ✓ | — | ✓ |
| live-dependency (e2e) | debugging only | — | ✓ |
| performance | — | — | flagged only |
| chaos | — | — | scheduled or on demand |

**CI is the merge gate**, and it is hermetic — it never reaches a deployed environment. STG gates the release, not the merge.

**A developer can run every STG-scoped tier locally.** That is the point: a defect found before the PR exists costs one command, and the same defect found post-merge costs a build failure and a rollback. Local runs use the same tests, seeding, setup, and teardown as the pipeline — there is no local-only variant.

## Targets

- **`-mock` is the default** — LocalStack or an HTTP interceptor stands in for the dependency.
- **`-live` is explicit** and points at STG, using real STG credentials loaded the way the service loads them.
- **PROD is never a test target.** Enforced mechanically in test setup, not by convention.

## Tiers

- **Unit** — mandatory everywhere. Pure logic, fully hermetic: no network, no disk, no clock, no container. Blocks the merge.
- **Component** — slightly larger interconnected segments; the barrier between unit and integration. Ephemeral infrastructure is allowed (LocalStack, a container the run creates). **Discretionary** — add them where they earn their keep, not for every change. Blocks the merge when present.
- **Contract** — consumer-driven contracts between a service and its callers. File-based pact exchange, both sides verified in the same run, no broker. Cheap and fast, so it gates the merge: a contract break is caught before the code lands, not after a deploy.
- **Smoke** — extended health checks proving the deployed endpoints are ready to be tested. Fast-fail, serial, seconds. Decides hard/soft dependency backout and starts the blue/green flip; a smoke failure means no flip and a roll back to the last stable build.
- **Integration** — real or mocked endpoints in **limited scope**. Not exhaustive: depth is the merge gate's job, and an integration suite that re-proves unit-level behaviour only slows the release.
- **Live dependency** — **happy paths only, all real endpoints and real data** in STG. The question is "is it working end to end," not "is every branch correct." Runs locally only as a debugging aid; the STG run is the authority. On disk the token stays `e2e`.
- **Performance** — benchmarks, throughput, and soak/leak runs. Scheduled or opted into by label; never automatic on a merge.
- **Chaos** — mutation testing, fault injection (latency, jitter, connection drops), and deeper security probing. Scheduled or on demand.

## Layout

**Unit tests are colocated, in every language.** They sit beside the code they cover, always. A unit test filed away from its source is a placement defect, whatever the language's convention says.

**Every other tier lives under one top-level test root**, one subfolder per tier:

```
test/
├── component/
├── contract/
├── integration/
├── e2e/
├── smoke/
├── perf/
└── chaos/
```

- **The root is `test/`** unless the toolchain forces otherwise — Rust's `tests/` and Python's `tests/` are the standing exceptions, named by their language skill.
- **Every subfolder is optional.** Create one when that tier has tests; never scaffold an empty tier.
- **A tier may fold back into the unit files** when the language demands it. Go component tests needing unexported access are the standard case, and they keep their tier gate call.
- **Flat by feature inside each folder.** Never mirror the source tree.

## Suites

Scope comes from position in the tree, not from nesting inside the file.

- **Directory sets domain scope** — app, then feature.
- **File targets one module**, named for the unit under test.
- **Cases group flat, 0–1 levels deep.** One level of subtests under a suite is the ceiling; a subtest inside a subtest is never correct.
- **Titles state a scenario** — "rejects an expired card", not "test cancel error".

## Helpers and setup

Placement follows usage and nothing else:

| Used by | Lives in |
|---|---|
| One file | Bottom of that file |
| One package | Sibling file, same directory |
| Everywhere | Central test utility package |

- **Unexported at the first two levels.** Only the central package exports, because it has to.
- **In-file helpers sit at the bottom under one banner** — `// --- Helpers ---`, comment token matching the language. It is the only banner permitted. There is no `Setup` banner; setup functions live under `Helpers` with everything else.
- **A tier with its own fixtures gets an idiomatic file** — Go `helpers.go` / `setup.go`, TS `helpers.ts` / `fixtures.ts`, Rust `mod.rs`.

**Prefer an explicit helper call in the test body over an implicit lifecycle hook.** `createTestContext(...)` inside the case makes setup lazy, visible where it is used, and safe to parallelise. A hook chain forces every case in the file through setup it may not need, and the dependency is invisible from the test itself. Languages with no hook mechanism — Go, Rust — already work this way and need no change.

Where hooks are the idiom (`beforeEach`/`beforeAll`, `setUp`, fixture scopes):

- **Per-case hooks run N times.** A 10ms reset across 1,000 cases is 10 seconds of non-test time. Keep them to resetting mutated state and rebuilding light mocks.
- **Heavy immutable setup runs once** at file or suite scope — connection pools, mock servers, base configuration.
- **Anything built once is read-only.** A case needing to mutate a shared fixture clones it locally; it never rebuilds the shared one.
- **Allocate lazily.** Nothing heavy is constructed at module or file scope — a run filtered to one case should not pay for the other forty.

## Descriptions

**Optional and rare.** The test's name carries its intent. A description exists only for context a name cannot hold — a corner case, a non-obvious invariant, why an assertion is the shape it is.

- **One or two lines, directly above the declaration.** Line comments only, never a block comment or docstring.
- **Never a restatement of the name.** If it paraphrases the identifier, delete it.
- **Everything else stays comment-free.** The zero-comment rule is unchanged outside this slot and the banner.

## Coverage

**100% is the target for unit coverage, everywhere.** Write the suite as if the floor were 100 and justify what you leave out, rather than aiming at a number below it.

| Scope | Floor |
|---|---|
| Any new code | **85% hard minimum** |
| SDKs and shared libraries | **100%, required** |
| Security-critical (auth, payments, PII) | **100%, required** |

**85% is a failure threshold, not a budget.** Below it the build fails; between 85 and 100 the gap is something you can name and defend, not headroom to spend. A PR that drops a package's number needs a reason in the description.

Measured on unit tests. **Coverage is a floor, not a goal** — high coverage with weak assertions is worse than an honest gap, and a number hit by asserting nothing fails review.

## Test environment cost

Three settings dominate a suite's wall-clock time. Each is **a value the test supplies to the same code path**, never a second path selected by a test flag. If the code under test has to read `IS_TEST` to pick a number, the knob is in the wrong place — move it to the constructor and inject it.

- **Work factors drop to the legal minimum.** Bcrypt, Argon2, and PBKDF2 are designed to burn CPU: a factor of 10–12 costs ~100ms per hash. Inject the lowest the algorithm accepts (bcrypt 4) and an auth suite routinely loses 80–90% of its runtime.
- **Client timeouts drop to 50–100ms.** The 30-second default parks a worker for half a minute on every retry, backoff, and connection-failure case.
- **Loggers write to a null sink.** Thousands of lines to stdout is measurable IO. Buffer and emit only when the test fails.

**Production defaults are unchanged in all three cases.** Verifying the real work factor, the real timeout, and the real log destination is its own small test against the configuration, not a reason to keep the expensive value everywhere else.

## Isolation and teardown

- **Roll back rather than clean up.** Wrap a database-touching test in a transaction and roll it back at teardown. It drops per-case cleanup from hundreds of milliseconds to near zero, and it cannot leave residue.
- **Reset at the start, not only at the end.** A test that panics or times out skips its teardown, and the next run inherits the mess. Make state self-contained with a per-run namespace, or reset during setup so a crashed predecessor cannot poison you.
- **Never sleep to synchronise.** A fixed delay is simultaneously slower than needed and flaky under CI load. Poll the condition on a tight interval against a hard deadline, or wait on a signal the system already emits.
- **Substitute in-memory at the hermetic tiers.** An in-process fake or an in-memory engine avoids socket setup entirely; a container belongs to component and above.

## Rules

- **Nothing that needs a deployed service gates a merge.** The CI stage is hermetic; a test needing a deployed endpoint belongs to a STG tier, whatever folder it sits in.
- **Depth lives at the gate.** Unit, component, and contract are where behaviour is proven exhaustively. Every STG tier is deliberately narrow — integration is limited-scope, e2e is happy-path.
- **Nothing runs on edit or save.** No watchers, no save hooks. The developer decides when tests run.
- **A tier that only runs in CI is broken.** Every tier a developer can usefully run has a local invocation, using the same code the pipeline uses.
- **Component is the only discretionary tier.**
- **Never let a lower tier depend on a higher one.** Unit tests pass with no infrastructure present.
- **A test that did not run never reports as passed.** Skipped-on-outage is its own visible state — see the hard/soft dependency rule in the `cicd` skill.
- **Parallelism is preferred, never mandatory.** Prefer it for unit, component, and contract, where the only cost is the discipline of owning your own fixtures. A suite that stays serial is not a defect and needs no apology.
- **Never add it in bulk.** Parallel tests resume together inside one process, so a package with mutable package-level state or environment writes breaks the moment it is switched on — and the failure is intermittent, not immediate. Add it per package, then prove it under a repeated race run.
- **Serial is the default for smoke, integration, e2e, and chaos.** These call real deployed infrastructure. Fanning them out loads the environment under test, so a timeout stops meaning "the code is slow" and starts meaning "the suite competed with itself" — and on a freshly flipped deployment that load lands exactly when the service is least able to absorb it.
- **Opting a deployed-tier suite into parallel is a deliberate, per-suite decision.** It needs a per-run data namespace, an isolated dependency, and a stated reason. Absent all three, keep it serial.
- **Either way, serial is declared, never accidental.** A test that must not run alongside others says so itself, never relying on run order or a single-worker flag.
- **Process isolation is what makes environment writes safe.** Thread- and goroutine-based runners share one process, so a case writing an env var or a package-level global corrupts its siblings. A process-pool runner isolates that at the cost of per-worker startup — choose the pool to match what the suite mutates, rather than assuming the default is safe.
- **Namespace every run's data.** Seeding, setup, and teardown are owned by the suite and scoped per run, so concurrent runs against the same environment do not collide.

## Why this split

The merge gate is the seconds-scale, infrastructure-free set — fast, cheap, collision-free, and the stage where defects are meant to be caught. Everything deployed moves behind the merge, where an environment exists to support it and a failure rolls back instead of blocking every open PR.
