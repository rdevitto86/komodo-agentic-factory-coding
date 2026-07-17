# Go slice — the API profile expressed in Go

Read this with `MODES: go, api` active. It is the Go form of `profile.md`'s bones plus a copy-me vertical slice of one endpoint end-to-end. Go anatomy defers to `~/.claude/standards/stack.md` §4 and `~/.claude/modes/go/coding.md`; this file shows the *shape*. All examples are comment-free — the exemplar's comments are human-authored and are not reproduced.

---

## 1. Bone → Go expression

| Bone | Go form |
|---|---|
| Error wrapped, cause matchable | `fmt.Errorf("failed to X: %w", err)`; sentinels as package vars matched with `errors.Is`; typed errors matched with `errors.As` |
| Interface declared at the consumer | the consuming package declares the interface; the implementation lives in its own package with a `New(Config)` constructor |
| Two-tier repository | a rich domain interface sits over a thin primitive-operations interface; only the primitive tier binds the driver, so domain logic tests against a fake |
| Injected clock | `Clock func() time.Time` field + an `s.now()` helper; no direct `time.Now()` in business logic |
| Hot-swappable config | `atomic.Pointer[T]` holding active/previous; double-checked locking for a cached token |
| Disciplined goroutine | `context.WithoutCancel` + `context.WithTimeout` + a bounding semaphore + `recover()` logging `debug.Stack()` |
| Routing | stdlib `net/http.ServeMux` with `"METHOD /v1/path"` patterns; no third-party router |
| Middleware | `[]func(http.Handler) http.Handler` slices composed via the SDK's `Chain`; appended for stricter route variants |
| Handler | a method on `*Service`; client errors via the SDK error emitter; success via a single local `writeJSON` |
| Config/secrets | fetched once at boot from the SDK secrets surface; hot-reloaded via the SDK watcher |
| Tests | colocated `_test.go` for unit (`-short`); `test/<tier>/` with build tags for the rest; component builds the real `*Service` with generated mocks + per-run RSA keys; `TestMain` for setup; `t.Helper()` on helpers |
| Codegen | `oapi-codegen` per target with its own config; mocks via `//go:generate mockgen` into `test/mocks/`; a freshness gate in CI |

---

## 2. Vertical slice — one endpoint, end to end

Copy the **structure**, not the names. `widget`/`thing` are placeholders for whatever this API's domain is.

### Layout

```
cmd/server/main.go        wiring: secrets → deps → Service → muxes → servers
cmd/server/setup.go       mux construction + middleware stacks + server lifecycle
internal/api/service.go   the Service aggregate: deps as interfaces + New(ServiceConfig)
internal/api/widget.go    the handler (method on *Service)
internal/db/widget.go     the repository (two-tier interface)
internal/clients/thing.go outbound client to a sibling API
internal/models/          generated from openapi.yaml — never hand-edited
test/component/widget_test.go  real Service + generated mocks
```

### The Service aggregate — `internal/api/service.go`

The consumer declares the interfaces it needs; concrete implementations live elsewhere and are injected.

```go
package api

type WidgetStore interface {
	GetWidget(ctx context.Context, id string) (*db.Widget, error)
}

type Service struct {
	Widgets WidgetStore
	Clock   func() time.Time
}

type ServiceConfig struct {
	Cache *db.CacheConfig
	Clock func() time.Time
}

func New(cfg ServiceConfig) (*Service, error) {
	store, err := db.NewWidgetStore(*cfg.Cache)
	if err != nil {
		return nil, fmt.Errorf("failed to create widget store: %w", err)
	}
	return &Service{Widgets: store, Clock: cfg.Clock}, nil
}

func (s *Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}
```

### The handler — `internal/api/widget.go`

Uniform shape: headers → extract+validate → call dep → map errors → success. Errors go through the SDK emitter; success through `writeJSON`.

```go
package api

func (s *Service) GetWidgetHandler(wtr http.ResponseWriter, req *http.Request) {
	wtr.Header().Set("Content-Type", "application/json")

	id := req.PathValue("id")
	if id == "" {
		httpErr.SendError(wtr, req, httpErr.Global.BadRequest, httpErr.WithDetail("id is required"))
		return
	}

	widget, err := s.Widgets.GetWidget(req.Context(), id)
	switch {
	case errors.Is(err, db.ErrWidgetNotFound):
		httpErr.SendError(wtr, req, httpErr.Global.NotFound, httpErr.WithDetail("widget not found"))
		return
	case err != nil:
		logger.Error("failed to get widget", err, logger.Attr("handler", "GetWidget"))
		httpErr.SendError(wtr, req, httpErr.Global.Internal, httpErr.WithDetail("failed to fetch widget"))
		return
	}

	writeJSON(wtr, http.StatusOK, widget)
}
```

### The repository — `internal/db/widget.go`

Two tiers: the domain method (`GetWidget`) on top of a primitive-operations interface (`CacheOperations`). Only the primitive tier knows the driver.

```go
package db

var ErrWidgetNotFound = errors.New("widget not found")

const widgetKey = "widget:"

type CacheOperations interface {
	Get(ctx context.Context, key string) (string, error)
}

type WidgetStore struct {
	ops CacheOperations
}

func NewWidgetStore(cfg CacheConfig) (*WidgetStore, error) {
	ops, err := redis.NewFromConfig(cfg.Endpoint, cfg.Password, cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to cache: %w", err)
	}
	return &WidgetStore{ops: ops}, nil
}

func NewWidgetStoreFromOps(ops CacheOperations) *WidgetStore {
	return &WidgetStore{ops: ops}
}

func (s *WidgetStore) GetWidget(ctx context.Context, id string) (*Widget, error) {
	raw, err := s.ops.Get(ctx, widgetKey+id)
	if err != nil {
		return nil, fmt.Errorf("failed to read widget: %w", err)
	}
	if raw == "" {
		return nil, ErrWidgetNotFound
	}
	var widget Widget
	if err := json.Unmarshal([]byte(raw), &widget); err != nil {
		return nil, fmt.Errorf("failed to unmarshal widget: %w", err)
	}
	return &widget, nil
}
```

### The outbound client — `internal/clients/thing.go`

Non-2xx becomes a typed SDK `HTTPError` (so callers match it with `errors.As`); the body is size-limited and always drained + closed.

```go
package clients

const maxThingBody = 1 << 20

func (c *HttpClient) GetThing(ctx context.Context, id, bearer string) (*Thing, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/things/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build get thing request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearer)

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get thing: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()
	}()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxThingBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read thing response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &sdkhttp.HTTPError{StatusCode: resp.StatusCode, Body: raw}
	}

	var thing Thing
	if err := json.Unmarshal(raw, &thing); err != nil {
		return nil, fmt.Errorf("failed to unmarshal thing response: %w", err)
	}
	return &thing, nil
}
```

### The wiring — `cmd/server/setup.go`

Middleware stacks are slices composed with the SDK `Chain`; routes registered on the mux.

```go
publicMW := []func(http.Handler) http.Handler{
	mw.RequestIDMiddleware,
	mw.TelemetryMiddleware,
	mw.RateLimiterMiddleware,
}

mux := http.NewServeMux()
mux.Handle("GET /v1/widgets/{id}", mw.Chain(http.HandlerFunc(svc.GetWidgetHandler), publicMW...))
```

### The component test — `test/component/widget_test.go`

Builds the **real** `*Service` with a generated mock store; drives it over `httptest`.

```go
func TestGetWidget(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := mocks.NewMockWidgetStore(ctrl)
	store.EXPECT().GetWidget(gomock.Any(), "w1").Return(&db.Widget{ID: "w1"}, nil)

	svc := &api.Service{Widgets: store}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/widgets/{id}", svc.GetWidgetHandler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/widgets/w1")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}
```
