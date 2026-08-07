---
name: sql
description: PostgreSQL and DynamoDB — naming, columns, migrations, indexes, security. Load before reading or writing any .sql file or anything under migrations/ or migrate/.
user-invocable: false
---

# SQL

PostgreSQL-targeted with DynamoDB notes. Generic hygiene — parameterised queries, `EXPLAIN ANALYZE`, `NOT NULL` defaults, no `SELECT *` — is assumed.

## Naming

- **Tables**: plural `snake_case` (`users`, `order_items`). Junction tables combine both names alphabetically (`order_products`). No `tbl_` prefixes.
- **Columns**: `snake_case`, spelled out (`description`, not `desc`). Booleans take `is_` / `has_`. Foreign keys are `<referenced_singular>_id`.

## Standard columns

- **`id`** — `uuid` primary key (v4/v7; string in DynamoDB). **Never expose sequential integer IDs externally** — enumeration risk. Composite PKs only for pure junction tables.
- **`created_at` / `updated_at` / `deleted_at`** — `timestamptz`. `created_at` comes from a DB default (`NOW()`), never the app. `updated_at` via trigger or ORM hook on every write. `deleted_at` nullable on soft-delete tables.

## Migrations

- **Every schema change is a migration.** No manual schema edits in any environment; app code never builds schema.
- **Files are immutable once merged.** Name them `<sequence>_<description>.up.sql` / `.down.sql`, with a description like `add_stripe_customer_id_to_users`.
- **Every `up` has a tested reversing `down`.**
- **Large tables**: `ADD COLUMN ... DEFAULT NULL`, then backfill separately. Never lock the table.

## Soft deletes

Use `deleted_at timestamptz` when history matters, the record may be referenced, or a delete may need undoing. Always filter `WHERE deleted_at IS NULL`, backed by a partial index. Hard-delete only ephemeral data or regulatory deletion — tombstone first.

## Indexes and queries

- **Always index foreign keys** — Postgres does not do it for you.
- **Partial indexes** for soft-delete filters (`WHERE deleted_at IS NULL`).
- **Enforce uniqueness at the database** with a unique index, not only in the app.
- **`EXPLAIN ANALYZE` before adding an index.** No speculative indexes.
- **`LIMIT` anything** that could return unbounded rows.

## DynamoDB

Access pattern first, then table. Single-table design for entities accessed together. The partition key must distribute load — no hot partitions. Sort key for intentional range queries. No joins: denormalise and accept write duplication. TTL for ephemeral records. GSIs and transactions are expensive; use them deliberately.

## Security

Least-privilege database users, no DDL in production. Separate read and write users for read-heavy workloads. Credentials live in a secrets manager, never in source. TLS everywhere outside local. Audit access to payments and PII tables where supported.
