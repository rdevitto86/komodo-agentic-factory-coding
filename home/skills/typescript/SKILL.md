---
name: typescript
description: TypeScript and JavaScript standards — strictness, boundaries, async, test tiers. Load before reading or writing any .ts, .tsx, .js, .jsx, .mjs, package.json, or tsconfig.json.
user-invocable: false
---

# TypeScript

Zero comments, zero JSDoc. Error messages lead with a verb phrase and never name the function.

## Conventions

- **`"strict": true` everywhere.**
- **`unknown` over `any`** for external data. Validate at boundaries with Zod — a type is not a runtime guarantee.
- **`??` for defaults, not `||`** — `||` swallows `0` and `''`.
- **No non-null `!`.** If you cannot prove it is non-null, narrow it.
- **`async`/`await` over promise chains.** `Promise.all` for independent work; never `await` inside a loop that could run in parallel.
- **Throw typed errors.** Do not return `null` to signal failure.
- **One primary export per module.** No circular imports. Barrel `index.ts` sparingly — it hurts tree-shaking.
- **Naming**: PascalCase types and components, camelCase variables and functions, SCREAMING_SNAKE constants, kebab-case module files, PascalCase component files. Booleans take `is` / `has` / `can` / `should`. No `I` prefix on interfaces. `req` / `res` for request and response.
- **Prefer a closure over a new class or extra parameter** when it captures scope the caller already has — an event handler, a memoized selector, a factory returning configured functions. Skip it inside a render loop or a hot path: a closure allocated per call/render defeats memoization (`useCallback`/`useMemo`, referential equality checks) and adds GC pressure. Measure before choosing a closure over a plain function in code a profiler already flags.

## Component rules

- **No business logic in components.** Use a service or store layer.
- **Derive state rather than syncing it.** Pass minimal props.
- **Effects need correct dependencies and a cleanup.**
- **Utility classes only** — no inline styles, no `!important`.

## Testing

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

**Suites, helpers, and descriptions** follow the `sdlc` skill. The TS mechanics:

- **One `describe` per module**, `it` cases flat inside it with scenario titles. Never a `describe` inside a `describe`.
- **In-file helpers sit at the bottom**, not exported, under the one permitted banner:

```ts
// --- Helpers ----------------------------------------------------
```

- **Shared fixtures get `helpers.ts` or `fixtures.ts`** in the tier folder; anything global goes to `@/test/helpers`.
- **Parallel is preferred for unit, component, and contract — never mandatory.** Vitest parallelises files but not cases within a file, so opting in means `sequence.concurrent: true` or `describe.concurrent` on the suite. Enable it per suite, not repo-wide: shared module state breaks intermittently, not loudly.
- **Serial for smoke, integration, e2e, and chaos.** They hit deployed infrastructure, so set `fileParallelism: false` in the Vitest tier config and leave Playwright at `fullyParallel: false` with `workers: 1`.
- **A description above a case is optional** — one or two lines, corner-case context only.

**Hooks, pools, and cost** — the `sdlc` skill owns the policy; these are the Vitest and Playwright mechanics.

- **Prefer `const ctx = await createTestContext({...})` in the case over `beforeEach`.** A per-case hook runs once per `it`, so a 10ms mock reset across 1,000 cases is 10 seconds of dead time — and it fires for cases that never touch what it built.
- **`beforeAll` for pools, mock servers, and base config**, and treat everything it returns as frozen. A case that needs a mutated fixture does `structuredClone(base)` locally.
- **`pool: 'forks'` when the suite writes `process.env` or a module-level singleton.** The default `threads` pool shares one process, so `vi.stubEnv` in one file leaks into another. Forks isolate at the cost of per-worker startup — pick per tier, not repo-wide.
- **`vi.waitFor` or `expect.poll`, never `await setTimeout`.** A fixed delay is slower than needed and flaky under CI load; both helpers return the moment the condition holds.
- **Cheapen the test config, not the code** — bcrypt rounds at the minimum, `fetch`/client timeouts at 50–100ms, and the logger pointed at a no-op sink, all injected rather than branched on `NODE_ENV` inside the module under test.

**Runner configuration** — Vitest config in `vite.config.ts`, with a glob covering both trees:

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

**Tier definitions, merge/release gates, and coverage floors are owned by the `sdlc` skill** — this section covers TS/JS mechanics only.
