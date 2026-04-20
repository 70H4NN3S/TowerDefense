# Tower Defense Mobile Game

A multiplayer tower defense game for smartphones with real-time chat, alliances, leaderboards, and events.

## Overview

Players defend a gate from waves of monsters that spawn and follow a predefined path on a map. Towers are placed along the path using an **energy** resource that regenerates over time. If a monster reaches the gate, the player loses.

The metagame loop consists of earning **gold** (from matches) to level up owned towers (levels 1–10) and **diamonds** (from premium activities, events, and occasional match rewards) to unlock new tower types from the shop.

## Feature Summary

| Feature              | Description                                                           |
| -------------------- | --------------------------------------------------------------------- |
| Single-player waves  | Core gameplay loop: monsters follow a path, player places towers.     |
| Multiplayer          | Head-to-head matches synchronized via WebSocket.                      |
| Chat                 | Global, alliance, and direct messaging.                               |
| Alliances            | Player-created groups with shared chat and cooperative events.        |
| Leaderboard          | Global and alliance rankings based on trophies/rating.                |
| Shop                 | Diamonds purchase new towers; gold purchases tower upgrades.          |
| Events               | Time-limited challenges with special rewards.                         |

## User Interface

Clash Royale-style vertical layout with a persistent bottom navigation bar exposing five sections:

1. **Shop** — buy towers with diamonds, occasional gold bundles.
2. **Towers** — view and level up owned towers with gold.
3. **Main** — play button, current trophies/resources, daily rewards.
4. **Alliance** — alliance roster, chat, alliance-specific events.
5. **Events** — time-limited challenges and leaderboards.

## Tech Stack

### Backend
- **Language:** Go (latest stable)
- **Database:** PostgreSQL
- **Philosophy:** Lean on the standard library. Only unavoidable external dependencies:
  - `github.com/jackc/pgx/v5` — PostgreSQL driver (stdlib `database/sql` has no native Postgres support)
  - `github.com/gorilla/websocket` — WebSocket implementation (stdlib has no WebSocket support)
  - `golang.org/x/crypto/bcrypt` — password hashing (part of the Go extended standard library)
- **Explicitly rejected:** Gin, Echo, Fiber, GORM, sqlx, zap, logrus, chi, etc. We use `net/http`, `database/sql`, `encoding/json`, and `log/slog`.

### Frontend
- **Framework:** React (with TypeScript, Vite)
- **Mobile wrapper:** Capacitor (iOS + Android)
- **Game rendering:** Pixi.js (WebGL canvas for the game board)
- **UI outside the game canvas:** React components

## Repository Layout

```
.
├── backend/                  Go server, PostgreSQL migrations
├── frontend/                 React + Capacitor + Pixi.js client
├── docs/                     Design docs, ADRs, game-balance notes
├── README.md                 This file
├── DEVELOPMENT.md            Step-by-step roadmap
├── CLAUDE.md                 Instructions for Claude when editing this repo
└── .claude/                  Rules and slash-command definitions
    ├── commands/             `/fix-issue`, `/review`
    └── rules/                code-style, database, testing, etc.
```

## Getting Started

Prerequisites: Go 1.22+, PostgreSQL 15+, Node.js 20+, and the Capacitor CLI once you reach the mobile-packaging phase.

```bash
# Clone and enter the repo
git clone <repo-url>
cd tower-defense

# Backend
cd backend
cp .env.example .env              # edit DB credentials
go run ./cmd/server

# Frontend (in another terminal)
cd frontend
npm install
npm run dev
```

Database setup, migrations, and production deployment are covered in `DEVELOPMENT.md`.

## Development Workflow

Read `DEVELOPMENT.md` for the phased roadmap, and `CLAUDE.md` + `.claude/rules/` for coding conventions. In short:

- Work on throwaway branches named `claude/<short-topic>`.
- Commit in small, logically atomic units with conventional-commit subjects.
- Every function ships with unit tests.
- Never force-push; never touch `main` directly.

## License

TBD.