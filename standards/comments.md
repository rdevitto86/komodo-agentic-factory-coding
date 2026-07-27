# Comments

**Scope: every file, every language, every agent, no exceptions.** Zero comments. Not on functions, methods, types, structs, interfaces, fields, vars, consts, packages, files, helpers, tests — nothing. The code, types, and identifier names are the documentation. If they can't carry the meaning on their own, fix the name or the structure — don't wrap it in prose.

## The rule

Do not write a comment. Ever. This applies to every declaration and every line, with no "why", "public API", or "edge case" exception — those licenses are retired. If a behavior, constraint, or gotcha genuinely needs to survive for the next reader, it goes in `TODO.md`, a PR description, or a design doc — never inline in code.

## What is exempt (these are not "comments" — they're toolchain mechanics)

- **Machine directives** the compiler, linter, or codegen tool reads: `//go:*`, `//nolint`, `eslint-disable`/`eslint-enable`, `# noqa`, `@ts-expect-error`/`@ts-ignore`/`@ts-nocheck`, `prettier-ignore`, SPDX headers, `# type:`/`# pragma`, shebangs, codegen markers (`// Code generated ... DO NOT EDIT`).
- **Test-file section banners** in the format below — structural dividers, not prose.
- **Comments the user explicitly asks for** in that turn.
- **Pre-existing comments you didn't author** — never remove or alter one, even when you're editing that exact line for an unrelated reason (e.g. changing `1 << 20` but leaving `// 1MB` in place). Only the user removes their own comments. Mechanical enforcement only checks lines you add; this is a judgment rule you must apply yourself.

## Hard violations (reject on sight, in any file)

- Any new function/method/class doc — for any reason, including "the why," public API notes, or edge-case notes.
- Any new comment on a type, struct, interface, enum, field, var, or const.
- Any new file/package/module header.
- Any new inline comment inside a function body.
- Any new trailing comment on any line.
- Any new doc comment on a test function.
- Change history, author tags, dates, ownerless TODOs, commented-out code, jokes/apologies/editorial.
- **Treating surrounding code as license.** A comment elsewhere in the file, the diff, or the conversation is never justification — judge every new comment against this file alone.
- **Deleting a comment you didn't author** — including a trailing/inline comment on a line you're editing for other reasons. Zero-comments applies to what you add, never as grounds to strip what's already there.

If you find yourself reaching for a comment, that's a signal the name or the structure is wrong. Rename, extract, or restructure instead.

## Test-file section banner

Reserved for exactly two structural sections, never prose. Form: `// ── <Label> ── …`, and `<Label>` is `Setup` or `Helpers` — nothing else gets the heavy banner treatment. Test tiers (unit, component, integration, e2e, perf, chaos) live in separate files/folders (a top-level `test/`/`tests/` tree, one subfolder per tier — see `~/.claude/modes/go/coding.md` §5.2 and `~/.claude/modes/ts/coding.md` §7.2) or, for unit tests, colocated with the source; either way a single file holds one tier, so banners are never used to separate tiers within a file.

```
// ── Setup ────────────────────────────────────────────────────────────────
// ── Helpers ─────────────────────────────────────────────────────────────
```

`Setup` and `Helpers` are the only labels that get a comment of any kind. Any other grouping — a fake/mock block, a cluster of tests by subject — gets **no comment at all**, banner or plain: the rule at the top of this file has no test-file carve-out beyond these two banners, and the enforcement hook blocks anything else. If a grouping needs to be visible, express it in the identifier names or split it into its own file; the file's location and its test names carry the grouping.

**Placement of helpers/setup.** Keep helper functions and setup/fixture code in the test file that uses them — don't split them into a separate file just to hold them. One `Setup`/`Helpers` section near the top (or bottom) of the file holds everything shared across tests in that file. Only break helpers/setup into their own file (a `_test_helpers`/`suite` file) when the volume genuinely warrants it — many shared fixtures reused across multiple test files in the package — not as a default.
