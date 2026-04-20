# Database

PostgreSQL is the single source of truth. All schema changes go through versioned migrations; no ad-hoc `ALTER TABLE` in production.

## Schema conventions

- **Primary keys:** UUID v4 unless there's a compelling reason (e.g. `schema_migrations.version` is an integer). Generate UUIDs application-side; do not rely on `gen_random_uuid()` being installed.
- **Timestamps:** every table has `created_at timestamptz NOT NULL DEFAULT now()` and, where mutation is allowed, `updated_at timestamptz NOT NULL DEFAULT now()`.
- **Soft deletes:** avoided unless product-required. When required, use a `deleted_at timestamptz` column with a partial index `WHERE deleted_at IS NULL` on the lookup columns.
- **Naming:**
  - Tables: plural, snake_case: `users`, `owned_towers`, `alliance_members`.
  - Columns: snake_case, singular: `user_id`, `trophies`, `created_at`.
  - Join tables: combine the two singular names alphabetically: `alliance_user` is fine, but prefer meaningful names like `alliance_members`.
  - Indexes: `idx_<table>_<columns>`. Unique indexes: `uq_<table>_<columns>`. Partial indexes append `_where_<predicate>`.
  - Foreign keys: `fk_<table>_<referenced_table>`.
- **Enums:** prefer `text` columns with a `CHECK (col IN ('a','b','c'))` constraint over PostgreSQL enum types. Enum types are painful to evolve.
- **Money / resources:** use `bigint`, never floats. Store integer quantities (gold, diamonds).

## Migrations

- Every schema change has a pair of files:
  ```
  internal/db/migrations/NNNN_description.up.sql
  internal/db/migrations/NNNN_description.down.sql
  ```
  Numbered `0001`, `0002`, … zero-padded to 4 digits. Never renumber.
- `.up.sql` and `.down.sql` are **both required** and both tested locally (`make migrate-up && make migrate-down && make migrate-up`).
- Keep migrations **narrow**: one logical change per pair. "Add a column" and "backfill data" are two separate migrations.
- **Backfills:** for any table that might already have rows, provide a data migration in a separate numbered file. Never assume an empty table.
- Migrations are **never** edited after they've been committed to `main`. Roll forward with a new migration.
- Migrations must be idempotent when feasible: `CREATE TABLE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`, guard against reruns.

## Queries

- Use `database/sql` with the `pgx/stdlib` driver or `pgxpool` directly. **No ORMs. No query builders.** Plain SQL in `.sql` files or in small `const` strings next to the function that uses them.
- Parameters always use placeholders (`$1`, `$2`, …). **Never** string-concatenate user input into SQL. This is a hard rule.
- `SELECT *` is forbidden outside of migrations and ad-hoc debugging. Name every column.
- Every query has a `context.Context` as its first argument and respects cancellation.
- Row scanning uses named fields, not positional decoding by index into a slice.

### Transactions

- Multi-statement writes go through a transaction.
- Transactions are scoped to a single function and passed down via a `Querier` interface that accepts both `*pgxpool.Pool` and `pgx.Tx`:

  ```go
  type Querier interface {
      Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
      QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
      Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
  }
  ```

- Use `SERIALIZABLE` or `REPEATABLE READ` isolation for money flows (spending gold/diamonds). Use row-level locks (`SELECT ... FOR UPDATE`) when you need to coordinate within a single row.

## Indexes

- Every foreign key has an index on the referencing side.
- Every column used in a `WHERE`, `JOIN`, or `ORDER BY` on a hot path has an index.
- Consider a covering index when a query reads many rows and only a few columns.
- Review `EXPLAIN (ANALYZE, BUFFERS)` on any new query that hits a table larger than a few thousand rows in expected load.

## Constraints

- Foreign keys are declared with `ON DELETE` explicitly (`CASCADE`, `RESTRICT`, `SET NULL`). The default is not acceptable.
- `CHECK` constraints are declared for any invariant the application relies on: non-negative amounts, valid enum-as-text columns, bounded integers (e.g. tower level 1–10).
- `NOT NULL` by default. Nullable columns must have a reason that's documented near the declaration.

## Test data and fixtures

- Integration tests spin up a fresh schema in a disposable database (`pg_tmp` or a throwaway container).
- Fixtures are created via the same service functions that production uses, not raw `INSERT`s. This catches schema drift.
- Every test that writes to the DB either runs in a transaction that's rolled back at teardown, or truncates its touched tables.

## Running migrations

- Locally: `make migrate-up`, `make migrate-down`, `make migrate-redo`.
- In production: `cmd/migrate` runs as a one-shot container/init step; the server refuses to start if the applied version is behind the embedded expected version.

## What not to do

- No stored procedures, triggers, or PL/pgSQL unless a strong case is made and reviewed.
- No LISTEN/NOTIFY for the core multiplayer loop — we use WebSocket + in-process hub, not Postgres pub/sub.
- No caching layer (Redis) until profiling says we need one.