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
├── integration/
└── e2e/
```

- **Flat by feature inside each tier folder.** Do not mirror the source tree.
- **Files under `test/` import via the path alias** (`$lib` for SvelteKit, `@/` for Vue) — relative paths break once a test moves out of colocation.
- **Playwright e2e lives in `test/e2e/`**, never a bare top-level `e2e/`.

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

**Scope note.** TypeScript at this layer has no evidence-backed use for performance or chaos tiers. Add `test/perf/` only when a real need appears — do not scaffold it speculatively.

**Tier definitions, merge/release gates, and coverage floors are owned by the `sdlc` skill** — this section covers TS/JS mechanics only.
