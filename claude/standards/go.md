# Go Standards

Base Go coding standards for Komodo. Project-level configs may extend these but should not contradict them.

---

## 1. Formatting & tooling

- All code formatted with `gofmt` or `goimports` — no exceptions
- `golangci-lint` must pass before merge; config lives in `.golangci.yaml` at repo root
- No disabling linter rules without a comment explaining why
- Run `go vet ./...` as part of CI
- The `gopls` LSP is the standard editor integration — formatting, lint diagnostics, refs, and rename. Agents should use the LSP tool when available before falling back to `go build` / `go vet` for quick checks; IDEs auto-discover it

---

## 2. Error handling

- Always handle error return values — never `_` an error
- Wrap errors with context using a short noun phrase describing the operation: `fmt.Errorf("fetching user: %w", err)` — never the function name
- Return errors up the call stack; don't log and return
- Log errors once — at the top of the call stack where you stop propagating
- No `panic` except for truly unrecoverable initialization failures (missing required config at startup)
- Sentinel errors (`var ErrNotFound = errors.New(...)`) for errors callers need to match; `errors.As`/`errors.Is` for inspection
- Error string must not contain the function name — put function context in structured log fields or stack traces, not the message

---

## 3. Naming

- Exported names must have a doc comment; doc comments must not open with the identifier name — write `// Returns the user for the given ID` not `// GetUser returns the user for the given ID`
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

- **Dependency injection over package-level singletons** — inject dependencies (`*http.Client`, DB handles, clocks, loggers) into structs via constructors or option functions. Package-level globals make units untestable without real I/O or monkey-patching.
- **Accept interfaces, return concrete types** — define interfaces at the point of consumption, not at the implementation. Keeps the dependency graph explicit and avoids premature abstraction.
- **Idiomatic wiring** — use constructor functions (`NewServer(...)`, `NewService(...)`) to assemble the dependency graph. Avoid `init()` side effects, global registries, and auto-registration.
- **Avoid init-time I/O** — `init()` functions must not make network calls, open files, or connect to databases. Failure in `init` produces unclear error messages and untestable startup sequences.

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
- Benchmark critical paths with `testing.B`; store benchmarks alongside the code they measure
- Database queries: use `EXPLAIN ANALYZE` before adding an index; N+1 queries are a bug
