# SQL / Database Standards

PostgreSQL-targeted, with DynamoDB notes. Generic SQL hygiene (index mechanics, `EXPLAIN ANALYZE`, parameterized queries, transactions, `NOT NULL` defaults, no `SELECT *`) is assumed — this covers Komodo conventions and the non-obvious points. Other databases extend this.

---

## 1. Naming

- Tables: plural `snake_case` (`users`, `order_items`); junction tables combine both names alphabetically (`order_products`, `role_permissions`); no `tbl_`/schema prefixes.
- Columns: `snake_case`, spelled out (`description` not `desc`); boolean prefix `is_`/`has_`; foreign keys `<referenced_singular>_id`.

---

## 2. Standard columns

- `id` — `uuid` PK by default (v4/v7; string in DynamoDB). Never expose sequential integer IDs externally (enumeration risk). Composite PKs only for pure junction tables.
- `created_at` / `updated_at` / `deleted_at` — `timestamptz`. `created_at` set by DB default (`NOW()`), never the app; `updated_at` via trigger/ORM hook on every write; `deleted_at` nullable on soft-delete tables.

---

## 3. Migrations

- Every schema change is a migration — no manual schema edits in any environment; app code never builds schema.
- Files immutable once merged. Naming `<sequence>_<description>.up.sql` / `.down.sql`, description like `add_stripe_customer_id_to_users`. Every `up` has a tested reversing `down`.
- Large tables (millions of rows): `ADD COLUMN ... DEFAULT NULL` then backfill separately — never lock the table.

---

## 4. Soft deletes

Use `deleted_at timestamptz` when history/audit matters, the record may be referenced, or deletes may be undone. Always filter `WHERE deleted_at IS NULL` in app queries, backed by a partial index. Hard-delete only ephemeral data (sessions, temp) or regulatory deletion — tombstone first.

---

## 5. Indexes & queries (Komodo specifics)

- Always index foreign keys (Postgres doesn't). Partial indexes for soft-delete filters (`WHERE deleted_at IS NULL`). Enforce uniqueness with unique indexes at the DB, not just the app. `EXPLAIN ANALYZE` before adding — no speculative indexes.
- `LIMIT` any query that could return unbounded rows.

---

## 6. DynamoDB notes

Access pattern first, then table. Single-table design for entities accessed together. Partition key must distribute load (no hot partitions); sort key for intentional range queries. No joins — denormalize and accept write duplication. TTL for ephemeral records. GSIs and transactions are expensive — use intentionally.

---

## 7. Security

Least-privilege DB users (no DDL in production); separate read/write users for read-heavy workloads. Credentials in secrets manager only, never source. TLS in all non-local environments. Audit access to sensitive tables (payments, PII) where supported.
