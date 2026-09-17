# API and backend security

## Secrets
- No hardcoded secrets in source, config, IaC, or a container layer. Values resolve at runtime from a secrets manager; an env var may name where to look, never carry the value.
- Rotate immediately on suspected exposure, and treat git history as exposure.
- Never log secrets, tokens, PII, or full bodies with sensitive fields. Redact at the logger.
- `.env` is local-dev only, always gitignored.

## Rate limiting and input contracts
- Every public or partner endpoint has a rate limit, per identity with an unauthenticated floor for login, signup, and reset. Return `429` with `Retry-After`.
- Cost-weight expensive operations; enforce quotas server-side, never from a client header.
- Validate every request against a schema before business logic. Cap payload size and pagination limits explicitly.
- Reject unknown fields at a public or partner boundary.
- Idempotency keys on every non-idempotent write a client might retry.
- GraphQL gets query-depth and complexity limits.
- Never pass unvalidated input to a query, shell, path, URL, deserialiser, or template.

## Versioning
- Version in the URL path or a dedicated header, never by payload shape. A breaking change is a new version.
- Publish a deprecation window with `Sunset` and `Deprecation` headers. A deprecated version keeps its full security posture until removed.

## Authentication and authorization
- Match the scheme to the trust boundary: API key for coarse server-to-server reads; OAuth 2.0/OIDC with minimal scopes for user-delegated access; mTLS inside a trust boundary; HMAC signing for webhooks with a clock-skew window.
- An API key never travels in a query string. Every credential is independently revocable.
- Every endpoint requires auth unless documented as public. Deny by default.
- Authorization lives at the service layer and checks ownership per resource on every request. Never bind a request body straight onto a model.
- Least privilege for every service account and role. MFA for admin access to production. Rate-limit and lock out every credential endpoint.

## Tokens, sessions, CSRF
- Pin the JWT algorithm server-side; never trust the token's `alg`, `kid`, `jku`, or `x5u`. Verify `iss`, `aud`, `exp`, `nbf` on every request.
- Never authorize on an unverified claim.
- Short-lived session tokens with refresh rotation, server-side revocation, and identifier rotation on privilege change.
- Cookies: `HttpOnly`, `Secure`, `SameSite` (`Strict` for admin), explicit `Path`.
- Anti-CSRF on every mutating route authenticated by an ambient credential. Never a state change through `GET`.

## Cryptography
- Never implement a primitive. Platform library at its defaults.
- AEAD only (GCM, ChaCha20-Poly1305). No ECB, no unauthenticated CBC, no MD5 or SHA-1 in a security context.
- Passwords use argon2id, scrypt, or bcrypt at a current cost.
- CSPRNG for every token, salt, IV, and nonce; never reuse or hardcode one.
- Constant-time comparison for every secret.

## Dependencies
- Vulnerability and secret scanning in CI on every change. No merge with a known High or Critical CVE without a recorded exception carrying owner, expiry, and compensating control.
- Lock versions, commit the lockfile, pin base images by digest.
- Review transitive dependencies and install-time scripts when adding a direct one.

## Data handling
- Classify before storing. PII encrypted at rest and in transit; TLS 1.2 floor, 1.3 where possible.
- Define retention before storing. Deletion reaches backups and derived datasets.
- Never copy production data to a lower environment without scrubbing PII.

## SSRF, traversal, deserialization
- Any server-side fetch of a client-influenced URL is SSRF until proven otherwise. Allowlist hosts, re-validate after DNS and each redirect, deny link-local, loopback, and private ranges.
- Never build a filesystem path from input without resolving to absolute and checking the root.
- Never deserialise untrusted data into arbitrary types.

## Errors and configuration
- Auth failure returns the same response whether the identity is unknown or the credential is wrong.
- Never echo stack traces, internal field names, or constraint messages to an untrusted caller.
- CORS is an explicit-origin allowlist; never a wildcard with credentials.
- No `0.0.0.0/0` ingress, public buckets, or `*` IAM. Debug surfaces (profiling, introspection, verbose errors) off in production.

## Agent and LLM surfaces
- Model output is untrusted input; anything the model reads is attacker-controlled.
- Retrieved or user text never selects a tool or authorizes an action. Validate tool arguments server-side as an HTTP body.
- Scope credentials per tool at least privilege; never put a secret in a prompt.

## OWASP quick map
| Category | Mitigation |
|---|---|
| Broken access control | Service-layer authz, per-resource ownership, tenant scoping in the query |
| Injection | Parameter binding, argument arrays |
| Cryptographic failures | AEAD, TLS, memory-hard KDF, constant-time compare |
| Misconfiguration | Deny by default, no wildcard CORS, no debug surface in prod |
| Vulnerable components | CI scanning, lockfiles, digest pins |
| Auth failures | Pinned JWT alg, verified claims, short-lived tokens, rate-limited credential paths |
| Integrity failures | Verify signatures on anything executed or deserialised |
| Logging failures | Signal on auth failure and PII access; never log the secret |
| SSRF | Host allowlist, re-validate after DNS and redirects |
