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

## upgrade-modal-stat-deltas

- **Author:** claude, 2026-04-22
- **Context:** Phase 13 Towers screen — the UpgradeModal shows current stats and
  gold cost but cannot display next-level stat deltas because `GET /v1/towers`
  only returns the current level's stats. Fix: extend the `OwnedTower` response
  to include a `next: TowerStats | null` field populated from the `tower_levels`
  table (null at max level). The frontend UpgradeModal is already structured to
  display a "next stats" section once the data is available.
  Priority: medium — cosmetic; upgrade still works correctly without it.

## alliance-browse

- **Author:** claude, 2026-04-22
- **Context:** Phase 13 Alliance screen — the "Browse Alliances" button is a
  disabled stub because there is no `GET /v1/alliances` list endpoint. To
  implement: add a paginated list endpoint (name/tag/trophy filter) and a
  `GET /v1/invites` endpoint so players can see pending invites. Then wire up
  the Browse tab in `src/screens/Alliance/index.tsx`.
  Priority: medium — needed for organic alliance growth.

## daily-reward

- **Author:** claude, 2026-04-22
- **Context:** Phase 13 Main screen — the DailyRewardSlot component is a stub
  ("Come back tomorrow") because there is no daily-reward system in the backend
  yet. To implement: add a `daily_rewards` table, a claim endpoint, and a streak
  counter. The frontend component can then show the claimable reward and call the
  endpoint on tap.
  Priority: low — nice-to-have for retention; not on the Phase 13 roadmap.

## events-progress-bars

- **Author:** claude, 2026-04-22
- **Context:** Phase 13 Events screen — progress bars are rendered at 0% because
  the `GET /v1/events` response does not include per-user progress. Fix: add a
  `progress` field to the response (joined from `event_progress`) so the frontend
  can render accurate tier progress. The `event_progress__bar-fill` CSS is already
  in place; just needs a `width` value from the API.
  Priority: medium — events feel hollow without visible progress.
