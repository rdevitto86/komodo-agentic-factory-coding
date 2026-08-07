---
name: sdlc
description: SDLC standard shared across every language — test tiers, merge/release gates, coverage floors. Load before reading or writing any test file.
user-invocable: false
---

# SDLC

Defines the tiers and what each one is for, language-agnostically. Every language skill (`go`, `typescript`, `python`, …) owns its own tooling, folder mechanics, and repo-scaffold layout, and must not contradict this. When each tier runs, and what happens when it fails, is owned by the `ci-cd` skill.

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
- **Smoke** — extended health checks proving the deployed endpoints are ready to be tested. Fast-fail, fully parallel, seconds. Decides hard/soft dependency backout and starts the blue/green flip; a smoke failure means no flip and a roll back to the last stable build.
- **Integration** — real or mocked endpoints in **limited scope**. Not exhaustive: depth is the merge gate's job, and an integration suite that re-proves unit-level behaviour only slows the release.
- **Live dependency** — **happy paths only, all real endpoints and real data** in STG. The question is "is it working end to end," not "is every branch correct." Runs locally only as a debugging aid; the STG run is the authority. On disk the token stays `e2e`.
- **Performance** — benchmarks, throughput, and soak/leak runs. Scheduled or opted into by label; never automatic on a merge.
- **Chaos** — mutation testing, fault injection (latency, jitter, connection drops), and deeper security probing. Scheduled or on demand.

## Coverage

| Scope | Floor |
|---|---|
| New code | **90% minimum**, 100% preferred |
| SDKs and shared libraries | **100%, required** |
| Security-critical paths (auth, payments, PII) | **100%** |

Measured on unit tests. **Coverage is a floor, not a goal** — high coverage with weak assertions is worse than an honest gap, and a number hit by asserting nothing fails review.

## Rules

- **Nothing that needs a deployed service gates a merge.** The CI stage is hermetic; a test needing a deployed endpoint belongs to a STG tier, whatever folder it sits in.
- **Depth lives at the gate.** Unit, component, and contract are where behaviour is proven exhaustively. Every STG tier is deliberately narrow — integration is limited-scope, e2e is happy-path.
- **Nothing runs on edit or save.** No watchers, no save hooks. The developer decides when tests run.
- **A tier that only runs in CI is broken.** Every tier a developer can usefully run has a local invocation, using the same code the pipeline uses.
- **Component is the only discretionary tier.**
- **Never let a lower tier depend on a higher one.** Unit tests pass with no infrastructure present.
- **A test that did not run never reports as passed.** Skipped-on-outage is its own visible state — see the hard/soft dependency rule in the `ci-cd` skill.
- **Parallel by default; serial is declared.** A test that must not run alongside others says so itself, never relying on run order or a single-worker flag.
- **Namespace every run's data.** Seeding, setup, and teardown are owned by the suite and scoped per run, so concurrent runs against the same environment do not collide.

## Why this split

The merge gate is the seconds-scale, infrastructure-free set — fast, cheap, collision-free, and the stage where defects are meant to be caught. Everything deployed moves behind the merge, where an environment exists to support it and a failure rolls back instead of blocking every open PR.
