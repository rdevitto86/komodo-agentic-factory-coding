---
name: standards-api-security
description: API-specific security — rate limiting, versioning and deprecation, auth scheme selection (API key, OAuth scopes, mTLS), and input-contract enforcement (schema validation, payload/pagination limits, idempotency). Distinct from standards-security's general OWASP baseline. Load before designing an endpoint, choosing an auth scheme for a service-to-service or public API, or setting a rate limit, quota, or version policy.
user-invocable: false
paths: "**/api/**, **/routes/**, **/controllers/**, **/*.proto, **/openapi*, **/swagger*, **/graphql/**, **/*.graphql, **/handlers/**"
---

# API security

`standards-security` owns the OWASP baseline — injection, XSS, authn/JWT mechanics, crypto. This skill is the layer above it: the decisions specific to an API's shape as a contract between two systems — how many requests it accepts, which version it honours, how a caller proves who it is, and what it refuses to parse. Apply both; neither substitutes for the other.

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
- **Reject unknown fields at a public or partner boundary** — the general rule from `standards-security`, restated because a schema-driven API can silently accept and store an unexpected field if additionalProperties isn't pinned to `false`.
- **Idempotency keys on every non-idempotent write a client might retry** (payment, order creation). Without one, a network retry after a timeout can double-execute a mutation the caller believed had failed.
- **A GraphQL endpoint gets query-depth and query-complexity limits** in addition to schema validation — a well-typed but unbounded nested query is still a denial-of-service vector that REST's flat endpoint shape doesn't expose.

## Errors and information exposure

- **An auth failure returns the same response whether the identity doesn't exist or the credential is wrong.** Distinguishing the two lets an attacker enumerate valid identities.
- **Never echo the schema-validation internals of a rejected payload back to an untrusted caller** beyond what's needed to fix the request — a stack trace, an internal field name, or a database constraint message belongs in the server log, not the response body.
- **CORS is an allowlist of explicit origins for a browser-facing API**, never a wildcard once credentials (cookies, `Authorization` headers) are in play — a wildcard `Access-Control-Allow-Origin` paired with `Access-Control-Allow-Credentials: true` is a direct account-takeover vector, and some runtimes reject the combination outright.

## Quick-reference

| Concern | Control |
|---|---|
| Rate limiting | Per-identity limit, `429` + `Retry-After`, cost-weighted for expensive operations |
| Versioning | Path or header version, breaking change = new version, `Sunset`/`Deprecation` headers |
| Auth scheme | API key (coarse), OAuth scopes (delegated), mTLS (service trust boundary), HMAC (webhooks) |
| Input contract | Schema validation at the boundary, capped payload/pagination, `additionalProperties: false` |
| Idempotency | Idempotency key on every retryable write |
| GraphQL | Query-depth and complexity limits alongside schema validation |
| Errors | Identical response for unknown identity vs. wrong credential; no internals in the body |
| CORS | Explicit origin allowlist whenever credentials are in play, never a wildcard |
