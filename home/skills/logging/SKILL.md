---
name: logging
description: Logging baseline: levels, per-environment verbosity, required fields, what never to log.
user-invocable: false
---

# Logging

## Levels

| Level | When |
|---|---|
| `ERROR` | Unexpected failures needing attention — unhandled exceptions, failed service calls, data integrity issues |
| `WARN` | Unexpected but recovered — retries, fallbacks, deprecated usage, near-limit conditions |
| `INFO` | Significant lifecycle events — startup, shutdown, config loaded, key state transitions |
| `DEBUG` | Diagnostics during development — bodies, intermediate values, branch decisions |

**Never use `INFO` or `DEBUG` for routine requests in production.** The noise buries real errors.

## Per environment

| Environment | Active | Retention |
|---|---|---|
| local | DEBUG and above | session |
| dev, perf | DEBUG and above | 24 hours |
| staging, qa | ERROR, WARN | per-deploy or 7 days |
| prod | ERROR, WARN | 30 days minimum |

Higher environments emit less so real errors are immediately visible.

## Format

**Deployed environments emit JSON.** Local development may use line-oriented output.

Required fields on every entry: `timestamp`, `level`, `service`, `message`.
Strongly recommended: `trace_id`, `user_id`.

**The message is a verb phrase with no function name** and no interpolated values — those go in fields.

## Always log

Startup and shutdown, panics with stack, auth failures, access to PII, and every state transition that changes money or permissions.

## Never log

- Secrets, tokens, keys, passwords, or full card numbers
- Raw PII, or full request/response bodies containing it
- **Implementation plan or phase labels** — "Phase 3b", "stage 4 TODO". They are meaningless outside the current dev cycle and rot immediately.

## Log once

Log at the top of the stack. Logging and returning the same error fragments one failure into several entries.

## UI logging

The browser console carries high-level events only. Never stack traces, never raw API bodies — both leak internals to anyone with devtools open.
