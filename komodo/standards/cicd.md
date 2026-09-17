# CI/CD

Test tiers and coverage floors belong to the SDLC standard. This file owns which stage runs what and what happens on failure.

## Stages
| Stage | Where | Runs | Gates |
|---|---|---|---|
| DEV | developer machine | unit, component, contract, smoke, integration | the commit and the push, via local hooks |
| CI | ephemeral runner per PR | unit, component, contract in full, plus security scans | the merge |
| STG | deployed post-merge | smoke, integration, e2e, perf when flagged | the release |
| PROD | deployed on approval | smoke | nothing after it |

## DEV
- Nothing runs on save. The developer decides when tests run.
- pre-commit gates the commit: formatter, linter, and comment lint on staged files, secret scan of the staged diff. Never tests.
- pre-push gates the push: the repo's verify target.
- Both are local-only. A hook that needs infrastructure belongs in CI.

## CI
- Ephemeral, disposable, hermetic. Never reaches STG; holds no production-adjacent credentials. LocalStack or equivalents stand in for cloud dependencies.
- Runs unit, component, and contract in full. Contract verifies both sides in-process against file-based pacts.
- Security gates, all blocking: secret scan on any verified live credential including branch history; dependency scan on High or Critical with no recorded exception; static analysis on High or Critical rules on changed files.
- Path filters never skip the security scans. Draft PRs run the secret scan; the rest wait for ready-for-review.
- Scan the built image, not just the manifest.
- A repo claiming more than one OS runs CI on a matrix covering each. Lint and content-only checks run once on the baseline; only platform-sensitive legs fan out.
- Every stage's entry point is one command that resolves the same way on each leg. A step that branches on `runs-on` is testing the branch, not the code.
- Cancel superseded runs on the same ref; never on main. Cache dependencies and build output.
- One build CLI is the entrypoint for every stage; the repo's `cicd.yaml` parameterises it and never replaces its judgment. It emits a manifest: coverage, digests, artifact paths, and posts a summary to the PR.

## STG
- Deploy the inactive stack, smoke, flip, integration, e2e, perf when flagged, then prerelease. A smoke failure means no flip and no rollback needed.
- Seed before and tear down after, namespaced per run. The CI/CD service holds a per-service lock on STG-scoped stages.
- Hard dependency down: stage fails, release blocked. Soft dependency down: affected tests report `SKIPPED-OUTAGE` and the release proceeds. A test that did not run never reports as passed.

## Express release
- CI runs in full, never stripped. STG runs functional smoke only. PROD is auto-approved.
- Eligibility is declared in `cicd.yaml` and opted into per build, paired with an automated size check (patch or minor bump, dependency-only diff). A label alone never ships.

## Scheduled and on-demand
- Performance: weekly cron, `perf` label, or manual dispatch. Soak: `soak` label or manual. Chaos: cron or manual. Vulnerability and base-image releases go through the express path.

## PROD
- Regular releases need explicit human approval at the SCM host's native reviewer gate. The same artifact is promoted; no rebuild.
- Blue/green as in STG. The approval gate notifies on open and cancels after a configurable timeout (default 18h); expiry is final and a late approval is a no-op.
- A build-config flag marks a release STG-only; nothing in PROD fires for it.

## Rollback
- STG and PROD roll back automatically to the last stable build. Health probes keep watching after the flip and trigger the same rollback.

## Feature flags
- Canary and A/B are flag operations on an already-flipped release, never deploy strategies. A flag never changes the contract or the code between STG and PROD, and resolves identically in both.

## Build config
- `cicd.yaml` at the repo root declares which stages run, per-stage test selection, express eligibility, dependency classification, rollout shape, and environments. It selects behaviour; it does not define new stages.
- Parallel wherever possible. Serial only where a test declares it owns a shared resource.
