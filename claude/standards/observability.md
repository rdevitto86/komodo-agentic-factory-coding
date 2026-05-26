# Observability Standards

Covers structured logging correlation, metrics, distributed tracing, and health checks. Log levels, required fields, and what not to log are in `logging.md` — this document covers the instrumentation layer.

---

## 1. The three pillars

| Pillar | Purpose | Tool |
|--------|---------|------|
| Logs | Discrete events with context | forge SDK logger |
| Metrics | Aggregate measurements over time | CloudWatch / Prometheus |
| Traces | Request flow across service boundaries | AWS X-Ray / OpenTelemetry |

All three must be present in every service. Logs alone are not sufficient in a distributed system.

---

## 2. Correlation — trace_id

Every inbound request must generate or propagate a `trace_id`. This ID flows through all downstream calls and appears in every log line and span.

**In Go (forge SDK):**
```go
// TelemetryMiddleware sets trace_id on the context — do not reinvent.
// Pass ctx through every function call; never drop it.
traceID := middleware.TraceIDFromContext(ctx)

// Logger picks up trace_id automatically when using WithContext:
logger.WithContext(ctx).Info("processing order")
```

**Propagation rules:**
- Pass `trace_id` in the `X-Trace-Id` header on all outbound HTTP calls.
- Pass `ctx` through every function call — never drop the context.
- Include `trace_id` in every log line (automatic when using `logger.WithContext(ctx)`).

---

## 3. Metrics

Emit metrics for every service:

| Metric | Type | Labels |
|--------|------|--------|
| Request rate | Counter | endpoint, method |
| Error rate | Counter | endpoint, status_code |
| Latency | Histogram | endpoint (p50/p95/p99) |
| Saturation | Gauge | resource (goroutines, pool size, queue depth) |

**Cardinality rule:** Never use high-cardinality values (user IDs, order IDs, session tokens) as metric labels. Use bucketed or categorical labels only. A metric with unbounded label cardinality will OOM the metrics backend.

---

## 4. Distributed tracing

Instrument all cross-boundary operations:
- Inbound HTTP requests (handled by `TelemetryMiddleware`)
- Outbound HTTP calls (inject `X-Trace-Id` header)
- Database queries (span per query)
- External service calls (SQS, SNS, Secrets Manager)

```go
ctx, span := tracer.Start(ctx, "db.get_order")
defer span.End()

if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, "failed to get order")
}
```

Span names: `<component>.<operation>` in snake_case — `db.get_order`, `http.post_payment`, `cache.get_session`.

---

## 5. Health check endpoints

Every service exposes two endpoints:

| Endpoint | Checks | Returns |
|---------|--------|---------|
| `GET /health` | Process liveness only | `200 {"status":"ok"}` always |
| `GET /ready` | Dependency reachability | `200` when ready, `503` when not |

Liveness (`/health`) must never check external dependencies — a slow database should not restart the container. Readiness (`/ready`) checks connection pool availability and critical dependency reachability.

---

## 6. Alerting

Alert on symptoms, not causes. "p99 latency > 1s for 5 minutes" is actionable; "CPU > 80%" is not.

Every alert must have a runbook link in the alert body. Starting thresholds:

| Condition | Duration | Response |
|-----------|---------|----------|
| Error rate > 1% | 5 min sustained | Page on-call |
| p99 latency > 2s | 5 min sustained | Page on-call |
| `/health` failing | Immediate | Page on-call |
| Error rate 0.1–1% | 15 min sustained | Slack notification |

Tighten thresholds over time as you learn the service's baseline. A noisy alert is worse than no alert.

---

## 7. Frontend observability

- Use the project's observability service (Sentry, Datadog RUM) for error tracking — do not roll custom error collection.
- Track navigation events and major user flows at INFO level — not every click or keypress.
- Never log tokens, session data, or PII. See `logging.md`.
- Track Web Vitals (LCP, FID, CLS) and alert on regressions between deploys.
