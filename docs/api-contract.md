# API Contract

This document is the single source of truth for the HTTP and WebSocket API
that the frontend consumes. All types are derived directly from the Go handler
and service source code.

---

## Conventions

| Convention | Detail |
|---|---|
| Base URL | `https://<host>/` |
| Auth header | `Authorization: Bearer <access_token>` |
| Content-Type | `application/json` for all request and response bodies |
| Timestamps | RFC 3339 UTC strings (`"2026-04-21T10:00:00Z"`) |
| IDs | UUID v4 lowercase strings (`"550e8400-e29b-41d4-a716-446655440000"`) |
| Unknown request fields | Rejected with `400 invalid_body` |

---

## Error envelope

Every error response uses this shape:

```json
{
  "error": {
    "code": "snake_case_machine_readable_code",
    "message": "Human-readable sentence safe to show the user.",
    "details": {}
  },
  "request_id": "optional-string"
}
```

`details` is only present for `validation_failed` errors (see [Validation errors](#validation-errors)).

---

## Validation errors

```json
{
  "error": {
    "code": "validation_failed",
    "message": "One or more fields are invalid.",
    "details": {
      "fields": [
        { "field": "display_name", "reason": "must not exceed 32 characters" }
      ]
    }
  }
}
```

---

## Authentication

### `POST /v1/auth/register`

Rate-limited: 10 req / min per IP.

**Request**
```json
{ "email": "string", "username": "string", "password": "string" }
```

**Response `201`**
```json
{ "access_token": "string", "refresh_token": "string", "expires_in": 3600 }
```

| Error code | Status | Meaning |
|---|---|---|
| `email_taken` | 409 | Email already registered |
| `username_taken` | 409 | Username already taken |
| `invalid_body` | 400 | Malformed JSON |

---

### `POST /v1/auth/login`

Rate-limited: 10 req / min per IP.

**Request**
```json
{ "email": "string", "password": "string" }
```

**Response `200`**
```json
{ "access_token": "string", "refresh_token": "string", "expires_in": 3600 }
```

| Error code | Status | Meaning |
|---|---|---|
| `invalid_credentials` | 401 | Wrong email or password (never reveals which) |

---

### `POST /v1/auth/refresh`

Rate-limited: 10 req / min per IP.

**Request**
```json
{ "refresh_token": "string" }
```

**Response `200`** — same shape as register/login.

Refresh tokens are **rotated on use** — store the new `refresh_token` from
the response.

| Error code | Status | Meaning |
|---|---|---|
| `token_expired` | 401 | Refresh token has expired (30-day TTL) |
| `invalid_token` | 401 | Token is malformed or tampered |

---

## Profile

### `GET /v1/me` *(auth)*

Returns the current player's profile. **Creates a profile on first call** and
returns `201`; subsequent calls return `200`.

**Response `200` / `201`**
```json
{
  "user_id": "uuid",
  "display_name": "string",
  "avatar_id": 0,
  "trophies": 0,
  "gold": 0,
  "diamonds": 0,
  "energy": 5,
  "energy_max": 5,
  "xp": 0,
  "level": 1
}
```

`energy` is the lazily-computed current value (1 point regenerated per 30
minutes, hard cap `energy_max` = 5).

---

### `PATCH /v1/me` *(auth)*

Both fields are **optional**; omit a key to leave it unchanged.

**Request**
```json
{ "display_name": "string", "avatar_id": 0 }
```

Constraints:
- `display_name` — ≤ 32 Unicode code points
- `avatar_id` — integer 0–99

Both fields are applied **atomically** in a single DB write, so no concurrent
reader observes a half-updated row.

**Response `200`** — same shape as `GET /v1/me`.

| Error code | Status | Meaning |
|---|---|---|
| `validation_failed` | 400 | Field constraint violated (see details) |
| `profile_not_found` | 404 | Profile not yet created (call `GET /v1/me` first) |

---

## Shop

### `GET /v1/shop/towers` *(auth)*

Returns the full tower catalog with an `owned` flag per entry.

**Response `200`**
```json
{
  "towers": [
    {
      "id": "uuid",
      "name": "string",
      "rarity": "common|rare|epic|legendary",
      "base_damage": 100,
      "base_range": 3,
      "base_rate": 1,
      "cost_diamonds": 50,
      "description": "string",
      "owned": false
    }
  ]
}
```

---

### `POST /v1/shop/towers/{id}/buy` *(auth)*

Purchase a tower template for `cost_diamonds`.

**Response `201`**
```json
{
  "tower": {
    "template_id": "uuid",
    "name": "string",
    "rarity": "string",
    "cost_diamonds": 50,
    "description": "string",
    "current": { "level": 1, "gold_cost": 0, "damage": 100, "range": 3, "rate": 1 }
  }
}
```

`current.gold_cost` is the cost to upgrade to the next level (0 at max level).

| Error code | Status | Meaning |
|---|---|---|
| `insufficient_diamonds` | 409 | Not enough diamonds |
| `already_owned` | 409 | Player already owns this tower |
| `tower_not_found` | 404 | Template UUID does not exist |

---

## Towers (owned)

### `GET /v1/towers` *(auth)*

Lists all towers the player owns with their current level stats.

**Response `200`**
```json
{
  "towers": [
    {
      "template_id": "uuid",
      "name": "string",
      "rarity": "string",
      "cost_diamonds": 50,
      "description": "string",
      "current": { "level": 1, "gold_cost": 200, "damage": 100, "range": 3, "rate": 1 }
    }
  ]
}
```

---

### `POST /v1/towers/{id}/upgrade` *(auth)*

`{id}` is the template UUID. Spends `current.gold_cost` gold to advance the
tower one level.

**Response `200`** — same tower shape with updated `current`.

| Error code | Status | Meaning |
|---|---|---|
| `insufficient_gold` | 409 | Not enough gold |
| `max_level` | 409 | Tower is already at the maximum level |
| `not_owned` | 409 | Player does not own this tower |
| `tower_not_found` | 404 | Template UUID does not exist |

---

## Matches (single-player)

### `POST /v1/matches` *(auth)*

Start a solo match. Costs **1 energy**.

**Request**
```json
{ "map_id": "string" }
```

**Response `201`**
```json
{
  "match": {
    "id": "uuid",
    "player_one": "uuid",
    "mode": "solo",
    "map_id": "string",
    "seed": 1234567890,
    "started_at": "2026-04-21T10:00:00Z",
    "ended_at": null,
    "winner": null
  }
}
```

Use `seed` to initialise the deterministic client-side simulation. The server
reruns the simulation on `POST /v1/matches/{id}/result` to validate the
submitted outcome.

| Error code | Status | Meaning |
|---|---|---|
| `insufficient_energy` | 409 | No energy left |
| `unknown_map` | 400 | `map_id` is not a known map |

---

### `POST /v1/matches/{id}/result` *(auth)*

Submit the match outcome. The server validates plausibility (gold earned vs
map maximum, wave count vs map definition) before awarding prizes.

**Request**
```json
{
  "monsters_killed": 42,
  "waves_cleared": 3,
  "gate_hp": 15,
  "victory": true,
  "gold_earned": 500
}
```

**Response `200`**
```json
{
  "match": { "...same match shape, ended_at and winner filled..." },
  "gold_awarded": 500,
  "trophy_delta": 25
}
```

`trophy_delta` is `0` on defeat and `25` on victory.

| Error code | Status | Meaning |
|---|---|---|
| `match_not_found` | 404 | Match UUID does not exist |
| `match_not_owned` | 403 | Match belongs to another player |
| `match_already_ended` | 409 | Result already submitted |
| `validation_failed` | 400 | Summary values are implausible |

---

## Matchmaking

Join the queue to be paired with another player for a co-op ranked match.
When a match is found the server pushes a `match.found` WebSocket message to
both players (see [WebSocket](#websocket)).

### `POST /v1/matchmaking/join` *(auth)*

**Request**
```json
{ "map_id": "string" }
```

**Response `200`**
```json
{ "status": "queued" }
```

| Error code | Status | Meaning |
|---|---|---|
| `already_queued` | 409 | Player is already in the queue |
| `profile_not_found` | 404 | Profile not yet created |

---

### `DELETE /v1/matchmaking/leave` *(auth)*

**Response `204`** — no body.

---

## Chat

### `GET /v1/chat/channels/{id}/messages` *(auth)*

Paginated message history, newest-first.

**Query params**

| Param | Type | Default | Description |
|---|---|---|---|
| `before` | RFC 3339 | (none) | Exclusive cursor — return only messages created before this timestamp |
| `limit` | int 1–100 | 50 | Number of messages to return |

**Response `200`**
```json
{
  "messages": [
    {
      "id": "uuid",
      "channel_id": "uuid",
      "user_id": "uuid",
      "body": "string",
      "created_at": "2026-04-21T10:00:00Z"
    }
  ]
}
```

For the next page pass `created_at` of the last message as `before`.

| Error code | Status | Meaning |
|---|---|---|
| `channel_not_found` | 404 | Channel UUID does not exist |
| `not_member` | 403 | Player is not a member of this channel |

---

### `POST /v1/chat/channels/{id}/messages` *(auth)*

Rate-limited: 10 messages / 10 seconds per user-channel pair.

**Request**
```json
{ "body": "string" }
```

`body` must be 1–500 Unicode code points. Global channel (`00000000-0000-4000-8000-000000000001`) auto-joins the sender on first message.

**Response `201`**
```json
{
  "message": {
    "id": "uuid",
    "channel_id": "uuid",
    "user_id": "uuid",
    "body": "string",
    "created_at": "2026-04-21T10:00:00Z"
  }
}
```

After a successful send the server also pushes a `chat.message` WS event to
all channel members (see [WebSocket](#websocket)).

| Error code | Status | Meaning |
|---|---|---|
| `body_empty` | 422 | Body was empty or whitespace-only |
| `body_too_long` | 422 | Body exceeds 500 characters |
| `not_member` | 403 | Not a member of this channel |
| `rate_limited` | 429 | Throttled — respect the `Retry-After` response header (seconds) |

---

## Leaderboard

### `GET /v1/leaderboard/global` *(auth)*

**Query params**

| Param | Type | Default | Description |
|---|---|---|---|
| `after_rank` | int | 0 | Exclusive rank cursor (omit for first page) |
| `limit` | int 1–100 | 25 | Page size |

**Response `200`**
```json
{
  "entries": [
    { "rank": 1, "user_id": "uuid", "trophies": 9000 }
  ],
  "next_cursor": 25
}
```

`next_cursor` is `null` on the last page. Pass it as `after_rank` for the
next page.

---

### `GET /v1/leaderboard/alliances` *(auth)*

**Query params**

| Param | Type | Default | Description |
|---|---|---|---|
| `after_trophies` | int | (none) | Composite cursor part 1 |
| `after_id` | UUID | (none) | Composite cursor part 2 — last alliance UUID |
| `limit` | int 1–100 | 25 | Page size |

Omit both cursor params for the first page. Both must be provided together for
subsequent pages.

**Response `200`**
```json
{
  "entries": [
    {
      "alliance_id": "uuid",
      "alliance_name": "string",
      "alliance_tag": "string",
      "total_trophies": 5000,
      "member_count": 12
    }
  ],
  "next_cursor_trophies": 5000,
  "next_cursor_id": "uuid"
}
```

Both cursor fields are `null` together on the last page.

---

### `GET /v1/alliances/{id}/leaderboard` *(auth)*

Member leaderboard for a specific alliance.

**Response `200`**
```json
{
  "entries": [
    { "rank": 1, "user_id": "uuid", "role": "leader|officer|member", "trophies": 1200 }
  ]
}
```

---

## Events

### `GET /v1/events` *(auth)*

Returns events that are currently active or start within the next 7 days.

**Response `200`**
```json
{
  "events": [
    {
      "id": "uuid",
      "kind": "string",
      "name": "string",
      "description": "string",
      "starts_at": "2026-04-21T10:00:00Z",
      "ends_at": "2026-04-28T10:00:00Z",
      "config": {}
    }
  ]
}
```

`config` is an opaque JSON object whose shape depends on `kind`.

---

### `POST /v1/events/{id}/claim` *(auth)*

Claim a reward tier. `tier` is **zero-based**.

**Request**
```json
{ "tier": 0 }
```

**Response `200`**
```json
{ "rewards": { "gold": 500, "diamonds": 50 } }
```

`rewards` keys are resource names; values are integer amounts.

| Error code | Status | Meaning |
|---|---|---|
| `event_not_found` | 404 | Event UUID does not exist |
| `event_not_active` | 409 | Event has not started or has ended |
| `tier_invalid` | 400 | Tier index is out of range |
| `tier_not_reached` | 409 | Player has not reached this tier yet |
| `tier_already_claimed` | 409 | This tier was already claimed |

---

## Alliances

### `POST /v1/alliances` *(auth)*

Create a new alliance. The requester becomes its leader.

**Request**
```json
{ "name": "string", "tag": "string", "description": "string" }
```

**Response `201`**
```json
{
  "alliance": {
    "id": "uuid",
    "name": "string",
    "tag": "string",
    "description": "string",
    "leader_id": "uuid",
    "channel_id": "uuid",
    "created_at": "2026-04-21T10:00:00Z"
  }
}
```

`channel_id` is the UUID of the automatically-created alliance chat channel.

| Error code | Status | Meaning |
|---|---|---|
| `alliance_name_taken` | 409 | Name already in use |
| `alliance_tag_taken` | 409 | Tag already in use |
| `already_in_alliance` | 409 | Requester is already in an alliance |

---

### `GET /v1/alliances/{id}` *(auth)*

**Response `200`** — `{ "alliance": { ...same shape... } }`

| Error code | Status | Meaning |
|---|---|---|
| `alliance_not_found` | 404 | — |

---

### `DELETE /v1/alliances/{id}` *(auth)*

Disband the alliance. Leader only.

**Response `204`** — no body.

| Error code | Status | Meaning |
|---|---|---|
| `alliance_permission_denied` | 403 | Requester is not the leader |
| `alliance_not_found` | 404 | — |

---

### `GET /v1/alliances/{id}/members` *(auth)*

**Response `200`**
```json
{
  "members": [
    {
      "user_id": "uuid",
      "alliance_id": "uuid",
      "role": "leader|officer|member",
      "joined_at": "2026-04-21T10:00:00Z"
    }
  ]
}
```

---

### `DELETE /v1/alliances/{id}/members/{userID}` *(auth)*

Kick a member. Leader or officer only.

**Response `204`** — no body.

| Error code | Status | Meaning |
|---|---|---|
| `cannot_kick_leader` | 403 | Cannot kick the leader |
| `alliance_permission_denied` | 403 | Insufficient role |

---

### `POST /v1/alliances/{id}/members/{userID}/promote` *(auth)*

Promote a member to officer. Leader only.

**Response `204`** — no body.

---

### `POST /v1/alliances/{id}/members/{userID}/demote` *(auth)*

Demote an officer back to member. Leader only.

**Response `204`** — no body.

---

### `POST /v1/alliances/{id}/invites` *(auth)*

Send an invitation. Leader or officer only.

**Request**
```json
{ "user_id": "uuid" }
```

**Response `201`**
```json
{
  "invite": {
    "id": "uuid",
    "alliance_id": "uuid",
    "user_id": "uuid",
    "status": "pending",
    "created_at": "2026-04-21T10:00:00Z"
  }
}
```

| Error code | Status | Meaning |
|---|---|---|
| `already_invited` | 409 | Target already has a pending invite |
| `already_in_alliance` | 409 | Target is already in an alliance |
| `alliance_permission_denied` | 403 | Insufficient role |

---

### `POST /v1/invites/{id}/accept` *(auth)*

Accept a pending invite addressed to the current user.

**Response `204`** — no body.

| Error code | Status | Meaning |
|---|---|---|
| `invite_not_found` | 404 | Invite does not exist or belongs to someone else |
| `invite_not_pending` | 409 | Invite is no longer pending |
| `already_in_alliance` | 409 | User joined another alliance in the meantime |

---

### `POST /v1/invites/{id}/decline` *(auth)*

**Response `204`** — no body.

---

### `GET /v1/me/alliance` *(auth)*

Returns the current user's alliance membership.

**Response `200`**
```json
{
  "membership": {
    "user_id": "uuid",
    "alliance_id": "uuid",
    "role": "leader|officer|member",
    "joined_at": "2026-04-21T10:00:00Z"
  }
}
```

| Error code | Status | Meaning |
|---|---|---|
| `not_in_alliance` | 404 | User is not in any alliance |

---

### `POST /v1/me/alliance/leave` *(auth)*

Leave the current alliance.

**Response `204`** — no body.

| Error code | Status | Meaning |
|---|---|---|
| `leader_must_transfer` | 409 | Leader must transfer leadership or disband before leaving |

---

## Health

### `GET /healthz`

No auth required.

**Response `200`** — empty body. Used by load balancers.

---

## WebSocket

### Connection

```
GET /v1/ws
```

**Auth** — pass a valid access token in one of two ways:

1. Query parameter: `?token=<jwt>` (simpler for mobile)
2. `Sec-WebSocket-Protocol` header: set the value to the JWT string (for
   browser environments where custom headers on WS connections are restricted)

Connection is rejected with `401` if the token is missing, expired, or invalid.

---

### Message envelope

Every message in both directions uses this wrapper:

```json
{ "v": 1, "type": "string", "payload": {} }
```

- `v` must be `1`. The server closes the connection on version mismatch.
- `type` identifies the message.
- `payload` is the type-specific object.

---

### Client → Server messages

#### `ping`

```json
{ "v": 1, "type": "ping", "payload": {} }
```

Server replies immediately with `pong`.

---

#### `match.input`

Send player actions during a **multiplayer** session.

```json
{
  "v": 1,
  "type": "match.input",
  "payload": {
    "seq": 1,
    "input": {
      "place_towers": [
        {
          "template_id": "uuid-string",
          "tile": { "col": 2, "row": 3 },
          "damage": 100,
          "range": 3.0,
          "rate": 1.0,
          "gold_cost": 50
        }
      ]
    }
  }
}
```

- `seq` must increase monotonically. Inputs with `seq ≤` the last accepted
  value are silently dropped (duplicate/stale protection).
- Each player may only place towers in their own half of the map
  (`role=1` → left half columns, `role=2` → right half columns). Out-of-bounds
  placements in `place_towers` are silently filtered.
- If the channel is full (client flooding) the message is silently dropped;
  the client reconciles against the next `match.snapshot`.

---

#### `chat.typing`

Broadcast a typing indicator to other members of a channel.

```json
{
  "v": 1,
  "type": "chat.typing",
  "payload": { "channel_id": "uuid" }
}
```

Silently dropped if the user is not a member of the channel.

---

### Server → Client messages

#### `pong`

```json
{ "v": 1, "type": "pong", "payload": {} }
```

---

#### `match.found`

Pushed to both players when matchmaking finds a pair.

```json
{
  "v": 1,
  "type": "match.found",
  "payload": {
    "match_id": "uuid",
    "map_id": "alpha",
    "seed": 1234567890,
    "opponent_id": "uuid",
    "role": 1
  }
}
```

- `role` `1` → player owns the **left half** of the map (columns 0 …
  `map.cols/2 - 1`)
- `role` `2` → player owns the **right half** (columns `map.cols/2` …
  `map.cols - 1`)
- After receiving `match.found` the client should immediately start sending
  `match.input` messages and listening for `match.snapshot`.

---

#### `match.snapshot`

Pushed every **200 ms** during a multiplayer session with the authoritative
simulation state.

```json
{
  "v": 1,
  "type": "match.snapshot",
  "payload": {
    "tick": 1,
    "state": {
      "map": {
        "id": "string",
        "cols": 16,
        "rows": 10,
        "waypoints": [{ "x": 0.5, "y": 5.5 }],
        "gate": { "x": 15.5, "y": 5.5 },
        "tiles": [{ "col": 2, "row": 3 }]
      },
      "towers": [
        {
          "id": 1,
          "template_id": "uuid-string",
          "tile": { "col": 2, "row": 3 },
          "damage": 100,
          "range": 3.0,
          "rate": 1.0,
          "cooldown": 0.0
        }
      ],
      "monsters": [
        {
          "id": 1,
          "max_hp": 200,
          "hp": 150,
          "speed": 2.0,
          "reward": 10,
          "progress": 4.5,
          "alive": true
        }
      ],
      "waves": [ { "number": 1, "groups": [] } ],
      "wave_idx": 0,
      "wave_time": 0.4,
      "spawn_records": [],
      "gold": 150,
      "gate_hp": 20,
      "tick": 1,
      "next_id": 5
    }
  }
}
```

The client renders from `state`. `tick` is the monotonically increasing
simulation step counter.

---

#### `match.ended`

Pushed once when the session ends (victory or defeat).

```json
{
  "v": 1,
  "type": "match.ended",
  "payload": {
    "winner_id": "uuid",
    "gold_awarded": 500,
    "trophy_delta": 25
  }
}
```

- `winner_id` is **omitted** (`""`) on defeat.
- `trophy_delta` is `0` on defeat and `25` on victory.
- After receiving this event the client should stop sending `match.input`.

---

#### `chat.message`

Pushed when a new message is sent to any channel the user is a member of.

```json
{
  "v": 1,
  "type": "chat.message",
  "payload": {
    "channel_id": "uuid",
    "message": {
      "id": "uuid",
      "channel_id": "uuid",
      "user_id": "uuid",
      "body": "string",
      "created_at": "2026-04-21T10:00:00Z"
    }
  }
}
```

---

#### `chat.typing`

Re-broadcast of another user's typing indicator. The original sender does not
receive an echo.

```json
{
  "v": 1,
  "type": "chat.typing",
  "payload": {
    "channel_id": "uuid",
    "user_id": "uuid"
  }
}
```

---

#### `error`

Sent by the server when a WS message cannot be processed.

```json
{
  "v": 1,
  "type": "error",
  "payload": {
    "code": "snake_case_code",
    "message": "Human-readable description."
  }
}
```

---

## Well-known IDs

| Resource | UUID |
|---|---|
| Global chat channel | `00000000-0000-4000-8000-000000000001` |

The global channel is pre-seeded by migration `0008`. Any authenticated user
can read and write to it; membership is auto-joined on first message.

---

## Map IDs

Currently available map IDs (from `backend/internal/game/sim/maps.go`):

| ID | Description |
|---|---|
| `alpha` | Starter map |

Additional maps will be added here as they are implemented.

---

## Resource limits summary

| Resource | Limit | Notes |
|---|---|---|
| Display name | 32 Unicode code points | |
| Avatar ID | 0–99 | |
| Chat message body | 1–500 Unicode code points | |
| Tower level | 1–10 | |
| Energy | 0–5 | 1 point per 30 min |
| Matchmaking buckets | 100 trophies | Players within same bucket are eligible |
| Chat rate limit | 10 msg / 10 s | Per user-channel pair |
| Auth rate limit | 10 req / min | Per IP, applies to register/login/refresh |
