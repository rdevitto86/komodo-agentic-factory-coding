---
name: standards-typescript
description: TypeScript and JavaScript standards — strictness, boundaries, async, test tiers. Load before reading or writing any .ts, .tsx, .js, .jsx, .mjs, package.json, or tsconfig.json.
user-invocable: false
paths: "**/*.ts, **/*.tsx, **/*.js, **/*.jsx, **/*.mjs, **/package.json, **/tsconfig.json"
---

# TypeScript

Zero comments, zero JSDoc. Error messages lead with a verb phrase and never name the function.

## Comment discipline

`rules-commenting` carries the shared template contract. This language's exempt machine directives, verified against the guard's own list: `// eslint-disable`, `// eslint-enable`, `// @ts-expect-error`, `// @ts-ignore`, `// @ts-nocheck`, `// prettier-ignore`, `// biome-ignore`, `// istanbul ignore`, `"use client"`, `"use server"`. Anything else — including JSDoc on an exported symbol — prompts for approval.

## Toolchain

- **Versions come from `package.json`.** Read it; never assume a major.
- **Vulnerability scanning** — `npm audit --omit=dev --audit-level=high` (or the pnpm/yarn equivalent) is the gate. Install with `npm ci` so the lockfile is honoured. Enable `eslint-plugin-security` for the static half. `standards-cicd` defines the gate; the `standards-security` skill states the bar.
- **Forge SDK** — published as `@komodo-forge-sdk/typescript`, including the shared CDK construct toolkit. Import the published package at a pinned version; never a relative path into a local checkout. Read its exports before concluding it lacks something.

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

## Testing

**Unit tests colocate as `.x.test.ts`; every other tier lives under a top-level `test/`.** The stack, folder scheme, suite structure, and runner config are in [testing.md](testing.md). Tier definitions, gates, and coverage floors are owned by the `standards-sdlc` skill.

## Quick-reference fields

The field set a TypeScript repo's `AGENTS.md` Quick-reference table carries. Every value is read from the repo, never assumed. A UI repo uses `standards-svelte` or `standards-vue` instead; a CDK repo uses `standards-cdk`.

| Field | Source on disk |
|---|---|
| Runtime + floor | `package.json` engines |
| Package name | `package.json` |
| Entrypoint | `main` / `exports` / `bin` |
| Build + test | `package.json` scripts |
| Path alias | `tsconfig.json` paths |

Drop a row whose value the repo genuinely lacks. Never add a row for a fact this skill or `standards-sdlc` already states by name.

## Repo layout

This skill carries no `Repo layout — <token>` section. **Create is unsupported for TypeScript** in `generate-repo` — no repo type token maps here. Use `generate-repo`'s Scaffold path (doc-pair-only) instead, or add a `Repo layout` section here first.

## Reference material

- **[testing.md](testing.md)** — approved stack, placement, suites, parallelism, Vitest and Playwright configuration.
