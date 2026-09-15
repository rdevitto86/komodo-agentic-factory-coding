---
name: standards-typescript
description: TypeScript and JavaScript standards — strictness, boundaries, async, test tiers. Load before reading or writing any .ts, .tsx, .js, .jsx, .mjs, package.json, or tsconfig.json.
user-invocable: false
paths: "**/*.ts, **/*.tsx, **/*.js, **/*.jsx, **/*.mjs, **/package.json, **/tsconfig.json"
---

# TypeScript

Zero comments, zero JSDoc. Error messages lead with a verb phrase and never name the function.

## Comment discipline

The real rules live in `claude-code/hooks/comments.py` and its shared rules module `claude-code/hooks/lib/comment_rules.py`, not here. `comments.py check` reports what is missing or malformed; comments are only ever added via the `write-comments` skill.

## Toolchain

- **Versions come from `package.json`.** Read it; never assume a major.
- **Vulnerability scanning** — `npm audit --omit=dev --audit-level=high` (or the pnpm/yarn equivalent) is the gate. Install with `npm ci` so the lockfile is honoured. Enable `eslint-plugin-security` for the static half. `standards-cicd` defines the gate; the `standards-api-security` skill states the bar.
- **Outdated dependencies** — `npm outdated` (or the pnpm/yarn equivalent) lists packages behind their declared range, no CVE required to surface. Advisory only, never a merge gate; bump one package at a time and re-run the test suite, rather than a blanket `npm update`.
- **Internal dependency graph** — the toolchain ships no graph command. The closest answer needing no install is `tsc --noEmit --explainFiles`, which lists every file the program pulls in and names what imported each one; that is file-level edges for a single `tsconfig.json`'s program, not a module- or symbol-level graph, and it says nothing about a file no entry point reaches. `npm ls` covers external packages only, never internal edges. Do not reach for a third-party graph tool. `assess-change-risk` states when to run it and what its output does to the tier.
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
- **Depend on a caller-defined interface or type, not a concrete class** — the Dependency Inversion Principle applied to TypeScript. A consumer declares the shape it needs (an `interface` with one or two members) and a concrete implementation is passed in; the implementation depends on the interface's shape, never the reverse. Never publish a broad interface beside its only implementation.
- **Wrap a long statement by collapsing one level at a time, never straight to one-arg-per-line.** Try the whole statement on one line first; if it doesn't fit, move the wrapped content to its own indented line(s) with a trailing comma and the closing paren/brace/bracket on its own line at the original indent — a function call may stay grouped at this step if it fits, but an object-literal's properties never do, always one property per line once wrapped. Only if that grouped form is still too long, break to one item per line. Apply identically to calls, object literals, destructured parameter lists, and multi-arg logger calls.
- **A numeric or short repeated string literal standing for a size, TTL, count, threshold, or spec-level value (a header name, a status string) gets extracted to a named constant.** Module-level `const` when shared by more than one function in the file; a function-local `const` when scoped to one call. A literal repeated across 2+ files or functions is the strongest signal to extract first. Two or more constants introduced together go in one grouped declaration or object, not scattered standalone `const` statements.
- **No blank line between two consecutive early-return guard clauses.** Exactly one blank line between the last guard clause in a sequence and the happy-path logic that follows it.

## Security standards

Language-specific insecure-usage patterns for `/assess-security` to pull from, beyond `standards-api-security`'s generic OWASP checklist — `standards-web-ui`'s Security section owns the broader rendered-surface bar this narrows to TypeScript/JavaScript mechanics.

- **`eval`/`new Function(...)` on any request- or user-derived string is code execution**, not a shortcut — no upstream validation makes it safe.
- **`dangerouslySetInnerHTML`/`innerHTML`/`v-html` with unsanitized content is stored/reflected XSS** — run untrusted HTML through a sanitizer (e.g. DOMPurify) first, or avoid the raw-HTML sink entirely.
- **`child_process.exec` with an interpolated string is command injection; `execFile`/`spawn` with an argument array is not.**
- **A recursive merge/`Object.assign` over untrusted JSON is a prototype-pollution vector** if a `__proto__`/`constructor`/`prototype` key reaches it unguarded — reject or strip those keys before merging.
- **A parameterized query builder or ORM binding, never a template literal building SQL/NoSQL query text** from request input.
- **`crypto.randomBytes`/`crypto.getRandomValues`, never `Math.random()`, for a token, key, or session ID.**
- **A JWT verify call must pin the expected algorithm** — accepting `alg: none` or letting the token's own header pick the algorithm lets an attacker forge a signature-free token.

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

This skill carries no `Repo layout — <token>` section. **Create is unsupported for TypeScript** in `git-repo-init` — no repo type token maps here. Use `git-repo-init`'s Scaffold path (doc-pair-only) instead, or add a `Repo layout` section here first.

## Reference material

- **[testing.md](testing.md)** — approved stack, placement, suites, parallelism, Vitest and Playwright configuration.
