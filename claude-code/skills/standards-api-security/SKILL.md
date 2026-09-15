---
name: standards-api-security
description: Backend/API security — the OWASP baseline (secrets, injection, authn/JWT, authz, crypto, dependencies, PII, SSRF) plus API-specific concerns — rate limiting, versioning and deprecation, auth scheme selection (API key, OAuth scopes, mTLS), and input-contract enforcement (schema validation, payload/pagination limits, idempotency). Distinct from the rendered-surface scope of standards-web-ui, standards-mobile-ui, and standards-desktop-ui (XSS, clickjacking, overlays, dark patterns). Load before touching an auth path, a query, a secret, an endpoint, or any server-side external boundary.
user-invocable: false
paths: "**/api/**, **/routes/**, **/controllers/**, **/*.proto, **/openapi*, **/swagger*, **/graphql/**, **/*.graphql, **/handlers/**, **/auth/**, **/*auth*, **/middleware/**, **/session*, **/token*, **/crypto/**, **/.env*, **/secrets/**, **/*.sql, **/migrations/**, **/*.tf"
---

# API and backend security

The platform UI skills own the rendered surface — `standards-web-ui`, `standards-mobile-ui`, and `standards-desktop-ui`, each carrying its own Security section for what that platform draws. This skill owns everything else: the OWASP baseline for a server-side boundary (secrets, injection, authn, crypto, dependencies, data handling), plus the decisions specific to an API's shape as a contract between two systems — how many requests it accepts, which version it honours, how a caller proves who it is, and what it refuses to parse. Apply both when a change touches a route that also renders; neither substitutes for the other.

**[review.md](review.md)** carries the review procedure — attack-surface enumeration, the severity bar, the per-vector sweep, and the report shape. Load it for a security review, an attack-vector review, or a dependency check.

## Secrets

- **No hardcoded secrets, tokens, or credentials** in source, config, IaC, or a container layer.
- **Secret values resolve at runtime from a secrets manager.** An env var may name *where* to look; it never carries the value in production.
- **Rotate immediately on suspected exposure**, and treat git history as exposure — a rotated key still reachable in an earlier commit is still leaked.
- **Never log** secrets, tokens, PII, or full bodies containing sensitive fields. Redact at the logger, not at each call site.
- **`.env` is local-dev only** — never committed, always gitignored.

## Rate limiting and quotas

- **Every public or partner endpoint has a rate limit.** No limit is a resource-exhaustion vector regardless of authentication state.
- **Limit per identity, not just per IP** — API key, client ID, or authenticated subject. An IP-only limit is trivially defeated by rotating source addresses; an identity-only limit still needs an unauthenticated floor to stop pre-auth abuse (login, signup, password reset).
- **Return `429` with `Retry-After`**, never a silent drop or a `5xx`. A caller that cannot distinguish "you're overloaded" from "you're throttled" cannot back off correctly.
- **Cost-weight expensive operations.** A bulk export or a full-text search consumes more of the budget than a single-record read — a flat per-request count under-throttles the operations that matter.
- **A quota is enforced server-side, at the gateway or the service, never trusted from a client-supplied header.**

## Versioning and deprecation

- **Version in the URL path or a dedicated header, never silently by payload shape.** A consumer must be able to pin a version and detect a breaking change before it ships, not after.
- **A breaking change is a new version, not a mutation of the old one.** Adding an optional field is non-breaking; renaming, removing, or changing the type of an existing field is.
- **Publish a deprecation window before removal**, with the sunset date carried in a response header (`Sunset`, `Deprecation`) so automated consumers can detect it without reading changelogs.
- **A deprecated version keeps its original security posture** — auth, rate limits, input validation — until it is actually removed. Deprecation is not permission to relax controls early.

## Authentication scheme selection

- **Match the scheme to the trust boundary**, don't default to one everywhere:
  - **API key** — coarse-grained, service-identifies-itself use (server-to-server, low-sensitivity read access). Never used alone to authorize a sensitive write.
  - **OAuth 2.0 / OIDC with scopes** — user-delegated access and third-party integrations. Scope the token to the minimum action set; a token minted for `read:orders` must fail on a write route even if the caller is otherwise trusted.
  - **mTLS** — service-to-service inside a trust boundary where both ends can hold a certificate; the strongest binding when it's operationally feasible.
  - **HMAC request signing** — webhooks and any channel where the caller cannot maintain a session (verify the signature over the full body, reject on clock skew beyond a fixed window, and treat the shared secret as a rotatable credential).
- **An API key is a credential, not a username.** Never accept one in a query string — it lands in access logs, browser history, and referrer headers. Header or body only.
- **Every credential is independently revocable** without rotating every other consumer's — a compromised partner's key must not force a platform-wide key rotation.

## Input contracts

- **Validate every request against a schema before it reaches business logic** — OpenAPI/JSON Schema, protobuf, or GraphQL's own type system. Reject on the boundary; don't let an invalid shape reach a handler that assumes valid input.
- **Cap payload size and pagination limits explicitly.** An unbounded `limit` parameter or an unbounded request body is a resource-exhaustion vector as real as a missing rate limit.
- **Reject unknown fields at a public or partner boundary** — internal and versioned payloads tolerate unknown fields for forward compatibility; a public schema-driven API can silently accept and store an unexpected field if `additionalProperties` isn't pinned to `false`.
- **Idempotency keys on every non-idempotent write a client might retry** (payment, order creation). Without one, a network retry after a timeout can double-execute a mutation the caller believed had failed.
- **A GraphQL endpoint gets query-depth and query-complexity limits** in addition to schema validation — a well-typed but unbounded nested query is still a denial-of-service vector that REST's flat endpoint shape doesn't expose.
- **Never pass unvalidated input** to a query, shell command, path, URL, deserialiser, or template renderer — the injection vector regardless of endpoint shape.

## Authentication and authorization

- **Every endpoint requires auth** unless explicitly documented as public. Deny by default; the public list is the allowlist.
- **Authorization checks live at the service layer**, not only at the gateway, and check ownership per resource on every request.
- **Least privilege** for every service account, IAM role, and user role. Never bind a request body straight onto a model — mass assignment sets `role` and `owner_id` for free.
- **MFA for admin access to production.** Rate-limit and lock out every credential endpoint — login, refresh, reset, MFA.

## Tokens, sessions, and CSRF

- **Pin the expected JWT algorithm server-side.** Never let the token's own `alg` header select the verifier, and never resolve a key from an unvalidated `kid`, `jku`, or `x5u`.
- **Verify `iss`, `aud`, `exp`, and `nbf` on every request.** A decode-without-verify call outside a test is a defect.
- **A claim is only as trustworthy as its signature check** — never authorize on an unverified `role` or `tenant_id`.
- **Short-lived session tokens** with refresh rotation, a server-side revocation path, and identifier rotation on every privilege change.
- **Cookie flags are non-negotiable** — `HttpOnly`, `Secure`, `SameSite` (`Strict` for admin), explicit `Path`.
- **Every mutating route needs anti-CSRF** when authenticated by an ambient credential (a cookie or browser-attached header). `SameSite` mitigates; a session-bound synchroniser token is the control. Never route a state change through `GET`.

## Cryptography

- **Never implement a primitive, mode, or protocol.** Use the platform library at its defaults.
- **AEAD only** (GCM, ChaCha20-Poly1305). No ECB, no unauthenticated CBC. No MD5 or SHA-1 in a security context.
- **Passwords use a memory-hard KDF** — argon2id, scrypt, or bcrypt at a current cost. SHA-family password hashing is critical, salted or not.
- **CSPRNG for every token, salt, IV, and security identifier.** Never reuse a nonce under a fixed key; never hardcode one.
- **Compare secrets in constant time** — tokens, HMACs, reset codes, signatures.

## Dependencies

- **Vulnerability and secret scanning in CI on every change.** The command belongs to the language skill (`standards-go`, `standards-typescript`, `standards-python`); `standards-cicd` defines the gate it blocks.
- **No merging with known high or critical CVEs** without a recorded exception carrying an owner, an expiry, and a compensating control.
- **Lock versions** — no floating ranges in production dependencies, a committed lockfile the install step honours, base images pinned by digest.
- **Review transitive dependencies** when adding a direct one, plus provenance and install-time scripts — a post-install hook runs with CI credentials.

## Data handling

- **Classify before storing** — public, internal, confidential, restricted.
- **PII encrypted at rest and in transit.** TLS 1.3 where both ends support it, 1.2 the floor; never disable certificate verification outside a local fixture.
- **Define retention before storing** any user data.
- **Deletion requests reach backups and derived datasets**, not just the primary store.
- **Never copy production data into a lower environment** without scrubbing PII first.

## SSRF and traversal

- **Any server-side fetch of a client-influenced URL is SSRF until proven otherwise**, including webhooks, PDF/image renderers, and link previews. Allowlist the destination host, re-validate after DNS resolution and every redirect, and deny link-local/loopback/private ranges (cloud instance metadata at `169.254.169.254` is the standard escalation).
- **Never build a filesystem path from input.** Resolve to an absolute path and confirm it is inside the intended root.
- **Never deserialise untrusted data into arbitrary types.** Use a data-only format with a declared schema.

## Errors, configuration, and information exposure

- **An auth failure returns the same response whether the identity doesn't exist or the credential is wrong.** Distinguishing the two lets an attacker enumerate valid identities.
- **Never echo the schema-validation internals of a rejected payload back to an untrusted caller** beyond what's needed to fix the request — a stack trace, an internal field name, or a database constraint message belongs in the server log, not the response body.
- **CORS is an allowlist of explicit origins for a browser-facing API**, never a wildcard once credentials (cookies, `Authorization` headers) are in play — a wildcard `Access-Control-Allow-Origin` paired with `Access-Control-Allow-Credentials: true` is a direct account-takeover vector.
- **Reject wildcard CORS with credentials**, `0.0.0.0/0` ingress, public storage buckets, and permissive IAM (`*` action or resource).
- **Confirm debug surfaces are off in production** — stack traces to clients, verbose errors, profiling endpoints, GraphQL introspection, directory listing, default credentials.

## Agent and LLM surfaces

- **Treat model output as untrusted input**, and any content the model reads as attacker-controlled — prompts, tool arguments, retrieved documents, web fetches.
- **Never let retrieved or user text select a tool or authorize an action.** Authorization is decided by the caller's identity, before the model runs.
- **Validate tool arguments server-side** exactly as an HTTP body — the model is a client, not a trusted component.
- **Scope credentials per tool, at least privilege**, and never place a secret in a prompt or system message.

## Incident response

Isolate, assess blast radius, rotate every affected credential, record the timeline, then run a blameless post-mortem with a concrete prevention step. **Notification clocks are legal, not technical** — regulator, contract, and card-scheme deadlines all differ; read the obligation that applies, never assume a window.

## OWASP Top 10 mitigations

Categories by name, no ranks — OWASP re-ranks and merges between editions. Read the current list if a rank matters. Injection covers XSS's data path (rendering/encoding specifics live in `standards-web-ui`); SSRF has merged into access control in some editions.

| Category | Mitigation |
|---|---|
| Broken access control | Authorization at the service layer, per-resource ownership, tenant scoping in the query |
| Injection | Parameter binding for queries, argument arrays for processes |
| Cryptographic failures | AEAD at rest, TLS in transit, memory-hard password KDF, constant-time comparison |
| Insecure design | Threat-model the boundary before implementing it; enumerate actors and assets |
| Security misconfiguration | Deny by default; no wildcard CORS, no `0.0.0.0/0` ingress, no debug surface in prod |
| Vulnerable and outdated components | CI scanning, committed lockfiles, digest-pinned images, prompt patching |
| Identification and auth failures | Pinned JWT algorithm, verified `iss`/`aud`/`exp`, short-lived tokens, rate-limited credential paths |
| Software and data integrity failures | Verify signatures on anything executed or deserialised; audit install-time scripts |
| Logging and monitoring failures | Signal on auth failure, authz denial, and PII access; never log the secrets themselves |
| SSRF | Allowlist outbound hosts, re-validate after DNS and each redirect, deny link-local and private ranges |

## Quick-reference

| Concern | Control |
|---|---|
| Secrets | Resolved at runtime from a secrets manager, never hardcoded or logged |
| Rate limiting | Per-identity limit, `429` + `Retry-After`, cost-weighted for expensive operations |
| Versioning | Path or header version, breaking change = new version, `Sunset`/`Deprecation` headers |
| Auth scheme | API key (coarse), OAuth scopes (delegated), mTLS (service trust boundary), HMAC (webhooks) |
| Input contract | Schema validation at the boundary, capped payload/pagination, `additionalProperties: false` |
| Idempotency | Idempotency key on every retryable write |
| GraphQL | Query-depth and complexity limits alongside schema validation |
| Tokens/CSRF | Pinned JWT alg, verified claims, `HttpOnly`/`Secure`/`SameSite` cookies, anti-CSRF on mutations |
| Cryptography | AEAD only, memory-hard password KDF, CSPRNG, constant-time comparison |
| Dependencies | CI vulnerability scan, no unexcepted high/critical CVEs, committed lockfile |
| Data handling | Classified before storing, encrypted at rest/transit, retention defined, PII scrubbed below prod |
| Errors | Identical response for unknown identity vs. wrong credential; no internals in the body |
| CORS | Explicit origin allowlist whenever credentials are in play, never a wildcard |
