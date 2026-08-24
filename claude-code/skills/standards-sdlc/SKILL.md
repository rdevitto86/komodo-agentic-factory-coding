---
name: standards-sdlc
description: SDLC standard shared across every language — test tiers, merge/release gates, coverage floors. Load before reading or writing any test file.
user-invocable: false
paths: "**/*_test.*, **/*.test.*, **/*.spec.*, **/test/**, **/tests/**, **/__tests__/**, **/e2e/**"
---

# SDLC

Defines the tiers and what each one is for, language-agnostically. Every language skill (`standards-go`, `standards-typescript`, `standards-python`, …) owns its own tooling, folder mechanics, and repo-scaffold layout, and must not contradict this. When each tier runs, and what happens when it fails, is owned by the `standards-cicd` skill.

**This file names no framework.** The approved stack per tier lives in the language skill's testing reference — `standards-go/testing.md` and `standards-typescript/testing.md`; `standards-python` carries its own inline. Read the one matching the repo before choosing a test tool.

**[reference.md](reference.md)** carries execution — where each tier runs, parallelism, isolation, environment cost. Load it for a pipeline or suite-design question. **Writing a test needs only this file.**

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

## Rules

- **Nothing that needs a deployed service gates a merge.** The CI stage is hermetic; a test needing a deployed endpoint belongs to a STG tier, whatever folder it sits in.
- **Depth lives at the gate.** Unit, component, and contract prove behaviour exhaustively. Every STG tier is deliberately narrow.
- **Never let a lower tier depend on a higher one.** Unit tests pass with no infrastructure present.
- **Component is the only discretionary tier.**
- **A test that did not run never reports as passed.** Skipped-on-outage is its own visible state.

**Execution rules — when tiers run, parallelism, isolation, environment cost — live in [reference.md](reference.md).** Read it for a pipeline or suite-design question, not to write a test.
