# SQL / Database Standards

Base standards for SQL databases at Komodo. Written with PostgreSQL as the target, with notes for DynamoDB where patterns differ significantly. Projects using other databases should extend this standard.

---

## 1. Table naming

- Plural snake_case: `users`, `order_items`, `payment_methods`
- Junction/join tables: both table names combined, alphabetical order: `order_products`, `role_permissions`
- No prefixes (no `tbl_`, no schema prefix in table name)
- Abbreviations only when universal: `id`, `url`, `uuid` — otherwise spell it out

---

## 2. Column conventions

- `id` — primary key, type `uuid` by default (string in DynamoDB). Never sequential integers for external-facing IDs.
- `created_at` — `timestamptz`, set by database default (`NOW()`), never by application
- `updated_at` — `timestamptz`, updated via trigger or ORM hook on every write
- `deleted_at` — `timestamptz`, nullable — present on any table that uses soft deletes (see §6)
- Boolean columns: prefix with `is_` or `has_`: `is_active`, `has_mfa`, `is_verified`
- Foreign keys: `<referenced_table_singular>_id`: `user_id`, `order_id`, `product_id`
- Avoid abbreviations in column names: `description` not `desc`, `quantity` not `qty`

---

## 3. Primary keys

- Use `uuid` (v4 or v7) as the default primary key type — safe for distributed systems, external APIs, and JS clients
- `uuid_generate_v4()` (PostgreSQL) or application-generated UUID
- Do not expose sequential integer IDs in external-facing APIs (enumeration risk)
- Composite primary keys only for pure junction tables with no lifecycle of their own

---

## 4. Indexes

- Always index foreign key columns (PostgreSQL does not do this automatically)
- Index columns that appear in `WHERE`, `ORDER BY`, or `JOIN ON` clauses in common queries
- Partial indexes for filtered queries on large tables: `WHERE deleted_at IS NULL`
- Unique indexes to enforce uniqueness constraints at the database level, not just the application
- Run `EXPLAIN ANALYZE` before adding an index — don't index speculatively
- Review index bloat quarterly on high-write tables

```sql
-- Foreign key index
CREATE INDEX idx_orders_user_id ON orders(user_id);

-- Partial index for soft-delete pattern
CREATE INDEX idx_users_active ON users(email) WHERE deleted_at IS NULL;

-- Composite index for common filter+sort
CREATE INDEX idx_orders_user_created ON orders(user_id, created_at DESC);
```

---

## 5. Migrations

- Every schema change is a migration — no manual schema edits in any environment
- Migration files are immutable once merged — never edit a migration that has run in any environment
- Naming: `<sequence>_<description>.up.sql` and `.down.sql`
- Description format: `create_users_table`, `add_stripe_customer_id_to_users`, `add_index_on_orders_user_id`
- Every `up` migration must have a corresponding `down` migration that fully reverses it
- Test the down migration before merging — a rollback that doesn't work is not a rollback
- Large table migrations (millions of rows): use `ADD COLUMN ... DEFAULT NULL` then backfill separately; never lock the table

---

## 6. Soft deletes

Use soft deletes (`deleted_at timestamptz`) for any entity where:
- History or audit trail matters
- The record may be referenced by other tables
- The delete may need to be undone

```sql
-- Soft delete pattern
ALTER TABLE users ADD COLUMN deleted_at timestamptz;
CREATE INDEX idx_users_not_deleted ON users(id) WHERE deleted_at IS NULL;

-- Query pattern — always filter in application queries
SELECT * FROM users WHERE deleted_at IS NULL;
```

Use hard deletes only for ephemeral data (sessions, temp records) or when regulatory data deletion is required — and even then, tombstone the record first.

---

## 7. Nullability

- Default to `NOT NULL` — null should mean "this value intentionally does not exist," not "we forgot to set it"
- Use `DEFAULT` values where appropriate rather than nullable columns for optional fields with a known fallback
- Nullable foreign keys are acceptable when a relationship is genuinely optional
- Document why a column is nullable if it's not obvious

---

## 8. Query conventions

- Never `SELECT *` in application code — name the columns explicitly
- Parameterized queries only — no string-concatenated SQL, ever (injection risk)
- `LIMIT` all queries that could return unbounded result sets
- Use transactions for multi-step writes that must be atomic
- Application code does not build schema — schema changes go through migrations

---

## 9. DynamoDB notes

DynamoDB differs significantly from relational databases. When using DynamoDB:

- Design the access pattern first, then the table — not the other way around
- Prefer single-table design for related entities that are always accessed together
- Partition key should distribute load evenly — avoid hot partitions
- Sort key enables range queries within a partition — use it intentionally
- No joins — denormalize where needed, and accept the write duplication cost
- Use TTL for ephemeral records (sessions, short-lived tokens) instead of delete operations
- GSIs are expensive — add them intentionally, not speculatively
- Transactions are available but expensive — use sparingly

---

## 10. Security

- Database users follow least-privilege: the application user can only do what the application needs (no DDL in production)
- Separate read and write users where read-heavy workloads justify it
- Connection strings and credentials are never in source code — secrets manager only
- Enable connection encryption (TLS) between application and database in all non-local environments
- Audit log access to sensitive tables (payments, PII) where the platform supports it
