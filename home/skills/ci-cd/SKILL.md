---
name: ci-cd
description: Pipeline stages, merge vs release gates, ephemeral CI infra, blue/green, rollback, feature flags.
user-invocable: false
---

# CI/CD

Tier definitions and coverage floors are owned by the `sdlc` skill. This file owns **which stage runs what**, and **what happens on failure**.

Platform-neutral. Every repo declares its pipeline in a build config; a CI/CD service reads that config and executes the stages. The stage model below is fixed — the config selects and parameterises stages, it does not invent them.

## The stages

| Stage | Where | Runs | Gates |
|---|---|---|---|
| **1 · DEV** | The developer's machine | unit, component, contract, smoke, integration | **The commit and the push**, via local hooks |
| **2 · CI** | Ephemeral runner, per PR | unit, component, contract — full | The merge |
| **3 · STG** | Deployed, post-merge | smoke → integration → e2e → perf (flagged) | The release |
| **4 · PROD** | Deployed, on approval | smoke | Nothing after it |

## Stage 1 — DEV

**Nothing runs on edit or save. Ever.** No watchers, no save hooks, no background test runners. The developer decides when tests run.

The point of Stage 1 is that a developer can run **STG-scoped tests before the PR exists**. Finding it locally costs one command; finding it post-merge costs a build failure and a rollback.

### Targets

- **`-mock` is the default** for every tier that has one. Mock responses come from LocalStack or an HTTP interceptor.
- **`-live` is explicit opt-in** and points at STG. It needs real STG credentials, loaded through the SDK Secrets Manager client the same way the service loads them.
- **PROD is never a local test target.** The check is mechanical — a resolved target on a PROD host aborts the run, not a convention anyone can forget.
- **e2e locally is a debugging tool, not a gate.** It works under `-live`; the Stage 3 run is still the authority.

**The local run is the same code as the STG run** — same tests, same seeding, same setup, same teardown. There is no local-only variant to drift.

### Local hooks — the first gates

Stage 1 gates its own output. These are the only gates before a remote runner sees the code, and they are the reason Stage 2 rarely fails on formatting or an obvious regression.

| Hook | Gates | Runs | Mandatory |
|---|---|---|---|
| **pre-commit** | The commit | Language formatter and linter on staged files. **Never tests** | Yes |
| **pre-push** | The push | Unit tests + coverage, delta-scoped to the changed files' packages | No |

Go reports coverage per package, so the pre-push delta is the set of packages containing changed files — the tightest scope the toolchain supports.

**Both are local-only.** No remote container, no runner, no network. A hook that needs infrastructure belongs in Stage 2.

**The hooks ship with the language SDK, not with a repo and not with agent config.** `komodo-forge-sdk-go` owns the Go implementation — `gofmt`, `goimports`, `golangci-lint` on commit; delta `go test -cover` on push. A repo installs from the SDK and points `core.hooksPath` at it; never hand-write a per-repo copy, and never reimplement one because the SDK's is inconvenient. The contract above is what the hook must do; the SDK decides how.

## Stage 2 — CI

**The most important stage.** It is where defects are supposed to be caught, and the only stage that gates the merge.

- **Ephemeral and disposable.** A container per run, torn down when it ends, whatever the result.
- **Hermetic.** Stage 2 never reaches STG. A STG outage must not block every open PR, and PR runners must not hold production-adjacent credentials.
- **LocalStack stands in for secret fetching** and any AWS-shaped dependency, so the run needs no real credentials at all.
- **Runs unit, component, and contract in full.** Contract verifies both sides in-process against file-based pacts — no broker.

### Runner rules

- **Cancel superseded runs** on the same ref. Never cancel a run on main.
- **Path filters** — `docs/`, `*.md`, `scripts/` skip the pipeline.
- **Cache** dependencies, build output, and base images. Re-downloading on every run is a defect.
- **Draft PRs run lint + unit only.** Marking ready for review triggers the full stage.

### Build CLI and manifest

**A single build CLI is the entrypoint for every stage.** It reads PR state — draft vs ready, labels, changed paths — and derives its own invocation from it. Never hand-construct the flags it would derive; a repo's build config parameterises the CLI, it does not replace its judgment.

The CLI streams progress and telemetry to stdout as it runs, then emits a build manifest — coverage, digests, artifact locations. On a failure, triage isolates the cause into exactly one bucket before reporting: a code bug, a broken contract, or a soft-dependency outage. A structured summary — coverage delta, immutable artifact path, container digest — posts back to the PR either way.

## Stage 3 — STG

```
deploy inactive stack → smoke → flip → integration → e2e → [perf] → prerelease
                          └ fail → no flip, roll back to last stable
```

| Sub-stage | What it is |
|---|---|
| **smoke** | Extended health checks proving STG endpoints are ready to be tested. Fast-fail, fully parallel, seconds. Decides hard/soft dependency backout, and starts the inactive→active flip |
| **integration** | Real or mocked endpoints, **limited scope**. Not exhaustive — that was Stage 2's job |
| **e2e** | Happy paths only, **all real endpoints and real data** |
| **perf** | Only when the build config or the PR flags it on. Never automatic |

**Seed before, tear down after**, owned by the suite that needs it and namespaced per run.

### Concurrency

Concurrent STG-scoped runs — two local `-live` sessions, or a local session against a pipeline run — contend on the same data. Two mitigations, in order:

1. **Namespace every run's data.** Makes most concurrent runs safe with no coordination.
2. **The CI/CD service holds a per-service lock** on STG-scoped stages. It already knows what is in flight; a convention that people remember to wait does not.

### Dependency outages

| Class | Down | Result |
|---|---|---|
| **Hard** | Stage fails, release blocked, roll back |
| **Soft** | Affected tests skip, release proceeds, reported `SKIPPED-OUTAGE` |

**A test that did not run never reports as passed.** Where a failure can be made to happen in Stage 1 or Stage 2 instead of STG, move it there — **CI is the martyr environment**.

## Express release

A designated build skips or strips stages and reaches PROD automatically. For patch-level and vulnerability releases where the full path costs more than it proves.

| Stage | Express behaviour |
|---|---|
| **1 · DEV** | Unchanged |
| **2 · CI** | **Full run, never stripped.** It carries the confidence the skipped stages would have provided |
| **3 · STG** | **Smoke only.** Functional smoke, not just health. No integration, no e2e, no perf |
| **4 · PROD** | **Automatically approved.** No human gate |

**e2e is too heavy for this path** — Stage 2 already covers the behaviour, and e2e's value is exercising real cross-service data, which a patch release rarely changes.

Eligibility is declared in the build config and opted into per build. **Never let the opt-in be the only control** — pair it with an automated check that the change is genuinely small (patch or minor bump, dependency-only diff). A designation alone means one mistaken label ships an untested major change straight to PROD.

## Scheduled and on-demand

Nothing here rides a merge. Each is triggered on its own.

| Suite | Trigger |
|---|---|
| **Performance** | Cron, Sunday night · a `perf` label opts one prerelease in · manual dispatch |
| **Soak / memory leak** | `soak` label · manual dispatch. Rare by design |
| **Chaos** | Cron · manual dispatch |
| **Vulnerability / base-image release** | Scan result, via the express path |

**A label is how a build opts into an expensive suite** — visible in review, no code change, and it leaves a record on the merge. The same mechanism designates an express release.

## Stage 4 — PROD

- **Regular releases require explicit human approval.** A green prerelease does not promote itself.
- **The same artifact is promoted.** No rebuild, no code difference, no contract difference between STG and PROD.
- **Blue/green, same as STG** — deploy inactive, smoke, flip.
- **Express releases are the one exception**, auto-approved by the rule above.

## Rollback

**STG and PROD both roll back automatically and flip back to the last stable build.** No holding a broken stack live to debug it — investigate from the rolled-back state.

A smoke failure before the flip needs no rollback at all: the flip simply never happens, and the last stable stack was serving the whole time. That is the point of blue/green.

**Health probes keep watching after the flip, not just before it.** A failed assertion during that extended window triggers the same automatic rollback, via the build CLI, as a pre-flip smoke failure.

## Feature flags

**Canary and A/B are not deploy strategies.** They are the flag service acting on an already-flipped release:

- **Canary** — a flag drives a slowed percentage rollout. Advancing the percentage is a flag operation, not a deploy.
- **A/B** — scoped toggles matched on request data (user, region, cohort).

**A flag never changes the contract or the code between STG and PROD.** A toggle requiring a rebuild is not a toggle.

**Flag scoping and request-matching rules must resolve identically in both environments.** A flag that matches a request differently in STG than in PROD is a contract break wearing a rollout costume — verify it the same way any other contract is verified, before the pre-release gate, not after.

## Build config

Each repo carries a build config declaring its pipeline; the CI/CD service consumes it and executes the stages. What belongs in it:

| Declares | Examples |
|---|---|
| Which stages run and in what order | Skip perf, add a chaos stage |
| Per-stage test selection | Which tiers, which suites, parallel or serial |
| Express eligibility and its automated conditions | Patch-only, dependency-only |
| Dependency classification | Which dependencies are hard, which are soft |
| Rollout shape | Canary percentages and advance mode |
| Environments and targets | Endpoints per stage, secret prefixes |

**The config selects behaviour; it does not define new stages.** A repo that needs a stage the model doesn't have is a reason to change this doctrine, not to special-case one pipeline.

## Parallel and serial

**Parallel wherever possible.** Serial only where a test genuinely owns a shared resource — and that requirement is declared by the test itself, never inferred from run order, file order, or a single-worker flag.
