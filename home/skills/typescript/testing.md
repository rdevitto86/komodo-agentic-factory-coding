# TypeScript testing

Full TS/JS testing standard — the approved stack, file placement, suite structure, runner configuration, and cost control. Go rules live in the `go` skill; Python rules in the `python` skill.

**This file owns mechanics only.** Tier definitions, merge and release gates, and coverage floors are owned by the `sdlc` skill. Which tier runs at which pipeline stage, and on what infrastructure, is owned by the `cicd` skill. Repo shape and service anatomy come from the framework skill in use (`svelte`, `vue`) or the repo's own `AGENTS.md`. Nothing here restates any of them.

## Approved testing stack

| Tier | Framework | Notes |
|---|---|---|
| Unit | Vitest | Native ESM and TS, shares the Vite config and aliases |
| Component | Vitest + `@testing-library/<framework>` | `@testing-library/svelte` or `/vue`; assert DOM and user events, never internals |
| Contract | The consumer-driven tool `sdlc` names | Pact ecosystem; the JS binding is the TS half of the same file-based exchange |
| Integration | Ephemeral instances, per `cicd` | Created and destroyed by the run; never a shared long-lived environment |
| Smoke | Vitest, serial | Extended health check proving a deployment is ready to test |
| E2E | Playwright | Thin happy-path flows against a deployed environment |
| Performance | Not chosen | Decide before the first suite; TS at this layer rarely earns the tier |
| Chaos | Not chosen | Same — the Go stack's choice does not transfer automatically |

**Notable rationale:**
- **Vitest over Jest** — Jest needs a parallel transform pipeline and its own module resolution; Vitest reuses the Vite config, so the aliases tests import through are the aliases the app builds with.
- **Playwright over Cypress** — real multi-browser support, first-class parallelism control, and no in-page execution model to work around.
- **Testing Library over shallow rendering** — asserting on rendered DOM survives a refactor of component internals; asserting on instances does not.
- **Two tiers deliberately left open** — recording a tool nobody has run is inventory, not a rule. Choose one when a real need appears, then record it here.

## Placement

**Only unit tests are colocated.** Component, integration, and e2e tests move to a top-level `test/` — the idiomatic root for TS, matching Go.

**Naming is `.x.test.ts`:**

| Source | Unit test |
|---|---|
| `order-service.ts` | `order-service.x.test.ts` |
| `UserCard.svelte` | `UserCard.x.test.ts` |
| `Button.vue` | `Button.x.test.ts` |

When several files form a cohesive unit (`src/lib/auth/`), place one unit test at the folder root, named after the folder.

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

- **Every subfolder is optional.** Create one when the tier has tests; never scaffold an empty tier.
- **Flat by feature inside each tier folder.** Do not mirror the source tree.
- **Files under `test/` import via the path alias** (`$lib` for SvelteKit, `@/` for Vue) — relative paths break once a test moves out of colocation.
- **Playwright e2e lives in `test/e2e/`**, never a bare top-level `e2e/`.

## Suites and helpers

**Suites, helpers, and descriptions** follow the `sdlc` skill. The TS mechanics:

- **One `describe` per module**, `it` cases flat inside it with scenario titles. Never a `describe` inside a `describe`.
- **In-file helpers sit at the bottom**, not exported, under the one permitted banner:

```ts
// --- Helpers ----------------------------------------------------
```

- **Shared fixtures get `helpers.ts` or `fixtures.ts`** in the tier folder; anything global goes to `@/test/helpers`.
- **A description above a case is optional** — one or two lines, corner-case context only.

## Parallelism

- **Parallel is preferred for unit, component, and contract — never mandatory.** Vitest parallelises files but not cases within a file, so opting in means `sequence.concurrent: true` or `describe.concurrent` on the suite. Enable it per suite, not repo-wide: shared module state breaks intermittently, not loudly.
- **Serial for smoke, integration, e2e, and chaos.** They hit deployed infrastructure, so set `fileParallelism: false` in the Vitest tier config and leave Playwright at `fullyParallel: false` with `workers: 1`.

## Hooks, pools, and cost

The `sdlc` skill owns the policy; these are the Vitest and Playwright mechanics.

- **Prefer `const ctx = await createTestContext({...})` in the case over `beforeEach`.** A per-case hook runs once per `it`, so a 10ms mock reset across 1,000 cases is 10 seconds of dead time — and it fires for cases that never touch what it built.
- **`beforeAll` for pools, mock servers, and base config**, and treat everything it returns as frozen. A case that needs a mutated fixture does `structuredClone(base)` locally.
- **`pool: 'forks'` when the suite writes `process.env` or a module-level singleton.** The default `threads` pool shares one process, so `vi.stubEnv` in one file leaks into another. Forks isolate at the cost of per-worker startup — pick per tier, not repo-wide.
- **`vi.waitFor` or `expect.poll`, never `await setTimeout`.** A fixed delay is slower than needed and flaky under CI load; both helpers return the moment the condition holds.
- **Cheapen the test config, not the code** — bcrypt rounds at the minimum, `fetch`/client timeouts at 50–100ms, and the logger pointed at a no-op sink, all injected rather than branched on `NODE_ENV` inside the module under test.

## Runner configuration

Vitest config in `vite.config.ts`, with a glob covering both trees:

```ts
test: {
  include: [
    'src/**/*.x.test.{ts,js}',
    'test/**/*.x.test.{ts,js}',
  ],
}
```

A separate `vitest.config.ts` must declare the alias explicitly; it only inherits Vite's `resolve.alias` when it shares the config file.

**Scope note.** TypeScript at this layer rarely earns a performance or chaos tier. The folders exist in the scheme; add one only when a real need appears.
