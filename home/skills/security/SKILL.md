---
name: security
description: Security baseline: secrets, input validation, authn/authz, PII, OWASP.
user-invocable: false
---

# Security

## Secrets

- **No hardcoded secrets, tokens, or credentials** in source or config.
- **A secrets manager**, never environment variables for sensitive values in production.
- **Rotate immediately on suspected exposure.**
- **Never log** secrets, tokens, PII, or full bodies containing sensitive fields.
- **`.env` is local-dev only** — never committed, always gitignored.

## Input validation

- **Validate and sanitise at every system boundary** — API edge, queue consumer, upload handler.
- **Server-side validation is mandatory.** Client-side validation is a UX concern only.
- **Check type, format, length, range, and allowed character set.**
- **Reject unexpected fields at untrusted external edges** (public API, upload, webhook). Internal and versioned payloads tolerate unknown fields for forward compatibility — reject at the perimeter, tolerate behind it.
- **Never pass unvalidated input** to a query, shell command, or template renderer.

## Authentication and authorization

- **Every endpoint requires auth** unless explicitly documented as public.
- **Authorization checks live at the service layer**, not only at the gateway.
- **Least privilege** for every service account, IAM role, and user role.
- **Short-lived session tokens** with refresh rotation.
- **MFA for admin access to production.**

## Dependencies

- **Vulnerability scanning in CI on every change.**
- **No merging with known high or critical CVEs** without a recorded, deliberate exception.
- **Lock versions** — no floating ranges in production dependencies.
- **Review transitive dependencies** when adding a direct one.

## Data handling

- **Classify before storing** — public, internal, confidential, restricted.
- **PII encrypted at rest and in transit** (TLS 1.2+ minimum).
- **Define retention before storing** any user data.
- **Deletion requests reach backups and derived datasets**, not just the primary store.
- **Never copy production data into a lower environment** without scrubbing PII first.

## OWASP mitigations

| Risk | Mitigation |
|---|---|
| Injection | Parameterised queries only; never concatenate SQL or shell commands |
| Broken access control | Authorization at the service layer, per-resource ownership checks |
| Cryptographic failures | TLS in transit, encryption at rest, constant-time secret comparison |
| Insecure design | Threat-model the boundary before implementing it |
| Security misconfiguration | Deny by default; no wildcard CORS, no `0.0.0.0/0` ingress |
| Vulnerable components | CI scanning, locked versions, prompt patching |
| Auth failures | Short-lived tokens, rotation, rate-limited login paths |
| Integrity failures | Verify signatures on anything executed or deserialised |
| Logging failures | Log auth failures and PII access; never log the secrets themselves |
| SSRF | Allowlist outbound hosts; never fetch a user-supplied URL unvalidated |

## Incident response

Isolate, assess blast radius, notify within an hour, record the timeline, rotate every affected credential, then run a blameless post-mortem with a concrete prevention step.
