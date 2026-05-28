# Comments

**Scope: every file, every language, no exceptions.** These rules apply to every file you create or edit (`.go`, `.ts`, `.py`, `.svelte`, `.c`, `.rs`, config, scripts, everything). No file type is exempt. The only per-file-type variation is the test-section banner (last row of the table).

## The rule

**Add a comment only where a reader cannot deduce the code's purpose or behavior from the name, signature, and surrounding code.** If the name already says it, the comment is noise — and worse, it drifts: comments are not compiled, not tested, and rot the moment the code changes underneath them. Prefer fewer comments, each carrying real weight.

A name like `SetCacheItem(...)` or `var worker = NewWorker()` is self-evident — no comment. Reach for one only when there is something the code cannot tell the reader on its own:

- a non-obvious contract — errors returned, preconditions, side effects
- a concurrency or ordering constraint (caller must hold a lock, must run before X)
- a hidden mutation — it modifies an argument, or shared state, in a way the signature doesn't reveal
- a surprising performance characteristic, or a workaround for an external quirk

**The bar is identical for every declaration — exported or not, function, type, or var.** Export status creates no obligation to comment; only non-obvious behavior does. When in doubt, leave it out.

**When you do comment, this intentionally overrides each language's native doc convention** (Go godoc, JSDoc, etc.). Those open a doc comment with the identifier name; we do not. If your training says "start the comment with the function name," that is the exact habit these rules override.

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

- **Documenting something whose name already conveys its purpose** — `// SetCacheItem sets a cache item`, `// worker is a worker`. The name is the documentation.
- **File / package / module-level doc block** — e.g. `// Package foo provides…`. No exceptions.
- **Opening a comment with the function/type/method name**, even paraphrased — `// Component skips…`, `// Config holds…`.
- **Multi-line / multi-paragraph doc block** — past 2 sentences, or any block comment sitting above a type/var declaration.
- **A doc comment on a test function** describing what it asserts.
- **Restating the code** — `i++ // increment i`.
- **Change history, author tags, dates, ownerless TODOs, commented-out code, jokes/apologies/editorial.**

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

**Bad — opens with the name (the godoc habit this file overrides):**
```go
// VerifyOTP looks up the stored OTP and compares it to the submitted code.
func (c *CacheClient) VerifyOTP(email, code string) error { ... }
```

## Test-file section banner — exact format only

```
// ── Unit Tests ──────────────────────────────────────────────────────────
// ── Component Tests ─────────────────────────────────────────────────────
// ── Integration Tests ───────────────────────────────────────────────────
```
