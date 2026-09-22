---
name: standards-sdlc
description: Test tiers: what unit, integration, and end-to-end each prove and cost.
globs: ["**/__tests__/**", "**/e2e/**", "**/test/**", "**/tests/**"]
---

# SDLC: test tiers

The language standard owns tooling and folder mechanics. The CI/CD standard owns when each tier runs and what a failure does. This file owns what each tier is for.

## Targets
- `-mock` is the default: a local stand-in for every dependency.
- `-live` is explicit and points at STG with real STG credentials loaded the way the service loads them.
- PROD is never a test target, enforced in test setup.

## Tiers
- **Unit**: mandatory everywhere. Pure logic, hermetic: no network, disk, clock, or container. Blocks the merge.
- **Component**: interconnected segments with ephemeral infrastructure the run creates. Discretionary. Blocks the merge when present.
- **Contract**: consumer-driven pacts, file-based, both sides verified in one run, no broker. Blocks the merge.
- **Smoke**: extended health checks on the deployed endpoints. Fast, serial, seconds. Decides the blue/green flip.
- **Integration**: real or mocked endpoints in limited scope. Depth is the merge gate's job.
- **e2e**: happy paths only over real endpoints and real data in STG. The STG run is the authority.
- **Performance**: benchmarks, throughput, soak. Scheduled or by label, never on merge.
- **Chaos**: fault injection and mutation testing. Scheduled or on demand.

## Layout
- Unit tests colocate beside the code they cover, in every language.
- Every other tier lives under one top-level `test/` (or the toolchain's own root), one optional folder per tier, flat by feature. Never mirror the source tree.
- A tier may fold back into unit files when the language demands unexported access.

## Helpers and setup
- Used by one file: bottom of that file under one `--- Helpers ---` banner, the only banner permitted. One package: a sibling file. Everywhere: a central test package.
- Prefer an explicit helper call in the test body over a lifecycle hook. Where hooks are the idiom, per-case hooks only reset mutated state; heavy immutable setup runs once and is read-only; allocate lazily.

## Descriptions
- Optional and rare. One or two lines above the declaration, only for context the name cannot hold. Never a restatement of the name.

## Coverage
- Target 100% unit coverage. Any new code has an 85% hard floor; SDKs, shared libraries, and security-critical code require 100%.
- Coverage is a floor. A number hit by asserting nothing fails review.

## Rules
- Nothing that needs a deployed service gates a merge. Depth lives at the gate; every STG tier is narrow.
- A lower tier never depends on a higher one. Unit tests pass with no infrastructure present.
- A test that did not run never reports as passed.
