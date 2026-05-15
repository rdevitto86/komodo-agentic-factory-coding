# Comments

Comments are a tax on every future reader. Write them only when they earn their place, and keep them tight.

## Inline comments

- Default to no comment. Well-named identifiers and small functions explain themselves.
- When you do comment, keep it to a single short line that says **why** the code is doing what it does — not what it does.
- Avoid restating the code in prose. `i++ // increment i` is noise.
- No multi-line inline blocks, no banners, no decorative separators (`// ===== Section =====`).
- Workarounds, hidden constraints, non-obvious invariants, surprising performance or ordering requirements — these earn a comment. Routine logic does not.

## Function / method / type doc comments

- One short paragraph, ideally one or two sentences. Say what the thing does at a high level and any contract that callers must know (preconditions, side effects, error semantics).
- Do **not** open with the function/method/type name. Write `// Returns the user for the given ID` not `// GetUser returns the user for the given ID`.
- Do not list every parameter and return value when the signature already makes it obvious. Document only the non-obvious ones.
- No essays. If a function needs paragraphs to explain, the function is wrong — split it.
- Do not reference the current task, PR, ticket, or caller ("added for the X flow", "used by Y"). That context belongs in the commit message and rots in the code.

## What never goes in comments

- Change history, author tags, dates — git knows.
- TODOs without an owner or ticket reference. If it matters, file it; if it doesn't, delete it.
- Commented-out code. Delete it; git remembers.
- Apologies, jokes, or editorial commentary.

## Rule of thumb

If removing the comment would not confuse a competent reader who knows the language and the codebase, remove it.
