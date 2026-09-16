# API design

The security standard owns whether a request is allowed through. This one owns the shape of the contract.

## Resources and URLs
- Nouns for resource paths, plural collection names: `POST /orders`, `GET /orders/{id}`.
- Nest one level for true ownership (`/orders/{id}/items`); a resource with its own lifecycle gets a top-level collection and a filter param.
- Path segments are stable identifiers, never a mutable field. Query params for filtering, sorting, pagination.
- Lowercase, hyphen-separated multi-word segments, one convention across the API including query param names.

## Schemas
- One field-name casing across the whole API. One envelope shape everywhere, either bare resources or `{ "data": ..., "meta": ... }`.
- Dates are ISO 8601 with an explicit offset. Null and absent are distinguishable where a client needs the distinction.
- A create returns the full created representation including server-assigned fields.
- Additive fields are backward compatible, and the schema declares the policy explicitly.

## Pagination, filtering, sorting
- Every collection that can grow is paginated: cursor-based for high-volume or frequently mutated collections, offset only for small stable ones.
- Pagination params and metadata are uniform across every collection endpoint. A default page size is explicit.
- Filter params name the field directly (`status=shipped`, `created_after=...`). Sort takes a field with one direction convention (`sort=-created_at`).

## Errors
- One error envelope for the whole API: a stable machine-readable code, a human message, and a per-field breakdown for validation.
- Status reflects the category: `400` malformed, `401` no credentials, `403` not authorized, `404` absent (or indistinguishable from `403` where existence is sensitive), `409` state conflict, `422` semantically invalid, `429` throttled, `5xx` server fault only.
- Validation errors list every failing field in one response.

## Idempotency
- A non-idempotent write a client might retry accepts an idempotency key header. The contract states which endpoints accept it, how long a key lives, and that a replay returns the original result.
- `PUT` on a fully specified resource is idempotent by HTTP semantics; reserve the key for `POST` and anything the method does not already guarantee.

## Versioning
- Choose one scheme at design time and hold it: a URL segment (`/v1/orders`) or a dedicated header (`Api-Version: 2024-01-01`). Never by payload shape, never mixed.
- A URL scheme bumps on any breaking change. Deprecation mechanics belong to the security standard.

## Hypermedia
- Links are opt-in per API. Where used, they live in one named envelope location with a stable relation vocabulary, and a link's presence signals that the action is currently valid. Do not half-adopt: a lone `self` link is dead weight.
