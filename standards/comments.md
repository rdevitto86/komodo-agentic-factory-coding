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

Reserved for exactly two structural sections, never prose. Form: `// ── <Label> ── …`, and `<Label>` is `Setup` or `Helpers` — nothing else gets the heavy banner treatment. Test tiers (unit, component, integration, e2e, perf, chaos) live in separate files/folders (a top-level `test/`/`tests/` tree, one subfolder per tier — see `~/.claude/modes/go/coding.md` §5.2 and `~/.claude/modes/ts/coding.md` §7.2) or, for unit tests, colocated with the source; either way a single file holds one tier, so banners are never used to separate tiers within a file.

```
// ── Setup ────────────────────────────────────────────────────────────────
// ── Helpers ─────────────────────────────────────────────────────────────
```

Any other section label — naming a fake/mock block, grouping a cluster of tests by subject — is a plain single-line comment, not a banner: `// Fake: CacheClientCallers` or `// OAuthTokenHandler`, no box-drawing. Use one only when the folder or file name doesn't already make the grouping obvious; don't over-index on labeling sections that are already implied by where the file lives.

**Placement of helpers/setup.** Keep helper functions and setup/fixture code in the test file that uses them — don't split them into a separate file just to hold them. One `Setup`/`Helpers` section near the top (or bottom) of the file holds everything shared across tests in that file. Only break helpers/setup into their own file (a `_test_helpers`/`suite` file) when the volume genuinely warrants it — many shared fixtures reused across multiple test files in the package — not as a default.
