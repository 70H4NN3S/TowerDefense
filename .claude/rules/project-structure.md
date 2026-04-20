# Project structure

The layout is fixed. Don't reorganize without explicit approval.

## Repository root

```
.
├── backend/                  Go server
├── frontend/                 React + Capacitor + Pixi.js client
├── docs/                     Design docs, ADRs, follow-ups, balancing notes
├── scripts/                  Repo-level helper scripts (Bash or Go)
├── docker-compose.yml        Local Postgres and dev services
├── .editorconfig
├── .gitignore
├── LICENSE
├── README.md
├── DEVELOPMENT.md
├── CLAUDE.md
└── .claude/
    ├── commands/             Slash command playbooks
    └── rules/                These files
```

## Backend (`backend/`)

We follow the [Standard Go Project Layout](https://github.com/golang-standards/project-layout) loosely, adapted for our scope.

```
backend/
├── cmd/
│   ├── server/               main.go for the HTTP/WS server
│   └── migrate/              main.go for the migration runner
├── internal/
│   ├── auth/                 JWT, password hashing, auth middleware glue
│   ├── chat/                 Chat service
│   ├── config/               Env-driven config loading
│   ├── db/
│   │   ├── db.go             Pool construction
│   │   └── migrations/       Numbered SQL files
│   ├── events/               Event engine
│   ├── game/
│   │   ├── match.go          Match lifecycle
│   │   ├── matchmaker.go
│   │   ├── resources.go      Gold, diamonds, energy
│   │   ├── session.go        Authoritative multiplayer loop
│   │   ├── sim/              Pure simulation primitives
│   │   └── towers.go         Tower catalog + ownership
│   ├── alliance/
│   ├── httpserver/
│   │   ├── handlers/         One file per resource (auth, profile, towers, ...)
│   │   ├── middleware/
│   │   ├── respond.go        JSON + error envelope helpers
│   │   └── router.go         ServeMux wiring
│   ├── leaderboard/
│   ├── logging/              slog helpers
│   ├── models/               Shared DTOs between service and handler layers
│   └── ws/                   WebSocket hub, protocol, client pump
├── pkg/                      Only if something is genuinely reusable outside
│                             this repo. Start empty.
├── testdata/                 Golden files, fixtures
├── .env.example
├── go.mod
├── go.sum
└── Makefile
```

### Rules for `internal/`

- Code under `internal/` is private to this module. Nothing outside imports it. This is enforced by the Go toolchain.
- Keep packages small and focused around a domain (`auth`, `towers`, `chat`), not a technical layer (`controllers`, `services`, `repositories`). A single package owns its data, its queries, its business logic, and its HTTP handler glue, split across files.
- Packages expose **types + constructor + methods**, not globals.
- Circular imports are not allowed. If you hit one, your package boundaries are wrong — split by domain, not by layer.

### Dependency direction

```
cmd/server  -->  httpserver  -->  <domain packages>  -->  db, logging, config
                                                       ↘
                                                         models
```

- `cmd/` depends on everything; nothing depends on `cmd/`.
- Domain packages (e.g. `game`, `alliance`) depend on `db`, `logging`, `config`, `models`.
- `httpserver/handlers` depend on domain packages and `models` — **never the other way around**.
- `models` depends on nothing internal.

### Handlers

- One file per resource: `handlers/auth.go`, `handlers/towers.go`, `handlers/matches.go`, …
- Each file exports a struct holding dependencies and a `Register(mux *http.ServeMux)` method that wires routes.
- Handlers are thin: decode → validate → call service → respond. All business logic lives in the domain package.

### Migrations

- Location: `internal/db/migrations/`.
- Name: `NNNN_description.up.sql` and `NNNN_description.down.sql`.
- See `.claude/rules/database.md` for content rules.

## Frontend (`frontend/`)

```
frontend/
├── public/
├── src/
│   ├── api/
│   │   ├── client.ts         fetch wrapper, auth, error normalization
│   │   ├── socket.ts         WebSocket client
│   │   ├── endpoints/        One file per backend resource
│   │   └── types.ts          Generated or hand-written shared DTOs
│   ├── components/           Reusable, presentational React components
│   ├── screens/
│   │   ├── Main/
│   │   ├── Shop/
│   │   ├── Towers/
│   │   ├── Alliance/
│   │   └── Events/
│   ├── game/
│   │   ├── logic/            Pure TS: pathing, targeting, simulation tick
│   │   ├── render/           Pixi.js: sprites, scene graph, tickers
│   │   └── engine.ts         Mount/unmount, lifecycle
│   ├── state/                Context/store modules (one per concern)
│   ├── hooks/                Custom React hooks
│   ├── lib/                  Framework-agnostic helpers
│   ├── styles/               Global CSS, tokens
│   ├── App.tsx
│   └── main.tsx
├── tests/                    Integration tests that span multiple modules
├── index.html
├── capacitor.config.ts
├── package.json
├── tsconfig.json
└── vite.config.ts
```

### Frontend layering

```
screens  -->  hooks  -->  state  -->  api
screens  -->  components
game/logic  (pure, tested)
game/render (imports logic, consumed by screens)
```

- A screen imports hooks, components, and game modules. Never imports `api/` directly — always goes through a hook or state module, so tests can swap implementations.
- `game/logic/` never imports Pixi, React, or anything environmental. It's pure TypeScript and runs under Vitest in Node.
- `game/render/` imports `game/logic/`. Screens import `game/render/` only through `game/engine.ts`.

## Docs (`docs/`)

```
docs/
├── adr/                      Architecture Decision Records (NNNN-title.md)
├── balancing.md              Tower/monster stats, wave scaling
├── followups.md              TODOs Claude or the human notices mid-flight
├── game-design.md            Core loop, progression, monetization
└── protocol.md               REST endpoints and WS message schemas
```

ADRs are numbered, titled, and immutable once merged. If a decision is superseded, add a new ADR and link back.

## File-size guidance

- A Go file longer than ~500 lines is a smell. Split.
- A React component file longer than ~250 lines is a smell. Split (extract child components, hooks, or helpers).
- A test file can be longer than its target — that's fine.

## What lives where: quick lookup

| If you're writing…                          | Put it in…                                           |
| ------------------------------------------- | ---------------------------------------------------- |
| A new HTTP route handler                    | `backend/internal/httpserver/handlers/<resource>.go` |
| A new business rule (e.g. tower leveling)   | `backend/internal/game/towers.go`                    |
| A SQL query                                 | Next to the function that uses it, in the domain pkg |
| A new DB table                              | A new migration under `backend/internal/db/migrations/` |
| A React screen                              | `frontend/src/screens/<Name>/index.tsx`              |
| Pure game math                              | `frontend/src/game/logic/`                           |
| A Pixi scene or sprite                      | `frontend/src/game/render/`                          |
| A reusable UI button                        | `frontend/src/components/`                           |
| An architecture decision                    | `docs/adr/NNNN-<slug>.md`                            |
| A TODO you can't finish now                 | `docs/followups.md`                                  |