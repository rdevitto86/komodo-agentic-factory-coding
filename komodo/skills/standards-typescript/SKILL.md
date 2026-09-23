---
name: standards-typescript
description: TypeScript: strictness, type narrowing, module boundaries, and inference.
globs: ["**/*.cjs", "**/*.js", "**/*.jsx", "**/*.mjs", "**/*.svelte", "**/*.ts", "**/*.tsx", "**/*.vue"]
---

# TypeScript and JavaScript

Komodo's default for user interfaces, with Vue or Svelte on top. A suggestion, not a mandate: an existing codebase keeps its language.

## Comments
- A JSDoc line on every exported function, class, and type: one sentence saying what it does or returns.
- A non-exported function gets a one-line `//` comment when its body is longer than a screen or its behaviour is not obvious.
- Error messages lead with a verb phrase and never name the function.

## Conventions
- `"strict": true` everywhere. `unknown` over `any` for external data; validate at boundaries with a schema library. A type is not a runtime guarantee.
- `??` for defaults, never `||`. No non-null `!`; narrow instead.
- `async`/`await` over promise chains. `Promise.all` for independent work; never `await` inside a loop that could run in parallel.
- Throw typed errors; never return `null` to signal failure.
- One primary export per module. No circular imports. Barrel `index.ts` sparingly.
- PascalCase types and components, camelCase variables and functions, SCREAMING_SNAKE constants, kebab-case module files, PascalCase component files. Booleans take `is`, `has`, `can`, `should`. No `I` prefix on interfaces.
- Depend on a caller-defined interface with one or two members, not a concrete class. Never publish a broad interface beside its only implementation.
- Prefer a closure when it captures scope the caller already has; avoid one inside a render loop or hot path.
- Wrap a long statement one level at a time: content on its own indented lines with a trailing comma, closer at the original indent. Object-literal properties go one per line once wrapped.
- Extract a literal that stands for a size, TTL, count, threshold, header, or status into a named `const`. Module-level when shared, function-local otherwise.
- No blank line between consecutive early-return guards; one blank line before the happy path.

## Security
- `eval` and `new Function()` on request- or user-derived text is code execution.
- `dangerouslySetInnerHTML`, `innerHTML`, `v-html`, `{@html}` with unsanitized content is XSS. Sanitize first or avoid the sink.
- `child_process.exec` with an interpolated string is command injection; use `execFile` or `spawn` with an argument array.
- A recursive merge over untrusted JSON is prototype pollution unless `__proto__`, `constructor`, and `prototype` keys are rejected.
- Parameterized queries or ORM binding, never a template literal building query text.
- `crypto.randomBytes` or `crypto.getRandomValues`, never `Math.random()`, for a token or session ID.
- A JWT verify call pins the expected algorithm; never accept `alg: none`.

## Testing
- Unit tests colocate as `<name>.test.ts`; every other tier lives under a top-level `test/`, one folder per tier, flat by feature.
- One runner per repo, declared in the manifest. `describe` per unit, `it` per behaviour, names that read as sentences.
- Mock at module boundaries with the runner's own mocking, never internals. No shared mutable fixtures across files.
- Component tests render through the framework's testing library and assert on what a user sees, not on implementation details.
- No `sleep`; await the condition. No `.only` or `.skip` reaches a commit.
