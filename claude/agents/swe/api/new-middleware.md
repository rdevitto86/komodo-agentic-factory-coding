# Skill: /new-middleware

Scaffold a new HTTP middleware in `komodo-forge-sdk-go` following established SDK conventions.

## Usage

```
/new-middleware <Name> [--read|--write|--auth] [--stateful]
```

- `<Name>` — PascalCase middleware name, e.g. `RateLimiter`, `Idempotency`, `IPWhitelist`. The exported function becomes `<Name>Middleware`.
- `--read` — Middleware applies to read (GET/HEAD) paths. No body parsing.
- `--write` — Middleware applies to mutating paths. May inspect request body.
- `--auth` — Middleware participates in the auth chain. Must call `next` only after validation passes.
- `--stateful` — Middleware requires initialization (e.g. in-memory store, Redis client). Generates a constructor instead of a plain handler wrapper.

**Must be run from inside a clone of the `komodo-forge-sdk-go` repo (`github.com/rdevitto86/komodo-forge-sdk-go`)** — this skill modifies the SDK itself, the one case where you work against the SDK source directly rather than the published module.

---

## Before generating anything

1. Read `http/middleware/exports.go` — understand the export pattern and which middlewares exist.
2. Read one existing middleware (e.g. `http/middleware/rate_limiter.go`) — follow naming, error handling, and `next.ServeHTTP` call conventions exactly.
3. Read `http/errors/` — confirm `httpErr.SendError` signature for returning errors from middleware.
4. Read `logging/runtime/` — confirm logger call signatures.

---

## Files to generate

### `http/middleware/<name>.go`

```go
package middleware

import (
	httpErr "github.com/rdevitto86/komodo-forge-sdk-go/http/errors"
	logger "github.com/rdevitto86/komodo-forge-sdk-go/logging/runtime"
	"net/http"
)

// <Name>Middleware TODO: describe what this middleware does.
func <Name>Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(wtr http.ResponseWriter, req *http.Request) {
		// TODO: implement middleware logic

		next.ServeHTTP(wtr, req)
	})
}
```

**Stateful variant** — generate a struct + constructor instead:

```go
type <Name>Middleware struct {
	// TODO: fields
}

func New<Name>Middleware(/* config */) *<Name>Middleware {
	return &<Name>Middleware{}
}

func (m *<Name>Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(wtr http.ResponseWriter, req *http.Request) {
		// TODO: implement
		next.ServeHTTP(wtr, req)
	})
}
```

---

### `http/middleware/<name>_test.go`

```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test<Name>Middleware(t *testing.T) {
	t.Run("passes through when conditions are met", func(t *testing.T) {
		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		})

		handler := <Name>Middleware(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		wtr := httptest.NewRecorder()

		handler.ServeHTTP(wtr, req)

		if !called {
			t.Error("expected next handler to be called")
		}
	})

	// TODO: add test cases for rejection, edge cases
}
```

---

### `http/middleware/exports.go`

Append the new middleware to the exports file so consumers can import it from a single location:

```go
var <Name>Middleware = <name>.<Name>Middleware  // or reference directly if same package
```

Follow the exact export style already in `exports.go`.

---

## After generating

1. Run `go build ./...` from the SDK root to confirm the middleware compiles.
2. Run `go test ./http/middleware/...` to confirm the test stub passes.
3. Remind the developer to:
   - Register the middleware in the appropriate stack in any service that needs it.
   - Document the config keys the middleware reads from `config.GetConfigValue` in the service's `README.md`.
