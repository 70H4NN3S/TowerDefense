# Code style

Follow the canonical community standards for each language. When the standard is ambiguous, the rules below are tie-breakers.

## Go

### Non-negotiables

- Run `gofmt -s` (or `goimports`) and `go vet` before every commit.
- Run `staticcheck ./...`; resolve every diagnostic or silence it with a justification comment.
- Pass `go test ./... -race`.

### Idioms we follow

- **Effective Go** and the **Google Go style guide** are authoritative. If anything below contradicts them, they win.
- Package names are short, lowercase, and singular (`auth`, not `authentication` or `auths`).
- Exported identifiers have doc comments starting with the identifier name.
- Error strings are lowercase and do not end with punctuation: `fmt.Errorf("user not found")`.
- Receiver names are a consistent 1–2 letters across a type: `func (u *User) ...`.
- Context is the first parameter: `func Fetch(ctx context.Context, id UUID) (...)`.
- No unnecessary `else` after a `return` (early-return style).
- Prefer slices over arrays; prefer `[]T` over `*[]T`.
- `any` is the name of the empty interface; don't spell `interface{}`.

### File organization

- One type per file when the type is substantial (service, aggregate). Small helpers can share a file.
- Tests live next to the code: `foo.go` → `foo_test.go`.
- Table-driven tests use `tt` as the range variable and the subtests are named via `t.Run(tt.name, ...)`.

### What not to do

- No panics in library code. `log.Fatal` is only allowed in `main`.
- No global mutable state. Dependencies are injected via constructor functions.
- No init functions that do work (registering flags, opening files, etc). `init` is reserved for pure registrations like protobuf types — which we don't use anyway.
- No struct embedding for "behavior inheritance"; prefer composition with named fields.
- No `interface{}` "just in case". Interfaces are declared on the consumer side.

### Logging

- Use `log/slog`. Every log line has a structured key/value list, never a formatted string with interpolation.

  ```go
  slog.Info("match started", "match_id", m.ID, "mode", m.Mode)   // ✅
  slog.Info(fmt.Sprintf("match %s started", m.ID))                // ❌
  ```

- Log levels: `Debug` for developer breadcrumbs, `Info` for state transitions, `Warn` for recoverable abnormalities, `Error` for failed operations. Nothing else uses `Error`.

## TypeScript / React

### Non-negotiables

- `npm run lint` and `npm run format` clean.
- `npm run typecheck` clean (no `any`, no `@ts-ignore` without a justification comment).
- `npm test` green.

### Idioms

- Airbnb + Prettier defaults, modified only where noted.
- `const` by default; `let` only when reassignment is required.
- Prefer named exports. `default` exports only for lazy-loaded route components.
- Hooks named `useThing`, pure functions named `thing`, components PascalCase.
- Props interfaces end with `Props`. Avoid `React.FC`; prefer explicit `function Foo(props: FooProps) { ... }`.
- Never `useEffect` for derived state. Derive during render.
- Never `useEffect` for event handling. Handle in the event callback.

### React specifically

- Components do **one** thing. Split aggressively.
- No business logic in components. Put it in hooks (`useMatchmaking`, `useChat`) or plain TS modules (`src/lib/`, `src/state/`).
- Lists always have keys derived from stable IDs, never array indices.
- Don't pass `setState` down; pass event handlers with intent-named props (`onPurchaseTower`).

### Pixi.js

- Keep rendering separate from logic. Pure game logic (pathing, targeting, simulation tick) lives in `src/game/logic/` and is tested with Vitest. Pixi objects live in `src/game/render/`.
- Never hold references to React state inside a Pixi ticker. Pass data in via an explicit update method.
- Dispose everything: destroy textures, stop tickers, remove event listeners on unmount.

## Common rules

### Comments

- Comments explain **why**, not **what**. The code says what.
- Every exported identifier is documented.
- `TODO:` comments must include the author and context, and a matching entry in `docs/followups.md`.

  ```go
  // TODO(claude, 2026-04-20): backfill trophies for legacy accounts; see docs/followups.md#legacy-trophies
  ```

### Formatting

- UTF-8, LF line endings, final newline, no trailing whitespace.
- `.editorconfig` enforces these; don't override it locally.

### Naming

- No abbreviations except universally understood ones (`id`, `url`, `http`). Don't invent new ones.
- Booleans are predicates: `isReady`, `hasUnreadMessages`, `canUpgrade`. Not `ready`, `flag`, `status`.
- Avoid Hungarian notation (`strName`, `arrTowers`).