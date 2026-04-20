# Development Roadmap

This document is the authoritative step-by-step plan for building the game. It is organized into **phases**, each phase into **milestones**, and each milestone into **small commit-sized tasks**. Every task is intended to be one (or a handful of) commits on a `claude/<topic>` branch.

Read `CLAUDE.md` and everything under `.claude/rules/` before starting work on any phase.

---

## Phase 0 — Foundations and tooling

### Milestone 0.1: Repository bootstrap
- [x] Initialize the git repository and create `main` as the permanent branch.
- [x] Add `README.md`, `DEVELOPMENT.md`, `CLAUDE.md`, `.claude/` rules and commands.
- [x] Add a top-level `.editorconfig`, `.gitignore`, `LICENSE` placeholder.
- [x] Create empty `backend/` and `frontend/` directories with placeholder `README.md` files.

### Milestone 0.2: Backend tooling
- [x] `cd backend && go mod init <module-path>` using Go 1.22+.
- [x] Pin Go version via `go.mod`.
- [x] Add a `Makefile` (or `justfile`) with targets: `run`, `test`, `lint`, `fmt`, `migrate-up`, `migrate-down`.
- [x] Configure `gofmt`, `go vet`, `staticcheck` (installed once via `go install`).
- [x] Confirm CI-friendly: `go test ./... -race -cover`.

### Milestone 0.3: Frontend tooling
- [x] `npm create vite@latest frontend -- --template react-ts`.
- [x] Configure ESLint + Prettier with the rules in `.claude/rules/code-style.md`.
- [x] Add scripts: `dev`, `build`, `preview`, `test`, `lint`, `format`.
- [x] Install Vitest + React Testing Library for unit/component tests.
- [x] Add Pixi.js as a dependency but do not wire it in yet.

### Milestone 0.4: Local PostgreSQL
- [x] Provide a `docker-compose.yml` at the repo root with a pinned `postgres:16` service.
- [x] Commit `backend/.env.example` with the minimum env variables (`DATABASE_URL`, `JWT_SECRET`, `LISTEN_ADDR`, `LOG_LEVEL`).
- [x] Document how to bring the stack up in `README.md`.

---

## Phase 1 — Backend core infrastructure

### Milestone 1.1: Configuration and logging
- [x] `internal/config/config.go` — loads env vars, validates them, returns an immutable `Config` struct. **No external library**; use `os.Getenv` + a small helper.
- [x] `internal/logging/logging.go` — thin wrapper over `log/slog` (stdlib). Export `New(level string) *slog.Logger`.
- [x] Unit tests for both modules.

### Milestone 1.2: Database connection and migrations
- [x] `internal/db/db.go` — opens a `*pgxpool.Pool`, applies sensible defaults (max conns, lifetime, ping on start).
- [x] `internal/db/migrations/` — numbered SQL files: `0001_init.up.sql`, `0001_init.down.sql`.
- [x] `cmd/migrate/main.go` — a tiny migration runner built on top of `database/sql` + `pgx/stdlib` that reads files in lexicographic order and records applied migrations in `schema_migrations`. **Do not pull in `golang-migrate`.**
- [x] Unit tests for migration ordering and idempotency.

### Milestone 1.3: HTTP server skeleton
- [x] `cmd/server/main.go` — starts an `http.Server` with graceful shutdown via `signal.NotifyContext`.
- [x] `internal/httpserver/router.go` — custom router built on `http.ServeMux` (Go 1.22 pattern matching is enough).
- [x] `internal/httpserver/middleware/` — request logging, panic recovery, request ID, CORS, JSON content-type enforcement.
- [x] `GET /healthz` returns `200 OK` with build info.
- [x] Integration test: spin the server on an ephemeral port and hit `/healthz`.

### Milestone 1.4: Response and error conventions
- [x] `internal/httpserver/respond.go` — `RespondJSON`, `RespondError`, shared error envelope.
- [x] Conform to the conventions in `.claude/rules/error-handling.md`.
- [x] Unit tests for every helper.

---

## Phase 2 — Accounts and authentication

### Milestone 2.1: Schema
- [ ] Migration `0002_users.up.sql`: `users(id uuid pk, email citext unique, username citext unique, password_hash text, created_at, updated_at)`.
- [ ] Migration `0002_users.down.sql` reverses it.

### Milestone 2.2: Password hashing
- [ ] `internal/auth/password.go` — `Hash(plain string) (string, error)` and `Verify(hash, plain string) error` using `golang.org/x/crypto/bcrypt`.
- [ ] Tests including wrong-password, empty input, and max-length cases.

### Milestone 2.3: JWT
- [ ] `internal/auth/jwt.go` — implement HS256 signing/verification manually using `crypto/hmac`, `crypto/sha256`, `encoding/base64`, `encoding/json`. **No `golang-jwt` dependency.**
- [ ] Exported `Sign(claims Claims, secret []byte, ttl time.Duration) (string, error)` and `Parse(token string, secret []byte) (Claims, error)`.
- [ ] Tests: valid token, tampered signature, expired token, malformed token, clock-skew edge cases.

### Milestone 2.4: Registration and login endpoints
- [ ] `POST /v1/auth/register` — validates input, hashes password, inserts row, returns token.
- [ ] `POST /v1/auth/login` — verifies password, returns token.
- [ ] `POST /v1/auth/refresh` — issues a new token given a valid (unexpired) one.
- [ ] Rate limit these endpoints via a per-IP token bucket (stdlib implementation).
- [ ] Integration tests against a test PostgreSQL instance (see `.claude/rules/testing.md` for fixture patterns).

### Milestone 2.5: Auth middleware
- [ ] `internal/httpserver/middleware/auth.go` — extracts Bearer token, validates, puts `UserID` on the request context.
- [ ] Helper `UserIDFromContext(ctx) (uuid.UUID, bool)`.
- [ ] Tests for missing header, malformed token, expired token, happy path.

---

## Phase 3 — Player profile and resources

### Milestone 3.1: Schema
- [ ] Migration `0003_profiles.up.sql`:
  - `profiles(user_id pk, display_name, avatar_id, trophies int, gold bigint, diamonds bigint, energy int, energy_updated_at timestamp, xp bigint, level int, created_at, updated_at)`.
- [ ] CHECK constraints on non-negative gold/diamonds/energy/trophies.

### Milestone 3.2: Resource service
- [ ] `internal/game/resources.go` — `AddGold`, `SpendGold`, `AddDiamonds`, `SpendDiamonds`. All operations are atomic (`SELECT ... FOR UPDATE` inside a transaction).
- [ ] Energy regenerates at `1 per N seconds`; computed lazily on read (don't run a scheduler).
- [ ] Exhaustive tests: race conditions (use `testing/synctest` or goroutine stress), underflow protection, concurrent spending.

### Milestone 3.3: Profile endpoints
- [ ] `GET /v1/me` — returns profile + computed energy.
- [ ] `PATCH /v1/me` — updates display name and avatar only.
- [ ] Tests.

---

## Phase 4 — Towers and shop

### Milestone 4.1: Tower catalog
- [ ] Migration `0004_towers.up.sql`:
  - `tower_templates(id, name, rarity, base_damage, base_range, base_rate, cost_diamonds, description)` — static catalog.
  - `tower_levels(template_id, level, gold_cost, damage, range_, rate)` — 10 rows per template describing each upgrade.
  - `owned_towers(user_id, template_id, level, unlocked_at, primary key (user_id, template_id))`.
- [ ] Seed a handful of starter towers via a `0004_towers_seed.up.sql` migration.

### Milestone 4.2: Tower service
- [ ] `internal/game/towers.go`:
  - `ListCatalog(ctx)` — returns the full catalog.
  - `ListOwned(ctx, userID)`.
  - `Purchase(ctx, userID, templateID)` — debits diamonds, inserts `owned_towers`, rejects duplicates.
  - `LevelUp(ctx, userID, templateID)` — debits gold based on `tower_levels`, caps at level 10.
- [ ] Unit tests for every branch.

### Milestone 4.3: Shop and towers endpoints
- [ ] `GET /v1/shop/towers` — catalog with an `owned` flag.
- [ ] `POST /v1/shop/towers/{id}/buy` — wraps `Purchase`.
- [ ] `GET /v1/towers` — owned towers with current stats.
- [ ] `POST /v1/towers/{id}/upgrade` — wraps `LevelUp`.
- [ ] Integration tests covering insufficient resources, max level, unknown template.

---

## Phase 5 — Single-player match logic

### Milestone 5.1: Game simulation primitives
- [ ] `internal/game/sim/` — pure functions with no I/O:
  - `Map` — waypoints, gate position, tower-placement tiles.
  - `Monster` — hp, speed, reward, path progress.
  - `Tower` — owner, template level, cooldown.
  - `Wave` — spawn schedule.
- [ ] Deterministic tick function `Step(state, input, dt) State`. Property-based tests on conservation rules (hp never negative, gold never decreases without a reason, etc.).

### Milestone 5.2: Match lifecycle
- [ ] Migration `0005_matches.up.sql`:
  - `matches(id, player_one, player_two NULL, mode, started_at, ended_at, winner, seed, final_state jsonb)`.
- [ ] `internal/game/match.go`:
  - `StartSinglePlayer(userID, mapID)` — seeds RNG, persists match row.
  - `SubmitResult(matchID, summary)` — awards gold, trophies, diamonds.
- [ ] `POST /v1/matches` to start, `POST /v1/matches/{id}/result` to submit.
- [ ] Server-side replay validation stub (checks final state is plausible given the seed).
- [ ] Tests covering both replay acceptance and rejection.

---

## Phase 6 — WebSocket infrastructure

### Milestone 6.1: WS endpoint
- [ ] `internal/ws/hub.go` — a Hub that tracks connected clients by user ID; register/unregister through channels.
- [ ] `internal/ws/client.go` — read/write pumps, ping/pong, backpressure.
- [ ] `GET /v1/ws` — authenticates via token in the query string or `Sec-WebSocket-Protocol` header, upgrades.
- [ ] Tests using `httptest.NewServer` and a WS client helper.

### Milestone 6.2: Message protocol
- [ ] `internal/ws/protocol.go` — tagged JSON messages: `{"type": "...", "payload": {...}}`.
- [ ] Versioned via a top-level `v` field.
- [ ] Round-trip tests for every message type.

---

## Phase 7 — Multiplayer matches

### Milestone 7.1: Matchmaking
- [ ] `internal/game/matchmaker.go` — simple trophy-bucket queue; matches on arrival order within a bucket.
- [ ] `POST /v1/matchmaking/join`, `POST /v1/matchmaking/leave`.
- [ ] Push match-found events over WS.
- [ ] Tests for bucket assignment and fairness.

### Milestone 7.2: Authoritative match loop
- [ ] `internal/game/session.go` — one goroutine per active multiplayer match, running the simulation at a fixed tick rate.
- [ ] Input is validated (player can only act on their side; resource checks).
- [ ] Snapshots sent to both clients every N ticks.
- [ ] Client inputs delivered over WS with sequence numbers; duplicates/late inputs are dropped.
- [ ] Tests for desync detection and input replay.

### Milestone 7.3: Match completion
- [ ] Persist final state; award trophies based on ELO-lite delta; notify both clients.
- [ ] Tests.

---

## Phase 8 — Chat

### Milestone 8.1: Schema
- [ ] `chat_channels(id, kind, owner_id)` where `kind ∈ {'global','alliance','direct'}`.
- [ ] `chat_memberships(channel_id, user_id, joined_at)`.
- [ ] `chat_messages(id, channel_id, user_id, body, created_at)` with an index on `(channel_id, created_at desc)`.

### Milestone 8.2: Message service
- [ ] `internal/chat/service.go` — `Send`, `History`, `EnsureMembership`. Enforce max body length, basic profanity filter stub.
- [ ] Broadcast sent messages to channel members via the WS hub.

### Milestone 8.3: Chat endpoints and WS messages
- [ ] `GET /v1/chat/channels/{id}/messages?before=...&limit=...`.
- [ ] `POST /v1/chat/channels/{id}/messages`.
- [ ] WS messages: `chat.message`, `chat.typing`.
- [ ] Tests for rate limiting (per channel per user), pagination, permission checks.

---

## Phase 9 — Alliances

### Milestone 9.1: Schema
- [ ] `alliances(id, name unique, tag unique, description, leader_id, created_at)`.
- [ ] `alliance_members(user_id pk, alliance_id, role, joined_at)` — role ∈ {leader, officer, member}.
- [ ] `alliance_invites(id, alliance_id, user_id, status, created_at)`.

### Milestone 9.2: Alliance service
- [ ] `Create`, `Disband`, `Invite`, `AcceptInvite`, `Leave`, `Promote`, `Demote`, `Kick`.
- [ ] Automatically create an alliance chat channel on creation and add all members.
- [ ] Tests for every permission branch (officer can't disband, leader must transfer before leaving, etc.).

### Milestone 9.3: Alliance endpoints
- [ ] REST endpoints mirroring the service surface.
- [ ] Tests.

---

## Phase 10 — Leaderboard

### Milestone 10.1: Global leaderboard
- [ ] Materialized view refreshed every N minutes: `global_leaderboard(rank, user_id, trophies)`.
- [ ] `GET /v1/leaderboard/global?limit=&cursor=`.
- [ ] Tests for ranking stability and cursor pagination.

### Milestone 10.2: Alliance leaderboard
- [ ] `alliance_leaderboard(alliance_id, total_trophies, member_count)` — similar approach.
- [ ] `GET /v1/leaderboard/alliances`.
- [ ] Per-alliance member rankings: `GET /v1/alliances/{id}/leaderboard`.

---

## Phase 11 — Events

### Milestone 11.1: Event framework
- [ ] `events(id, kind, name, description, starts_at, ends_at, config jsonb)`.
- [ ] `event_progress(event_id, user_id, progress jsonb, claimed_rewards jsonb)`.
- [ ] Event engine in `internal/events/engine.go` with a `Kind` interface; start with one concrete kind (`kill_n_monsters`).

### Milestone 11.2: Event endpoints
- [ ] `GET /v1/events` — active/upcoming events.
- [ ] `POST /v1/events/{id}/claim` — claim a reward tier.
- [ ] Match results call `engine.RecordProgress(userID, eventContext)`.
- [ ] Tests for the event engine, reward idempotency, event time boundaries.

---

## Phase 12 — Frontend scaffolding

### Milestone 12.1: Project layout
- [ ] `src/api/`, `src/game/`, `src/screens/`, `src/components/`, `src/state/`, `src/hooks/`, `src/lib/`.
- [ ] Path aliases in `tsconfig.json`.
- [ ] Global style reset, CSS variables for the palette, vertical-first layout (`viewport-fit=cover`).

### Milestone 12.2: API client
- [ ] `src/api/client.ts` — thin `fetch` wrapper that attaches the JWT, retries idempotent requests, and normalizes errors.
- [ ] Typed endpoints mirroring backend routes.
- [ ] Tests with MSW (acceptable test-only dep) or `fetch` mocking via the stdlib `Response`.

### Milestone 12.3: Auth flow
- [ ] Login/Register screens.
- [ ] Token stored via Capacitor `Preferences` when running on-device, `localStorage` in the browser (abstract behind a `Storage` interface).
- [ ] Tests.

---

## Phase 13 — React UI shell (five sections)

### Milestone 13.1: Navigation
- [ ] Bottom tab bar with: **Shop**, **Towers**, **Main**, **Alliance**, **Events**.
- [ ] Route state persisted across reloads.
- [ ] Tests for navigation and deep-link restoration.

### Milestone 13.2: Main screen
- [ ] Play button, resource HUD (gold, diamonds, energy), trophies badge, daily rewards slot.

### Milestone 13.3: Towers screen
- [ ] Grid of owned towers; tapping one opens an upgrade modal with stat deltas and a gold-locked Upgrade button.

### Milestone 13.4: Shop screen
- [ ] Tower bundles with price in diamonds; owned towers are greyed out.
- [ ] Confirmation modal before purchase.

### Milestone 13.5: Alliance screen
- [ ] If no alliance: Create/Browse/Join. If in one: roster, chat tab, events tab.

### Milestone 13.6: Events screen
- [ ] Active events with progress bars, reward tiers, countdown timers.

Every component has Vitest tests. See `.claude/rules/testing.md`.

---

## Phase 14 — Pixi.js game canvas

### Milestone 14.1: Pixi bootstrap
- [ ] `src/game/engine.ts` — wraps `Application`, owns the ticker, exposes `mount(el)`, `unmount()`.
- [ ] `GameCanvas` React component that mounts the Pixi app on a `div` ref and cleans up in `useEffect`.

### Milestone 14.2: Map rendering
- [ ] Draw the path, the gate, and placement tiles from a JSON map definition (shared with the backend).
- [ ] Tests on the pure path-geometry helpers (stdlib only; no Pixi in tests — separate the logic from the renderer).

### Milestone 14.3: Monsters
- [ ] Monster sprite follows the path using the geometry helpers.
- [ ] Spawner driven by the wave schedule.
- [ ] HP bars.

### Milestone 14.4: Towers
- [ ] Drag-and-drop placement from a bottom tray; invalid tiles reject with a shake animation.
- [ ] Attack targeting (first, strongest, closest) — pure function, tested.
- [ ] Projectile animation.

### Milestone 14.5: HUD overlay
- [ ] Energy bar, wave counter, gate HP, pause button — all React components layered above the canvas.

---

## Phase 15 — Real-time networking in the client

### Milestone 15.1: WebSocket client
- [ ] `src/api/socket.ts` — reconnection with exponential backoff, heartbeat, message-type dispatch.
- [ ] Tests using a mock WebSocket.

### Milestone 15.2: Multiplayer integration
- [ ] Matchmaking screen transitions into the game canvas when the server signals `match.found`.
- [ ] Client sends inputs (`place_tower`, `sell_tower`, `upgrade_tower`) over WS.
- [ ] Client reconciles snapshots with local prediction.

### Milestone 15.3: Chat integration
- [ ] Global chat, alliance chat, DMs — all fed by the same WS client.
- [ ] Unread badges, rate-limit feedback.

---

## Phase 16 — Capacitor packaging

### Milestone 16.1: Capacitor wiring
- [ ] `npx cap init` and commit `capacitor.config.ts`.
- [ ] Add iOS and Android platforms; commit the generated scaffolding only after stripping anything auto-added that isn't needed.
- [ ] Document the local build flow (`npm run build && npx cap sync`).

### Milestone 16.2: Native-only niceties
- [ ] Splash screen, app icon set.
- [ ] Safe-area insets handled in CSS (the UI must not go under the notch).
- [ ] Haptics on tower placement via `@capacitor/haptics`.

---

## Phase 17 — Hardening

### Milestone 17.1: Observability
- [ ] Structured `slog` logs with `request_id`, `user_id`, `match_id`.
- [ ] Prometheus-compatible `/metrics` endpoint written by hand (no `prometheus/client_golang`): counters and histograms backed by `sync/atomic` and a tiny exposition-format writer.

### Milestone 17.2: Security audit
- [ ] Walk through `.claude/rules/security.md` and verify every item.
- [ ] Penetration-style tests (IDOR, auth bypass, rate limits, input validation).
- [ ] Confirm no secrets are logged.

### Milestone 17.3: Performance
- [ ] Load test with `vegeta` or a hand-rolled Go benchmark: 1k concurrent matchmaking requests, 100 concurrent multiplayer matches.
- [ ] Identify hot spots with `pprof`; avoid premature optimization.

### Milestone 17.4: Balancing
- [ ] Create `docs/balancing.md` with tower and monster stats.
- [ ] Pull the numbers into migration seed data so the data model is the source of truth.

---

## Phase 18 — Deployment

### Milestone 18.1: Containerization
- [ ] Multi-stage `Dockerfile` for the backend; final image is `FROM gcr.io/distroless/static:nonroot`.
- [ ] `docker-compose.prod.yml` with Postgres + server + reverse proxy.

### Milestone 18.2: Migrations in production
- [ ] `cmd/migrate` runs as an init container; server refuses to start if migrations aren't applied.

### Milestone 18.3: Mobile submission
- [ ] Configure app signing, bundle IDs, store listings, privacy policy URL.
- [ ] Produce release builds and walk through TestFlight / Play Console internal testing.

---

## Working cadence

For **every** milestone:

1. Start from `main`, pull, create `claude/<topic>`.
2. Break the milestone into small commits (ideally < 200 lines each, one logical change).
3. Write tests **with** the code, not after.
4. Run `go test ./... -race` and `npm test` before every commit.
5. Push the branch. Do not open a PR; the human will rebase it.
6. Never touch `main`; never force-push.

Refer to `.claude/rules/git-workflow.md` for the precise commit-message format.