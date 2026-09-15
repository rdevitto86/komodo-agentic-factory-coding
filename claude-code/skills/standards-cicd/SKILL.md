---
name: standards-cicd
description: Pipeline stages, merge vs release gates, ephemeral CI infra, blue/green, rollback, feature flags.
user-invocable: false
paths: "cicd.yaml, .github/workflows/**, **/Dockerfile*, **/docker-compose*, **/buildspec.yml, **/.gitlab-ci.yml, **/Jenkinsfile, **/.circleci/**, **/Makefile, **/Taskfile*, **/*.pipeline.yml"
---

# CI/CD

Tier definitions and coverage floors are owned by the `standards-sdlc` skill. This file owns **which stage runs what**, and **what happens on failure**.

Platform-neutral. Every repo declares its pipeline in a build config; a CI/CD service reads that config and executes the stages. The stage model below is fixed — the config selects and parameterises stages, it does not invent them.

## The stages

| Stage | Where | Runs | Gates |
|---|---|---|---|
| **1 · DEV** | The developer's machine | unit, component, contract, smoke, integration | **The commit and the push**, via local hooks |
| **2 · CI** | Ephemeral runner, per PR | unit, component, contract — full, plus the security scans | The merge |
| **3 · STG** | Deployed, post-merge | smoke → integration → e2e → perf (flagged) | The release |
| **4 · PROD** | Deployed, on approval | smoke | Nothing after it |

## Stage 1 — DEV

**Nothing runs on edit or save. Ever.** No watchers, no save hooks, no background test runners. The developer decides when tests run.

The point of Stage 1 is that a developer can run **STG-scoped tests before the PR exists**. Finding it locally costs one command; finding it post-merge costs a build failure and a rollback.

### Targets

**`standards-sdlc` owns the target rules** — `-mock` default, `-live` opt-in, PROD never a local target, e2e's local-vs-STG authority, and the local run being the same code as the STG run. Read them there; they are not restated here.

### Local hooks — the first gates

Stage 1 gates its own output. These are the only gates before a remote runner sees the code, and they are the reason Stage 2 rarely fails on formatting or an obvious regression.

| Hook | Gates | Runs | Mandatory |
|---|---|---|---|
| **pre-commit** | The commit | Language formatter and linter on staged files, plus a secret scan of the staged diff. **Never tests** | Yes |
| **pre-push** | The push | Unit tests + coverage, delta-scoped to the changed files | No |

**Delta scope is whatever unit the language's coverage tool reports on** — the package, module, or file set containing the changes. Take the tightest scope the toolchain supports; the language skill names the tool.

**Both are local-only.** No remote container, no runner, no network. A hook that needs infrastructure belongs in Stage 2.

**The hooks ship with the language SDK, not with a repo and not with agent config.** A repo installs from the SDK and points `core.hooksPath` at it; never hand-write a per-repo copy, and never reimplement one because the SDK's is inconvenient. **This skill states what the hook must do; the language skill names the formatter, linter, and test command; the SDK decides how.**

## Stage 2 — CI

**The most important stage.** It is where defects are supposed to be caught, and the only stage that gates the merge.

- **Ephemeral and disposable.** A container per run, torn down when it ends, whatever the result.
- **Hermetic.** Stage 2 never reaches STG. A STG outage must not block every open PR, and PR runners must not hold production-adjacent credentials.
- **LocalStack stands in for secret fetching** and any AWS-shaped dependency, so the run needs no real credentials at all.
- **Runs unit, component, and contract in full.** Contract verifies both sides in-process against file-based pacts — no broker.

### Security gates

Three scans, all blocking on the merge. Thresholds are stated here so the gate is enforceable without loading anything else.

| Scan | Blocks on |
|---|---|
| **Secret** | Any verified live credential — including one only reachable in the branch's history |
| **Dependency** | A High or Critical advisory with no recorded exception |
| **Static analysis** | A High or Critical rule, on changed files only |

- **The command belongs to the language skill** — it names the vulnerability scanner and the linter's security rule set. This file owns only which stage runs it and what fails the merge.
- **Path filters never skip the security scans.** A docs-only PR skips the test pipeline; a lockfile or workflow change never does.
- **Draft PRs run the secret scan.** The other two wait for ready-for-review with the rest of the stage.
- **Scan the built image too**, not just the manifest — base-layer CVEs do not appear in a dependency lockfile.
- **An exception's shape is owned by `standards-api-security`** — a committed record with an owner, an expiry, and a compensating control. An expired exception fails the gate again; a permanent suppression is not an exception.
- **A finding blocks the merge without needing triage.** Load the `standards-api-security` skill (or `standards-web-ui` for a rendered-surface finding) only when deciding whether a finding is genuinely exploitable or an exception is justified — never to run the gate.

### Runner rules

- **A repo that claims more than one OS runs Stage 2 on a matrix covering every one of them**, sourced from the platform list its README or install doc states — a claim no runner exercises is a claim, not a fact. One OS is the baseline and blocks the merge; the others block too unless the config marks a specific leg as informational, with a recorded reason and an owner, the same shape a security exception takes.
- **Keep the matrix cheap by splitting what varies from what doesn't.** Lint, the security scans, and anything reading only file contents run once on the baseline OS. Only the legs where the platform can actually change the answer fan out: path handling, line endings, process and shell invocation, file permissions and symlinks, and the test suites that touch them.
- **Every stage's entry point must be one command that resolves the same way on each leg.** A matrix whose steps branch on `runs-on` is testing the branch, not the code; where a genuine per-OS step is unavoidable, put the branch inside the build CLI so the pipeline config keeps one invocation. A leg that needs a shell the platform lacks, or an executable bit the checkout does not carry, is the defect the matrix exists to find.
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

**`standards-sdlc` defines what each tier is and how it runs.** What belongs here is only what the *pipeline* does with it:

| Sub-stage | Its role in the release |
|---|---|
| **smoke** | Triggers the hard/soft dependency-backout decision (tier rule: `standards-sdlc`), and starts the inactive→active flip |
| **integration** | Limited scope — exhaustive coverage was Stage 2's job |
| **e2e** | Gates the prerelease |
| **perf** | Runs only when the build config or the PR flags it on. Never automatic |

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

### Approval gate

**The SCM host holds the pause, not a custom engine.** The build's source-control host pauses the run using its own native reviewer-gate feature — GitHub's required reviewers today, whatever the equivalent is on another host tomorrow. A paused job burns no compute and costs nothing to leave waiting, so nothing purpose-built is needed to hold it open. This repo's own job is narrower: notify the right people, and make sure the wait can't go on forever.

| When | What fires |
|---|---|
| PROD-ready, gate opens | `deployment.awaiting_approval` — approvers notified by email and Slack |
| 18h with no decision (default) | `deployment.approval_expired` — the run is cancelled at the SCM host, approvers notified it auto-failed |

- **18 hours is the default timeout**, declared in the build config and overridable per repo — long enough for a normal approval cycle, short enough that a release never sits forgotten. No mid-window reminder: one notification when the gate opens, one if it expires.
- **Expiry is final.** Once cancelled, a late approval is a no-op — the run no longer exists to promote. Shipping after expiry means re-triggering the release from STG, not resuming the old one. This avoids a race between a last-second click and the cancel job.
- **A crash between approval and timer-cancellation is harmless.** The expiry handler checks the run's actual status before acting — cancelling an already-finished run is a no-op, not a bug.

### STG-only releases

A build-config flag marks a release as STG-only. Stage 4 never starts for that run — no gate, no timers, no notifications. This isn't "reach the gate and auto-skip it"; there is no PROD-bound release to approve, so nothing about Stage 4 fires at all. Use this for changes that are deliberately staging-scoped, so they never sit blocking on an approval nobody intends to give.

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

**Each repo carries the build config at repo-root `cicd.yaml`.** One canonical path, no fallback lookup — the CI/CD service, the build CLI, and any agent all resolve it the same way. It declares the pipeline; the CI/CD service consumes it and executes the stages. What belongs in it:

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
