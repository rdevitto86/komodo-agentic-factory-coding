---
name: standards-comments
description: What earns a comment, what a comment says, and what is banned from one.
globs: []
roles: [builder, reviewer]
---

# Comments

## What earns a comment
- Every exported or public function, type, and module: one line saying what it does or returns, in the language's doc convention (godoc, docstring, JSDoc).
- A private function whose body runs longer than a screen or whose behaviour is not obvious from its name.
- A value the line cannot explain: a unit, a bound, an ordering requirement, the shape of a workaround.
- Nothing else. A statement, a branch, a loop, or a field is silent by default.

## Shape
- One line above a statement, two at most above a function. Never a paragraph, never a stacked pair of blocks.
- At most twenty words. What the code does, as it stands.
- Directives (`//go:generate`, `# noqa`, `// eslint-disable`) are not comments and do not count.
- Markers read `TODO: text`, `FIXME: text`, `NOTE: text`.

## Banned
- Restating the identifier below: `// increments counter` above `counter++`.
- A version, ticket, spec, PRD, or "as discussed". The reader has the code, not your conversation.
- First person and hedges: "we", "I think", "should", "probably".
- History: "previously", "now uses", "was changed", "refactored from".
- Reasoning about callers: "every call site passes a non-empty prefix".
- Hypotheticals about states the code does not reach.

## Language notes
- Go: exported identifiers get a godoc line starting with the name. Unexported ones follow the private rule.
- Python: docstrings, not `#` lines, for functions and classes. Module docstring on every module.
- TypeScript: JSDoc `/** */` on exports; `//` inside bodies.
- Shell: a header block under the shebang; almost nothing below it.
