# Testing Strategy

Cross-cutting testing standard for all Komodo services. Defines the six test tiers, what each one is for, which environment runs it, and which pipeline stage it blocks. Language-specific mechanics — frameworks, file placement, gate helpers — live in the language modes (`~/.claude/modes/go/coding.md` §5, `~/.claude/modes/ts/coding.md` §7) and must not contradict this file.

---

## 1. The two layers

Tiers group into two layers by what they need to run:

| Layer | Tiers | Needs | Runs in |
|---|---|---|---|
| **DEV** (aka local) | unit, component, integration | Nothing live — mocks and local containers only | Local dev loop, and the build stage of both DEV and STG builds |
| **STG** | live-dependency, performance, chaos | Real deployed apps, services, and data | STG pipeline; live-dependency also runs in QA and UAT |

**The DEV layer is the merge gate.** Unit, component, and integration must all pass before CI/CD proceeds and before a PR merges to main. The STG layer runs later in the pipeline and never gates a merge.

---

## 2. Tier definitions

### 2.1 Unit

- **Mandatory for all code.** The foundation of the pyramid — the widest tier, and the one every other tier assumes is already green.
- Pure logic, fully hermetic. No network, no disk, no clock, no container.
- Runs in the DEV (local) environment.
- **Blocks the PR merge to main.**

### 2.2 Component

- Advanced unit tests: slightly larger, interconnected segments of code — a step more abstract than a unit test.
- The barrier tier between unit and integration.
- **Optional and discretionary.** Add them where they earn their keep; there is no requirement to produce them for every change. Follow standard enterprise component-testing practice when you do.
- Runs locally alongside unit and integration.
- **Blocks the PR merge to main** when present.

### 2.3 Integration

- The last locally-run tier. Local automation tests covering the more abstract modules of an app or service.
- **Mocked or locally ephemeral — never live.** External dependencies are mocked, or stood up as throwaway local containers (testcontainers-style: a real Postgres/Redis engine, created and destroyed by the test run). Both are DEV-layer; neither touches a deployed service. These are explicitly not live-dependency tests and not end-to-end.
- Never run alone: they run alongside unit and/or component.
- Runs in the DEV (local) environment. Follow standard enterprise integration-testing practice.
- **Blocks the PR merge to main.**

### 2.4 Live dependency (e2e)

- Calls real deployed apps and services, and expects real data and real infrastructure.
- **Thin, happy-path only.** The question they answer is "is the functionality working and are the live resources actually up" — not exhaustive behavioral coverage, which belongs in the DEV layer.
- Runs in STG, QA, and/or UAT. Never runs locally.
- A later CI/CD stage; does not gate the merge.

> Identifier note: the tier is named **live dependency**, but its gate helper and folder keep the existing `e2e` spelling (`testutil.E2E(t)`, `test/e2e/`). The name describes intent; `e2e` stays the on-disk token so nothing has to be renamed.

### 2.5 Performance

- Benchmarks and throughput assessment across scenarios for an app or feature.
- **Exclusively STG**, in the STG CI/CD pipeline. Never a merge gate.

### 2.6 Chaos

- The niche tier, covering three things: mutation testing, advanced security and performance probing, and resilience fault injection (latency, jitter, connection drops).
- STG pipeline only. Never a merge gate.

---

## 3. Coverage requirements

| Scope | Requirement |
|---|---|
| New code, any service | **90% minimum**, 100% preferred |
| SDKs and common libraries (`komodo-forge-sdk-*`, shared libs) | **100%, required** |

Measured on unit tests, which are mandatory everywhere. Coverage is a floor, not a goal — high coverage with weak assertions is worse than an honest gap, and a number hit by asserting nothing fails review.

---

## 4. Pipeline mapping

| Stage | Tiers that run | What it blocks |
|---|---|---|
| Local dev loop | unit, component, integration | Nothing — fast feedback |
| PR build → main | unit + component + integration | **The merge.** All must pass |
| DEV / STG build stage | unit + component + integration | Build promotion |
| STG deploy (also QA, UAT) | live dependency | Promotion past the environment |
| STG pipeline | performance, chaos | Release |

---

## 5. Rules

- **The DEV layer never touches anything live.** If a test needs a deployed service or real data, it is a live-dependency test and belongs in STG — not in integration. A container the test run creates and destroys itself is not "live".
- **Integration never reaches a deployed dependency.** A test that calls a real running service has left the tier, regardless of what folder it sits in.
- **Live-dependency tests stay thin.** Depth belongs in the layer that runs on every PR; a slow, exhaustive STG suite delays every release and catches what unit tests should have caught.
- **Component is the only discretionary tier.** Unit is mandatory; integration, live-dependency, performance, and chaos each have a defined home and trigger.
- **Never let a lower tier depend on a higher one.** Unit tests must pass with no component, integration, or live infrastructure present.

---

## 6. Why this split

The merge gate is deliberately everything that can run without live infrastructure — fast, cheap, and free of cross-PR resource collisions. Anything needing real deployed dependencies moves behind the deploy, where an environment actually exists to support it. This keeps the PR loop quick and parallel-safe while still validating real infrastructure before release.
