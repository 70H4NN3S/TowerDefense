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

## refresh-token-rotation

- **Author:** claude, 2026-04-20
- **Context:** `auth.Service.Refresh` issues a new token pair but does NOT invalidate
  the old refresh token's JTI. A stolen refresh token can therefore be replayed
  indefinitely until expiry (30 days). Fix requires a `refresh_tokens` table (or
  a `jti_revocations` deny-list), marking the consumed JTI as used in the same
  transaction that issues the new pair. Already covered by the `jti` field in the
  Claims struct; only the store logic and migration are missing.
  Priority: high — ship before any public release.

## multiplayer-elo-trophy-loss

- **Author:** claude, 2026-04-21
- **Context:** `session.endMatch` currently awards +25 trophies on co-op victory and
  0 on defeat. A proper ELO-lite system should also deduct trophies on defeat (e.g.
  −15) and scale gains by the expected win probability based on the matchmaking bucket
  difference. The `sessionPlayer.trophies` field is already captured at match start to
  support this calculation. Implement once the leaderboard feature lands (Phase 8+).
  Priority: medium — ship before ranked seasons begin.

## migration-multi-statement

- **Author:** claude, 2026-04-20
- **Context:** The migration runner passes the entire SQL file to a single
  `ExecContext` call. `pgx/stdlib` handles multi-statement queries via the
  PostgreSQL simple-query protocol. If a migration ever needs explicit
  statement-level control (e.g. DDL + DML in the same file), the runner should
  be extended to split on `;` or use pgx's extended-query protocol. Not needed
  until we have a case that requires it.
