---
name: standards-api-design
description: API shape and contract conventions — resource naming and URL structure, request/response schema conventions, pagination/filtering/sorting, error response shape and status codes, idempotency-key design, versioning-scheme choice, and HATEOAS/hypermedia. Distinct from standards-api-security's enforcement of those same contracts (rate limiting, auth scheme, security-purpose input validation, deprecation/sunset headers). Load before designing or reviewing a route, an OpenAPI/proto/GraphQL schema, or a response shape.
user-invocable: false
paths: "**/api/**, **/routes/**, **/controllers/**, **/*.proto, **/openapi*, **/swagger*, **/graphql/**, **/*.graphql, **/handlers/**"
---

# API design

`standards-api-security` owns whether a request is allowed through and how it's enforced — auth scheme, rate limits, security-purpose validation, deprecation mechanics. This skill owns the shape of the contract itself: what a resource is called, what a response looks like, how a client asks for a page or a filter, what an error carries, and which versioning scheme a new API starts with. Apply both when a change touches a route — neither substitutes for the other.

## Resource naming and URL structure

- **Nouns, not verbs, for resource paths** — `POST /orders`, not `POST /createOrder`. The HTTP method carries the verb.
- **Plural collection names** — `/orders`, `/orders/{id}`, not `/order/{id}`.
- **Nest only one level for true ownership**, `/orders/{id}/items`; a resource with an independent lifecycle gets its own top-level collection with a filter query param instead of unbounded nesting (`/items?order_id={id}`, not `/orders/{id}/items/{item_id}/shipments/{shipment_id}`).
- **Path segments are stable identifiers** — a slug or numeric/UUID ID, never a field that changes over the resource's life (a display name, a mutable email).
- **Query params for filtering, sorting, and pagination; path segments for resource identity.** `/orders?status=shipped`, never `/orders/shipped`.
- **Lowercase, hyphen-separated multi-word segments** — `/shipping-addresses`, not `/shippingAddresses` or `/shipping_addresses`. Pick one convention per API and hold it everywhere, including query param names.

## Request and response schema conventions

- **One consistent casing for field names across the whole API** — `snake_case` or `camelCase`, never mixed within a single response body.
- **Envelope shape is fixed and documented** — either every response is a bare resource/array, or every response wraps in a consistent envelope (`{ "data": ..., "meta": ... }`). Never switch shape between endpoints in the same API.
- **Dates and times are ISO 8601 with an explicit UTC offset**, never a bare epoch int or a locale-formatted string.
- **Null and absent are distinguishable where the client needs the distinction** — a `PATCH` that means "clear this field" needs a way to say so that differs from "field not included."
- **A resource's create response returns the full created representation**, including server-assigned fields (`id`, timestamps), not just an ID.
- **Additive fields are backward compatible; the schema declares this explicitly** (`additionalProperties` policy, protobuf field numbering discipline, GraphQL nullable-by-default additions) so a consumer knows what it may ignore.

## Pagination, filtering, and sorting

- **Every collection endpoint that can grow unbounded is paginated** — cursor-based for high-volume or frequently-mutated collections (stable under concurrent writes), offset-based only for small, rarely-changing ones.
- **Pagination metadata is uniform across every collection endpoint** — the same param names (`limit`/`cursor` or `limit`/`offset`), the same response shape for `next`/`has_more`/`total`, no per-endpoint variation.
- **A default page size is set explicitly** and documented; never rely on the client always supplying one. (The upper bound enforced against that default is `standards-api-security`'s concern, not this skill's.)
- **Filter params name the field being filtered directly** (`status=shipped`, `created_after=...`), not an opaque query-language string, unless the API is explicitly a query-language API (GraphQL, a documented filter DSL).
- **Sort param takes a field name with an explicit direction convention** — a `-` prefix for descending (`sort=-created_at`) or a paired `sort`/`order` param — picked once and applied everywhere.

## Error response shape and status codes

- **One error envelope shape for the whole API**, carrying at minimum a machine-readable code, a human-readable message, and (for validation failures) a per-field breakdown. Never return a bare string or an ad hoc shape that differs by endpoint.
- **Status code reflects the failure category, not just "something went wrong":**
  - `400` — malformed request the server can't parse or that fails schema validation.
  - `401` — missing or invalid credentials.
  - `403` — authenticated but not authorized for this resource/action.
  - `404` — resource doesn't exist (or, where existence itself is sensitive, indistinguishable from `403` — a security decision, see `standards-api-security`).
  - `409` — conflicts with current state (duplicate create, version mismatch on update).
  - `422` — well-formed but semantically invalid (fails a business rule the schema can't express).
  - `429` — rate-limited (mechanics owned by `standards-api-security`).
  - `5xx` — server fault; never used for a client mistake.
- **The error code is a stable string, not the HTTP status repeated** — `resource_not_found`, `validation_failed` — so a client can branch on it without parsing prose.
- **Validation errors enumerate every failing field in one response**, not one-at-a-time — a client shouldn't have to resubmit repeatedly to discover the next problem.

## Idempotency-key design

- **A non-idempotent write a client might reasonably retry (payment, order creation) accepts a client-supplied idempotency key** in a request header, and the API's contract documents its scope: which endpoints accept it, how long a key is honoured, and what response the second call with the same key returns (the original result, not a fresh execution or an error).
- **The design decision here is the contract shape** — header name, key format, replay-window duration, and response-on-replay semantics. Enforcing that a client actually supplied one, storing it safely, and treating it as a security-relevant control against retry-driven double-execution is `standards-api-security`'s concern; the two are complementary, not duplicated.
- **A `PUT` on a fully-specified resource is naturally idempotent by HTTP semantics** and doesn't need a separate key; reserve the key mechanism for `POST` and any operation the method's own semantics don't already guarantee.

## Versioning strategy

- **Choose one versioning scheme when the API is designed, and hold it everywhere**: a URL path segment (`/v1/orders`) is the most visible and cache-friendly choice; a dedicated header (`Api-Version: 2024-01-01`) keeps the URL stable across versions at the cost of being less discoverable. Never version by payload shape alone, and never mix schemes within one API.
- **A URL-path scheme bumps on any breaking change**; a header scheme can use either a semantic version or a date-based version — pick the one that matches how the API's consumers are expected to pin.
- **This skill owns the initial choice of scheme; `standards-api-security` owns what happens to an existing version once it's superseded** — the deprecation window, the `Sunset`/`Deprecation` response headers, and the security posture a deprecated version must keep until removal. Read that skill before retiring a version; read this one before introducing the first.

## HATEOAS and hypermedia

- **Hypermedia links are opt-in per API, not a default expectation** — most REST APIs ship well without them; adopt link-based navigation only where clients genuinely need to discover available actions dynamically (a workflow with state-dependent transitions is the strongest case).
- **Where used, links live in a consistent, named location in the envelope** (`_links`, `links`) with a stable relation-name vocabulary (`self`, `next`, `prev`, action-specific rels) — never inline ad hoc URLs scattered through arbitrary fields.
- **A link's presence or absence communicates state** — an action's link appears only when the action is currently valid for that resource (no `cancel` link on an already-shipped order), so a client can drive its UI off link presence instead of duplicating the state machine.
- **Don't half-adopt it.** A response with one `self` link and nothing else is dead weight, not hypermedia — either commit to links driving navigable state, or drop them and document actions out of band (OpenAPI, GraphQL schema).

## Quick-reference

| Concern | Convention |
|---|---|
| Resource naming | Plural nouns, hyphen-separated, one nesting level for true ownership |
| Response shape | One casing convention, one envelope shape, ISO 8601 dates with UTC offset |
| Pagination | Cursor-based for high-volume/mutating collections, uniform param names everywhere |
| Filtering/sorting | Direct field-name params, one sort-direction convention |
| Errors | One envelope shape, stable machine-readable code, status reflects failure category |
| Idempotency | Client-supplied key on retryable unsafe writes; contract defines replay-window and replay response |
| Versioning | One scheme (URL path or header) chosen at design time, held everywhere |
| Hypermedia | Opt-in, consistent link location and relation vocabulary, link presence signals valid actions |
