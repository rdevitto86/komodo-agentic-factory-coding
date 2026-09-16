# Observability

**Three signals, one correlation ID.** Logs say what happened, metrics say how often and how slow, traces say where the time went. A signal that cannot be joined to the other two is a dead end during an incident.

**Every signal carries the same trace ID.** That join is the entire value — without it you have three separate tools and a manual guess.

---

## Logs

### Levels

| Level | When |
|---|---|
| `ERROR` | Unexpected failure needing attention — unhandled exception, failed dependency call, data integrity violation |
| `WARN` | Unexpected but recovered — retry, fallback, deprecated usage, near-limit condition |
| `INFO` | Significant lifecycle event — startup, shutdown, config resolved, key state transition |
| `DEBUG` | Diagnostics during development — payloads, intermediate values, branch decisions |

**Never `INFO` or `DEBUG` per-request in production.** Routine request chatter buries the errors it sits between. One canonical line per request is the exception — see below.

**A panic or fatal is an `ERROR` with a stack**, not its own level. The process dying is the signal; a separate level adds nothing a stack trace does not.

### Per environment

| Environment | Active | Retention |
|---|---|---|
| local | DEBUG and above | session |
| dev, perf | DEBUG and above | short |
| staging, qa | INFO and above | per-deploy |
| prod | INFO and above, sampled | policy floor |

**Staging keeps `INFO`.** It is where a deploy is validated, and lifecycle events are exactly what you validate against. Dropping to errors-only there was hiding the signal that says the rollout worked.

### Required fields

**Deployed environments emit JSON.** Local may use line-oriented output.

| Field | Status |
|---|---|
| `timestamp`, `level`, `service`, `message` | Required |
| `trace_id` | Required — no exceptions |
| `user_id` | Required when a user context exists |

**`user_id` is the internal pseudonymous key, never an email or a name.** That is what keeps it out of the PII rule below.

**The message is a verb phrase with no function name and no interpolated values.** Values go in fields — an interpolated message is unsearchable and uncountable.

### The canonical request line

**One structured `INFO` line per request, at the edge, on completion.** Route, method, status, duration, trace ID, user ID. It replaces per-request chatter rather than adding to it, and it is what makes a log store answer "what happened to this user at 14:02" without a trace.

### Always log

Startup and shutdown, panics with stack, auth failures, access to PII, and every state transition that changes money or permissions.

### Never log

- Secrets, tokens, keys, passwords, or full card numbers
- Raw PII, or full request/response bodies containing it
- **Implementation plan or phase labels** — "Phase 3b", "stage 4 TODO". Meaningless outside the current dev cycle, and they rot immediately.

### Log once

Log at the top of the stack. Logging and returning the same error fragments one failure across several entries and inflates the error rate.

---

## Metrics

**Every service exposes the same four**: request rate, error rate, duration distribution, and saturation of whatever it is bounded by — pool, queue depth, memory.

**Record duration as a histogram, never an average.** A mean latency hides the tail that is actually failing for someone.

**Cardinality is the cost.** A label's value set must be bounded and small — route template, status class, method. **Never a user ID, request ID, trace ID, raw path, or error message as a label**; each distinct value is a new time series, and one unbounded label can take down the metrics backend rather than the service.

**Alert on symptoms, not causes.** Error rate and latency are user-visible; CPU is not. A cause-based alert fires during a harmless spike and stays silent during a real outage.

**A metric with no dashboard and no alert is dead weight.** Delete it.

---

## Traces

**A span per meaningful unit of work** — the inbound request, each outbound call, each database query, each queue publish. Not per function.

**Propagate context across every boundary**, using the standard W3C trace context headers. A break in propagation splits one trace into orphans, which is worse than no trace at all because it looks complete.

**Span names are low-cardinality**, matching the metric label rule — the route template, not the resolved path. High-cardinality detail goes in span attributes.

**Record the error on the span** that failed, not only on the one that caught it.

**Sample by policy, and always keep the errors.** Head sampling on a fixed fraction is fine for volume; anything that errored or breached a latency threshold is kept regardless.

---

## Browser

**The console carries high-level events only.** Never stack traces, never raw API bodies — both leak internals to anyone with devtools open.

**Client errors report to the same backend as the server's**, carrying the trace ID from the originating request. A frontend error store nobody joins to the backend is a second dead end.
