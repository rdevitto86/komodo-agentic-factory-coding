# Write Comments — reference

Mechanical detail for the comment pass. Read it when drafting a proposal type you have not used recently; the judgment rules live in `SKILL.md`.

## The nine template types

Each proposal is shaped exactly as `comments.py apply` expects:

```json
{"file": "path/relative/to/repo/root/or/absolute", "line": 42, "template_type": "WHY", "text": "// polling, not a webhook -- upstream has none for this event"}
```

`template_type` splits into two families:

- **Implicit — a plain sentence or clause, no marker.** `WHY` (what the code does, plus the standing fact that shapes it), `HACK` (the shape of a deliberate workaround), `DOC` (a godoc-style summary), `FIELD` (a trailing note on a struct field). These are facts about the code; a reader assumes any comment carries information, so no signal word is needed.
- **Explicit — a marker prefix, because the comment is a call to action, not a fact.** `NOTE: ...` (an invariant or callout), `FIXME: ...` (a known defect), `TODO: ...` (a deferred task — no `(username)`, that convention is retired). Plus two structural types: `BANNER` (`// --- Setup ---`, `*_test.go` files only, this exact label) and `STEP` (`1. ...`, inside an indented function body).

`text` is the full comment line, already marker-formatted (or deliberately marker-free, for the implicit family) for the target file's language family — the validator does not add syntax for you. `line` is 1-indexed. For every type except `FIELD` it names the position your new line takes in the resulting file, and the line currently there shifts down. `FIELD` is different: `line` names an *existing* line, and your text is appended to its end, not inserted above it.

## Mandatory sites

`check` emits `MISSING` for a site that owes a comment. `FUNC_UNDOCUMENTED` is the broad one — every function declaration, in every language the lint resolves a family for, public and private. It is suppressed by a line comment above the declaration, by a block comment's closing line above it (JSDoc), by a Python docstring on the first body line, and by four exemptions: a test path, a one-statement body, a bodyless declaration, and a generated file. `RET_ARITY_3` and `RET_BOOL_DISCRIMINANT` stay Go-only and outrank it — a function that is both reports the discriminant rule, because that one names what the comment has to say.

A test path is any `_test.*`, `test_*`, `*.test.*`, `*.spec.*`, `*Test.*`, `conftest.*`, or a file under a `test`/`tests`/`__tests__`/`spec`/`testdata` directory. A generated file is one whose first five lines carry `DO NOT EDIT` or `@generated`, one suffixed `.gen.go`/`.pb.go`/`_pb2.py`/`.g.dart`/`.generated.ts`/`.d.ts`, or one under `vendor`/`node_modules`/`generated`/`third_party`.

## Line caps

A comment block is capped at **two lines above a function declaration and one line above everything else** — a `var`, a `const`, a `type`, or a statement. `check` reports an over-cap block as `OVER_LINES` and `apply` refuses a proposal that would create one, counting the run the proposal lands in rather than only the proposals in your batch: a single line spliced above an existing comment over a `var` is already over. A machine directive (`//go:generate`, `//nolint`) inside the run does not count, and a file's leading header run is exempt up to `HEADER_MAX_LINES`.

`STACKED` is now the separate rule for two *distinct* blocks landing within `ADJACENT_WINDOW` lines of each other; lines inside one block are the cap's business.

## `DOC` shape

`DOC` only ever targets a top-level declaration (`func`, `type`, `const`, `var`, or `package`) in a `.go` file — never something indented, never a non-Go file (Swift/JS/TS/Rust share enough keywords with Go's `func`/`var`/`const` that the shape check alone cannot tell them apart, so `check` and `apply` both gate `DOC` to `.go` by extension). **Exported and unexported both qualify** — the capitalization gate is gone, because `FUNC_UNDOCUMENTED` expects a comment on every function and name-first is the shape Go uses for both. The text must start with the exact declared name (`Package <name>` for a package doc) and be exactly one sentence, ending in `.`/`!`/`?` with no other terminal punctuation before it. This is the one case where starting a comment with the name it documents is correct, not an echo — both `check` and `apply` skip the echo check for `DOC` specifically.

A `DOC` comment written inline by a session agent is not blocked, but `comments.py check` still applies the echo rule to it — a name-first comment only survives if it clears the full `DOC` shape (exact declared name, exactly one sentence, top-level, exported, `.go`). Anything that misses is a `NAME_ECHO` finding for this pass to fix.

## `FIELD` shape

`FIELD` is judged separately per field, not per struct. Add one only when a field's zero value, optionality, or unit is not recoverable from its name and type alone — most fields in most structs get nothing. Two fields three lines apart can each independently earn one; that is not a stack, it is two separate judgment calls landing next to each other. Capped at 80 characters, not 120.

## Block-level `WHY` over a `const`/`var` group

Usually the wrong shape, and capped at one line regardless. A prose paragraph over a whole block tends to either genericize past the point of usefulness ("bounds resource usage") or bake in numbers and jargon that drift out of sync the next time either changes ("multi-GB", "this package"). Prefer a trailing comment on the individual value — matching whatever per-value convention the file already uses, or a `FIELD`-shaped one — when the fact is a unit or magnitude. Reserve a block-level `WHY` for the rare case where the value's very existence, not just its size, would surprise a reader: a ceiling that exists purely to defeat a specific attack the name does not hint at.

## Invoking the validator

```
python3 ~/.claude/hooks/comments.py --repo-root <repo root> apply <<< '<your proposals JSON>'
```

It prints `{"spliced": [...], "dropped": [...]}` and writes any spliced comment straight into the file. `apply` is the sanctioned write path precisely because it is the only one that enforces shape, echo, cap, and adjacency before the text ever reaches the file.

Your own judgment about template shape is not authoritative; the validator's is. Report what actually landed versus what it dropped, using its `dropped` array's reasons verbatim.

## Output format

```
## Result

<how many proposals were drafted, spliced, and dropped, one sentence>

## Mandatory sites

- **`path/to/file.go:42`** — commented / skipped because <reason>, one line per site the scan reported

## Spliced

- **`path/to/file.go:42`** — <the text that landed>

## Dropped

- **`path/to/file.go:17`** — <the validator's own reason, or your own reason if you chose not to propose it>

## Restoration check

- **<original clause>** — landed verbatim / landed reworded as <text> / dropped because <reason>, one line per clause of any comment you deleted-and-restored this band

## Notes

- **<anything adjacent worth flagging, e.g. a stale marker the session agent should delete>** — one line
```

- **Omit `## Dropped` entirely if nothing was dropped.**
- **Omit `## Restoration check` entirely if this band deleted no comment you were asked to restore.**
- **Omit `## Notes` entirely if empty.**
