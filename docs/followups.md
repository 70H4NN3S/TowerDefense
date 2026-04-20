# Follow-ups

Items noticed during development that are out of scope for the current task.
Format: `## <slug>` then bullet points with author, date, and context.

---

## uuid-library

- **Author:** claude, 2026-04-20
- **Context:** Resolved in Phase 2. Implemented `internal/uuid/` (~50 lines) using
  `crypto/rand` + `encoding/hex` with `type UUID string`. No external dependency
  required. SQL queries use `id::text` and `$1::uuid` casts for pgx compatibility.
  Reconsider if a richer UUID API (e.g. version 5 deterministic UUIDs) is ever needed.

## migration-multi-statement

- **Author:** claude, 2026-04-20
- **Context:** The migration runner passes the entire SQL file to a single
  `ExecContext` call. `pgx/stdlib` handles multi-statement queries via the
  PostgreSQL simple-query protocol. If a migration ever needs explicit
  statement-level control (e.g. DDL + DML in the same file), the runner should
  be extended to split on `;` or use pgx's extended-query protocol. Not needed
  until we have a case that requires it.
