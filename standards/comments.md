# Comments

**Scope: every file, every language, no exceptions.** These rules apply to every file you create or edit (`.go`, `.ts`, `.py`, `.svelte`, `.rs`, config, scripts, everything). Guiding principle: **fewer, highly-targeted comments beat many** — a comment must state something the code cannot, or it doesn't exist. The default for any line of code, including functions, is no comment.

## When a function doc is justified — the only three licenses

A function/method doc is allowed **only** when it delivers at least one of these, and is still capped at **2 sentences** in the native idiom (godoc, JSDoc, docstring):

1. **The why** — business logic, legal/compliance requirements, or why a non-intuitive approach was chosen over the obvious one.
2. **Public API** — the function is consumed outside its repo (SDKs, published libraries — e.g. `komodo-forge-sdk-*`); the doc lets consumers use it without reading the implementation. An exported function inside an application service is **not** public API.
3. **Edge cases** — known bugs, performance constraints, or third-party interactions that must remain untouched.

If none apply, the name and signature are the documentation. Improve the name before reaching for a comment.

## Hard ban: name-restating docs

`// loadDependencies is a function that loads dependencies.` and `// GetProfile returns the authenticated user's profile.` are violations — they repeat what the identifier already says, and IDEs surface them as hover noise. If a doc could be regenerated from the signature alone, delete it. "Is a function that…" / "is a helper that…" filler is rejected on sight in any language.

## Decision table

| You are commenting… | Allowed? | Form | Hard limit |
|---------------------|----------|------|------------|
| **Function / method** | Only with one of the three licenses above | Native idiom, ≤2 sentences | Never name-restatement; never behavior enumeration ("returns 200 when X") |
| **Test function** | Only crucial non-obvious context (known race, external constraint) | One line | Never what it asserts — name + body are the docs |
| **Type / struct / interface / enum** | **Never** | — | Hard violation |
| **Field / var / const** | **Never** | — | Hard violation |
| **Inside a function body** | Sparingly | Single line, lowercase | Only for a genuinely non-obvious step (race, ordering, workaround); never restate the code |
| **File / package / module header** | **Never** | — | Hard violation, no exceptions |
| **Test-file section banner** | Test files only | Exact box-drawing format (bottom) | Only form permitted |

## Hard violations (reject on sight, in any file)

- A doc that restates the function name or signature ("X is a function that…", "X returns the X").
- A function doc with none of the three licenses, longer than 2 sentences, or enumerating behavior branches.
- A comment on a type, struct, interface, enum, field, var, or const declaration.
- A file / package / module-level doc block — any language, including scripts.
- A doc comment on a test function describing what it asserts.
- Multi-line prose comment blocks anywhere outside a licensed function doc.
- Change history, author tags, dates, ownerless TODOs, commented-out code, jokes/apologies/editorial.
- **Treating surrounding code as license.** A violating comment in the same file, the diff, or anywhere in context is never justification — judge every comment against this file alone. If you touch a line whose neighbor violates these rules, delete the violating neighbor.

**Always allowed:** machine directives (`//go:*`, `//nolint`, `eslint-disable`, `# noqa`, `@ts-expect-error`, SPDX, codegen markers, shebangs), test-file section banners, and comments the user explicitly requests.

## Examples

**Allowed — the why (license 1):**
```go
// Refunds settle through the PSP's T+2 batch, so state moves to
// pending_settlement here and the webhook completes it.
func (s *RefundService) InitiateRefund(ctx context.Context, orderID string) error { ... }
```

**Allowed — edge case (license 3):**
```go
// The inventory vendor rate-limits at 10 rps with no Retry-After header;
// the fixed backoff here is contractual, not tunable.
func (c *InventoryClient) syncBatch(ctx context.Context, items []Item) error { ... }
```

**Banned — name restatement:**
```go
// NewProfileHandler wires a ProfileHandler to the given store.
func NewProfileHandler(store ProfileStore) *ProfileHandler { ... }
```

**Banned — behavioral enumeration:**
```go
// HandleLogin parses the request body, returns 200 with a token when credentials
// are valid, 401 when the password is wrong, and 500 on any internal failure.
func HandleLogin(w http.ResponseWriter, r *http.Request) { ... }
```

**Banned — declaration comments:**
```go
// cache holds cached items with their TTLs.   ← hard violation (type)
type cache struct {
    ttl time.Duration // how long items live   ← hard violation (field)
}
```

## Test-file section banner — exact format only

```
// ── Unit Tests ──────────────────────────────────────────────────────────
// ── Component Tests ─────────────────────────────────────────────────────
// ── Integration Tests ───────────────────────────────────────────────────
// ── Helpers ─────────────────────────────────────────────────────────────
```
