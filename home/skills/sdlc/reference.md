# SDLC — execution reference

How and where the tiers run. `SKILL.md` covers writing a test; this file covers running suites and designing a pipeline's test stages.

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

## Suites

Scope comes from position in the tree, not from nesting inside the file.

- **Directory sets domain scope** — app, then feature.
- **File targets one module**, named for the unit under test.
- **Cases group flat, 0–1 levels deep.** One level of subtests under a suite is the ceiling; a subtest inside a subtest is never correct.
- **Titles state a scenario** — "rejects an expired card", not "test cancel error".

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

## Execution rules

- **Nothing runs on edit or save.** No watchers, no save hooks. The developer decides when tests run.
- **A tier that only runs in CI is broken.** Every tier a developer can usefully run has a local invocation, using the same code the pipeline uses.
- **Parallelism is preferred, never mandatory.** Prefer it for unit, component, and contract, where the only cost is owning your own fixtures. A suite that stays serial is not a defect.
- **Never add it in bulk.** Parallel tests resume together inside one process, so a package with mutable package-level state breaks the moment it is switched on — intermittently, not immediately. Add it per package, then prove it under a repeated race run.
- **Serial is the default for smoke, integration, e2e, and chaos.** These call real deployed infrastructure. Fanning them out loads the environment under test, so a timeout stops meaning "the code is slow" and starts meaning "the suite competed with itself".
- **Opting a deployed-tier suite into parallel is deliberate and per-suite.** It needs a per-run data namespace, an isolated dependency, and a stated reason. Absent all three, keep it serial.
- **Serial is declared, never accidental.** A test that must not run alongside others says so itself, never relying on run order or a single-worker flag.
- **Process isolation is what makes environment writes safe.** Thread- and goroutine-based runners share one process, so a case writing an env var corrupts its siblings. Choose the pool to match what the suite mutates.
- **Namespace every run's data.** Seeding, setup, and teardown are owned by the suite and scoped per run.

## Why this split

The merge gate is the seconds-scale, infrastructure-free set — fast, cheap, collision-free, and the stage where defects are meant to be caught. Everything deployed moves behind the merge, where an environment exists to support it and a failure rolls back instead of blocking every open PR.
