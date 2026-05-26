# Comments

These rules apply to all languages. Follow them exactly — verbose comments are a hard violation.

## Function / method doc comments

Every function and method gets a doc comment. Write it in the idiom of the language (Go `//`, JS/TS `/** */`, Python `"""`, C/C++ `///`, Rust `///`).

**Rules:**
1. Start with a verb describing the action: `Creates`, `Runs`, `Validates`, `Helper that`, `Fetches`, etc.
2. Never open with the function or method name — not even paraphrased.
3. Keep it short: one sentence is the target. A second only when there is a non-obvious contract (error semantics, side effects, preconditions). Never a third.
4. Never write a multi-paragraph or multi-line verbose block.
5. Never document the entire file at the top of the file.

**Bad:**
```go
// ExecuteStatement runs a single SQL statement against the Aurora cluster.
// Named parameters in input.Parameters are converted to RDS Field values.
// Result rows are decoded into maps keyed by column name.
func (c *Client) ExecuteStatement(...) { ... }
```

**Good:**
```go
// Runs a single SQL statement against the Aurora cluster.
func (c *Client) ExecuteStatement(...) { ... }
```

**Bad (name-leading):**
```go
// VerifyOTP looks up the stored OTP and compares it to the submitted code.
func (c *CacheClient) VerifyOTP(email, code string) error { ... }
```

**Good:**
```go
// Validates the submitted OTP against the stored value for the given email.
func (c *CacheClient) VerifyOTP(email, code string) error { ... }
```

## Inline comments (inside function bodies)

**When to write one:**
- Calling out unique context on a var/const/struct field/object property — only when the name leaves real ambiguity or carries a non-obvious invariant. Max 125 characters, shorter is better.
- Labeling a distinct workflow section or significant downstream call: `// make request to payments downstream`, `// fetch user records from Komodo DB`.

**Rules:**
- Always lowercase, except: the first letter of the comment, acronyms (HTTP, AWS, SQL), and proper nouns.
- Use proper grammar.
- No multi-line inline blocks.
- Never restate what the code already says.

**Bad:**
```go
i++ // increment i
```

**Good:**
```go
TransactionID string // optional; ties execution to a BeginTransaction call
```

```go
// fetch and decode result rows
rows, err := decodeRecords(out.Records, out.ColumnMetadata)
```

## Types, structs, interfaces, vars, consts

No doc comment required on unexported types or simple structs whose fields are self-explanatory. If a field needs a note, put it inline — not in a block comment above the type.

**Bad:**
```go
// Config holds all settings required to construct a Client.
type Config struct {
    Endpoint string // leave empty in prod; used for component testing
}
```

**Good:**
```go
type Config struct {
    Endpoint string // optional; leave empty in prod
}
```

## Test files

- Never comment at the file, function, or section level in relation to tests themselves.
- Inline comments inside test functions are fine for explicit callouts and relevant context.
- Test helper functions may have a function-level doc comment.
- Section breaks use this exact format only:

```
// ── Unit Tests ────────────────────────────────────────────────────────────────────────────────────
// ── Component Tests ───────────────────────────────────────────────────────────────────────────────
// ── Integration Tests ─────────────────────────────────────────────────────────────────────────────
```

## What never goes in comments

- File-level package/module documentation blocks
- Change history, author tags, dates — git knows
- TODOs without an owner or ticket reference
- Commented-out code — delete it; git remembers
- Apologies, jokes, editorial commentary
- Restating what the code already says
