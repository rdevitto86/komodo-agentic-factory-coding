# Logging Standards

Universal logging baseline for all Komodo applications. Every app ships with logging — it is the primary tool for diagnosing production issues and must be treated as a first-class concern, not an afterthought.

---

## 1. Log levels

| Level | When to use |
|-------|-------------|
| `ERROR` | Unexpected failures that require attention: unhandled exceptions, service call failures, data integrity issues |
| `WARN` | Unexpected but recoverable: retries, fallbacks, deprecated usage, near-limit conditions |
| `INFO` | Significant lifecycle events: startup, shutdown, configuration loaded, key state transitions |
| `DEBUG` | Detailed diagnostic information useful during development: request/response bodies, intermediate values, branch decisions |

Never use `INFO` or `DEBUG` for routine requests in production — they generate noise that buries real errors.

---

## 2. Environment log levels

| Environment | Active levels | Retention |
|------------|---------------|-----------|
| `local` | DEBUG, INFO, WARN, ERROR | Session only |
| `dev` | DEBUG, INFO, WARN, ERROR | 24 hours |
| `perf` | DEBUG, INFO, WARN, ERROR | 24 hours |
| `staging` | ERROR, WARN | Per-deploy or 7 days |
| `qa` | ERROR, WARN | Per-deploy or 7 days |
| `prod` | ERROR, WARN | 30 days minimum |

The goal: higher environments emit less noise so real errors are visible immediately. Full logging in lower environments enables deep debugging without generating cost or compliance risk in production.

---

## 3. Log formats

Komodo uses two output formats depending on context:

### String logs
Human-readable, line-oriented output. Used for local development and wherever a developer is reading logs directly in a terminal.

```
2024-03-15T14:32:00Z [ERROR] payment-service: charge failed — stripe_id=ch_xxx reason=card_declined user_id=u_456
```

Format: `<timestamp> [<LEVEL>] <service>: <message> — <key=value pairs>`

### JSON logs
Structured output for log aggregation, querying, and search (CloudWatch, Datadog, etc.). Used in all deployed environments.

```json
{
  "timestamp": "2024-03-15T14:32:00Z",
  "level": "error",
  "service": "payment-service",
  "message": "charge failed",
  "stripe_id": "ch_xxx",
  "reason": "card_declined",
  "user_id": "u_456",
  "trace_id": "abc123"
}
```

**Required fields in every JSON log entry:** `timestamp` (ISO 8601 UTC), `level`, `service`, `message`.

**Recommended fields when available:** `trace_id`, `user_id`, `request_id`, `environment`.

---

## 4. UI / frontend logging

Browser console output follows additional rules to prevent security leaks:

- **Console output is high-level only** — no stack traces, no internal service names, no raw API error bodies
- Show enough for a support agent or developer to triage: error category, user-facing message, a reference ID
- Never log authentication tokens, session data, PII, or raw request payloads to the console
- Structured errors from the API (`error.code`, `error.message`) are safe to surface; `error.details` internals are not

```js
// Good — enough to triage
console.error('[checkout] Payment failed', { code: 'CARD_DECLINED', ref: 'ch_xxx' })

// Bad — exposes internals
console.error('[checkout] Error:', fullApiResponseBody)
```

For production browser telemetry, route errors to an observability service (e.g., Sentry) — do not rely on console logs.

---

## 5. What must always be logged

Regardless of environment or log level configuration:

- Application startup (service name, version, environment, port)
- Application shutdown (reason if known)
- Unhandled errors and panics (always ERROR)
- Authentication failures (ERROR or WARN — never swallow silently)
- External service call failures with enough context to retry or debug
- Any operation touching user PII or payment data (audit trail — what happened, who, when; not the data itself)

---

## 6. What must never be logged

- Passwords, tokens, API keys, secrets
- Full credit card numbers or CVVs
- PII in raw form: SSNs, full addresses, DOBs (use masked or reference IDs)
- Full request/response bodies unless explicitly needed for a debug session and stripped before commit
- Internal infrastructure details (internal IPs, DB connection strings) in production logs

---

## 7. Correlation and tracing

- Every inbound request should generate or propagate a `trace_id`
- The `trace_id` must flow through all downstream service calls for that request
- Log the `trace_id` on every log entry generated during request handling
- This is the primary tool for reconstructing what happened across services for a single user action
