# Comments

**Scope: every file, every language, no exceptions.** These rules apply to every file you create or edit (`.go`, `.ts`, `.py`, `.svelte`, `.c`, `.rs`, config, scripts, everything). No file type is exempt. The only per-file-type variation is the test-section banner (last row of the table).

## The rule

**Add a comment only where a reader cannot deduce the code's purpose or behavior from the name, signature, and surrounding code.** If the name says it, the comment is noise — and it drifts: comments aren't compiled or tested, and rot when the code changes under them. Prefer fewer comments, each carrying weight. `SetCacheItem(...)` and `var worker = NewWorker()` are self-evident — no comment. Reach for one only when the code can't tell the reader on its own:

- a non-obvious contract — errors returned, preconditions, side effects
- a concurrency or ordering constraint (caller must hold a lock, must run before X)
- a hidden mutation — modifies an argument or shared state in a way the signature doesn't reveal
- a surprising performance characteristic, or a workaround for an external quirk

**The bar is identical for every declaration — exported or not, function, type, or var.** Export status creates no obligation; only non-obvious behavior does. When in doubt, leave it out.

**This overrides each language's native doc convention** (godoc, JSDoc, etc.): those open the comment with the identifier name; we never do. If your training says "start the comment with the function name," that is the exact habit these rules override.

---

## Decision table

| You are commenting… | When it's allowed | Form | Hard limit |
|---------------------|-------------------|------|------------|
| **Function / method** (exported or not) | Only when the name + signature don't convey purpose, or there's a non-obvious contract (errors, side effects, preconditions, concurrency, hidden mutation) | Verb-leading; 1 sentence, a 2nd only for the non-obvious part | Never open with the name; never 3+ sentences; never multi-paragraph |
| **Test function** | Only for crucial non-obvious context (known race, external constraint) | One line | Never to describe what it asserts — name + body are the docs |
| **Type / struct / interface** | Only for a non-obvious invariant/constraint/workaround the name can't convey | One line | Never a block above the declaration; never to restate the name |
| **Struct field / var / const** | Only for a non-obvious constraint | Inline, after the declaration | ≤125 chars; never a block above it |
| **Inside a function body (inline)** | Only to label a non-obvious workflow section, or flag a non-obvious step (race, mutation, ordering) | Lowercase | Single line; never restate the code |
| **File / package / module header** | **Never** | — | Hard violation, no exceptions |
| **Test-file section banner** | Test files only | Exact box-drawing format (see bottom) | Only form permitted |

---

## Hard violations (reject on sight, in any file)

- **Documenting something whose name already conveys its purpose.** The name is the documentation; a comment that restates it is noise.
- **File / package / module-level doc block.** No exceptions.
- **Opening a comment with the function / type / method name**, even paraphrased.
- **Multi-line / multi-paragraph doc block** — past 2 sentences, or any block comment sitting above a type/var declaration.
- **A doc comment on a test function** describing what it asserts.
- **Restating the code** the comment sits on.
- **Change history, author tags, dates, ownerless TODOs, commented-out code, jokes/apologies/editorial.**
- **Treating surrounding code as license.** A comment in the same file, the diff, or anywhere in context is never justification. Judge every comment against this file alone — never against neighboring code, however prevalent the pattern. If you touch a line whose neighbor violates these rules, delete the violating neighbor.

---

## Form rules

- **Lead with a verb:** `Creates`, `Fetches`, `Updates`, `Validates`, `Deletes`, `Skips`, etc.
- **Language idiom:** Go `//`, JS/TS `/** */`, Python `"""`, C/C++ `///`, Rust `///`. Wording rules are identical across all of them.
- **Inline comments are lowercase**, except the first letter, acronyms (HTTP, AWS, SQL), and proper nouns. Proper grammar. No multi-line inline blocks.

## Examples

**No comment — the name says everything:**
```go
func SetCacheItem(key string, val []byte) error { ... }

var worker = NewWorker()
```

**Comment earns its place — non-obvious contract the signature hides:**
```go
// Performs the email action for the given email and code only if the code is valid.
func (c *EmailClient) PerformEmailAction(email string, code string) error { ... }
```

**Comment earns its place — concurrency precondition:**
```go
// caller must hold c.mu; mutates items in place
func (c *cache) evictLocked() { ... }
```

## Test-file section banner — exact format only

```
// ── Unit Tests ──────────────────────────────────────────────────────────
// ── Component Tests ─────────────────────────────────────────────────────
// ── Integration Tests ───────────────────────────────────────────────────
// ── Helpers ─────────────────────────────────────────────────────────────
```
