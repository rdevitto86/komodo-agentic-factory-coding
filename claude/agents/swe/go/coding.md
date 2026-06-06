# Go Standards

High-level Go idioms for Komodo. Cross-cutting doctrine — hard rules, error-string format, design/DI principles, code-reuse priority, comments — lives in `principles.md` and `comments.md`; this file does not restate it. Reuse `komodo-forge-sdk-go` before writing custom Go (`principles.md` §2). Project-level configs may extend these but must not contradict them.

---

## 1. Formatting & tooling

- All code formatted with `gofmt` or `goimports` — no exceptions
- `golangci-lint` must pass before merge; config lives in `.golangci.yaml` at repo root
- No disabling linter rules without a comment explaining why
- Run `go vet ./...` as part of CI
- The `gopls` LSP is the standard editor integration — formatting, lint diagnostics, refs, and rename. Agents should use the LSP tool when available before falling back to `go build` / `go vet` for quick checks; IDEs auto-discover it

---

## 2. Error handling

- Error message format (verb-leading, no function/noun prefix) follows `principles.md` §1 — the single source of truth
- Always handle error return values — never `_` an error
- Wrap with `%w` to preserve the chain: `fmt.Errorf("failed to fetch user: %w", err)`
- Return errors up the call stack; don't log and return. Log once, at the top of the stack where propagation stops
- No `panic` except for truly unrecoverable initialization failures (missing required config at startup)
- Sentinel errors (`var ErrNotFound = errors.New(...)`) for errors callers need to match; `errors.As`/`errors.Is` for inspection

---

## 3. Naming

- Comments and doc-comment requirements: governed entirely by `comments.md` (the single source of truth; it deliberately overrides Go's godoc naming convention)
- Interfaces named by behavior: `Reader`, `Writer`, `Handler` — not `IReader`, `ReaderInterface`
- Avoid stutter: `user.Service` not `user.UserService`
- Unexported identifiers: camelCase, no underscores
- Acronyms: consistent casing — `userID`, `httpURL`, `parseJSON` (all caps or all lower, never mixed)
- Test helper functions: `newTestServer(t)`, `mustParseTime(t, s)` — take `t *testing.T` as first arg
- HTTP handler variables: `req` for request objects (e.g. decoded body structs, outgoing `*http.Request`), `res` for response objects (e.g. `*http.Response`, response body structs) — Go handler signatures keep `r *http.Request` and `w http.ResponseWriter` by language convention

---

## 4. Package design

- One package per directory
- Package name = what it provides, singular, lowercase: `order`, `payment`, `middleware`
- Avoid `util`, `common`, `helpers`, `misc` — if you can't name it, split it differently
- Internal packages (`internal/`) for code that must not be imported outside the module
- Minimize exported surface — unexport everything that doesn't need to be consumed externally

---

## 5. Testing

See [`testing-go.md`](testing-go.md) for the full Go testing standard — colocation, table-driven structure, mocking at interface boundaries, coverage targets, race/timing, and integration test conventions.

---

## 6. Design patterns

Follow the design doctrine in `principles.md` §3–4 (dependency injection, accept interfaces / return concrete types, explicit wiring, composition, testability). Go-specific application:

- Assemble the dependency graph with constructor functions (`NewServer(...)`, `NewService(...)`) — not package-level globals, registries, or `init()` registration
- `init()` must not perform I/O — no network calls, file opens, or DB connections; init failures produce unclear errors and untestable startup sequences

---

## 7. Concurrency

- Never share mutable state without synchronization
- Prefer channels for coordination between goroutines; mutexes for protecting shared state
- Document goroutine lifetimes — who starts it, what stops it, what happens on error
- Always propagate and respect `context.Context` cancellation
- Use `sync.WaitGroup` or `errgroup` for goroutine lifecycle management
- `go func()` with no done signal is almost always wrong — capture and handle the goroutine

---

## 8. Performance

- Profile before optimizing — `pprof` is built in, use it
- Avoid premature allocation: prefer stack allocation, reuse slices with `[:0]`, use `strings.Builder`
- `sync.Pool` for frequently allocated short-lived objects on hot paths
- Benchmark critical paths with `testing.B` — see `testing-go.md` §6 for placement and the `benchstat` workflow
- Database queries: use `EXPLAIN ANALYZE` before adding an index; N+1 queries are a bug
