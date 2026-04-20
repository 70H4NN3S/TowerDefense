# Follow-ups

Items noticed during development that are out of scope for the current task.
Format: `## <slug>` then bullet points with author, date, and context.

---

## uuid-library

- **Author:** claude, 2026-04-20
- **Context:** Phase 1 uses `crypto/rand` hex strings for request IDs to avoid
  adding a dependency. Phase 2 (user registration) will need proper UUID v4
  generation (primary keys). `github.com/google/uuid` is the canonical choice;
  it is not on the current allowed-dependency list. Discuss before Phase 2 begins.

## migration-multi-statement

- **Author:** claude, 2026-04-20
- **Context:** The migration runner passes the entire SQL file to a single
  `ExecContext` call. `pgx/stdlib` handles multi-statement queries via the
  PostgreSQL simple-query protocol. If a migration ever needs explicit
  statement-level control (e.g. DDL + DML in the same file), the runner should
  be extended to split on `;` or use pgx's extended-query protocol. Not needed
  until we have a case that requires it.
