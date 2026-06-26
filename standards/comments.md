# Comments

**Scope: every file, every language, every agent, no exceptions.** Zero comments. Not on functions, methods, types, structs, interfaces, fields, vars, consts, packages, files, helpers, tests — nothing. The code, types, and identifier names are the documentation. If they can't carry the meaning on their own, fix the name or the structure — don't wrap it in prose.

## The rule

Do not write a comment. Ever. This applies to every declaration and every line, with no "why", "public API", or "edge case" exception — those licenses are retired. If a behavior, constraint, or gotcha genuinely needs to survive for the next reader, it goes in `TODO.md`, a PR description, or a design doc — never inline in code.

## What is exempt (these are not "comments" — they're toolchain mechanics)

- **Machine directives** the compiler, linter, or codegen tool reads: `//go:*`, `//nolint`, `eslint-disable`/`eslint-enable`, `# noqa`, `@ts-expect-error`/`@ts-ignore`/`@ts-nocheck`, `prettier-ignore`, SPDX headers, `# type:`/`# pragma`, shebangs, codegen markers (`// Code generated ... DO NOT EDIT`).
- **Test-file section banners** in the format below — structural dividers, not prose.
- **Comments the user explicitly asks for** in that turn.
- **Pre-existing human-written comments** you didn't touch — don't go on a deletion spree on adjacent code. Mechanical enforcement only checks lines you add.

## Hard violations (reject on sight, in any file)

- Any new function/method/class doc — for any reason, including "the why," public API notes, or edge-case notes.
- Any new comment on a type, struct, interface, enum, field, var, or const.
- Any new file/package/module header.
- Any new inline comment inside a function body.
- Any new trailing comment on any line.
- Any new doc comment on a test function.
- Change history, author tags, dates, ownerless TODOs, commented-out code, jokes/apologies/editorial.
- **Treating surrounding code as license.** A comment elsewhere in the file, the diff, or the conversation is never justification — judge every new comment against this file alone.

If you find yourself reaching for a comment, that's a signal the name or the structure is wrong. Rename, extract, or restructure instead.

## Test-file section banner

Structural dividers only — a short section label, never prose. Form: `// ── <Label> ── …`. `<Label>` names a section: a test tier (`Unit Tests`, `Component Tests`, `Integration Tests`), or a support section (`Setup`, `Helpers`, `Fake: <Type>`), optionally qualified by the subject under test. It labels a section; it must not explain code or carry a "why".

```
// ── Unit Tests ──────────────────────────────────────────────────────────
// ── Component Tests: OAuthTokenHandler ───────────────────────────────────
// ── Integration Tests ───────────────────────────────────────────────────
// ── Setup ────────────────────────────────────────────────────────────────
// ── Helpers ─────────────────────────────────────────────────────────────
// ── Fake: CacheClientCallers ─────────────────────────────────────────────
```
