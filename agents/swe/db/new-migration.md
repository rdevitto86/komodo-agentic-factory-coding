# Skill: /new-migration

Create a new database schema migration for a Komodo service.

## Usage

```
/new-migration <service> <description> [--up-only]
```

- `<service>` — Target service name without prefix, e.g. `cart`, `loyalty`. Full service path is `apis/komodo-<service>-api/`.
- `<description>` — Short snake_case description of what the migration does, e.g. `add_orders_table`, `add_status_index_to_orders`. Used in the filename.
- `--up-only` — Generate only an `up` migration (use for additive changes where rollback is unnecessary or handled separately).

**Must be run from inside the service directory** (`apis/komodo-<service>-api/`).

---

## Before generating anything

1. Read the service's `go.mod` to confirm the migration tool in use (look for `golang-migrate`, `goose`, or similar).
2. List `db/migrations/` (or `migrations/`) to find the highest existing sequence number. The new migration filename must use the next number.
3. Read the most recent migration file to understand the naming convention and SQL dialect in use.
4. Read `db/schema.sql` or equivalent if it exists — understand the current table structure before altering it.

---

## Migration files

### `db/migrations/<sequence>_<description>.up.sql`

```sql
-- Migration: <description>
-- Created: <date>

-- TODO: write forward migration SQL
-- Example:
-- CREATE TABLE example (
--     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
--     created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
--     updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
-- );
```

### `db/migrations/<sequence>_<description>.down.sql`

(Omit if `--up-only`.)

```sql
-- Rollback: <description>

-- TODO: write rollback SQL
-- Example:
-- DROP TABLE IF EXISTS example;
```

---

## Naming conventions

| Change type | Example description |
|-------------|-------------------|
| New table | `create_orders_table` |
| Add column | `add_status_to_orders` |
| Add index | `add_status_index_to_orders` |
| Remove column | `drop_legacy_flag_from_orders` |
| Rename column | `rename_ref_to_external_id_in_orders` |
| Add constraint | `add_unique_constraint_to_orders_ref` |

---

## After generating

1. Print the full filenames created and the next sequence number used.
2. Remind the developer to:
   - Review the SQL for correctness in the target dialect (PostgreSQL unless noted otherwise).
   - Test the migration locally with `make migrate-up` (or equivalent) before committing.
   - Test the rollback with `make migrate-down` unless `--up-only` was used.
   - For large tables, evaluate whether the migration needs `CONCURRENTLY` index creation or batched updates to avoid locking.
   - Update `db/schema.sql` if the project maintains a consolidated schema snapshot.
