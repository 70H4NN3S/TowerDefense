# Security

A multiplayer game with real-money purchases and user-generated content is a worthwhile target. Assume every request is hostile until proven otherwise.

## Authentication

### Passwords

- Hash with bcrypt (`golang.org/x/crypto/bcrypt`), cost 12 minimum. Increase when `time.Benchmark` against an average server shows headroom.
- **Never** store plaintext passwords, even in logs, tests, or error messages.
- Enforce minimum length (12 characters) and a check against a banned-common-password list. Do not enforce silly composition rules ("must contain one uppercase and one digit"); length + common-list is stronger and less hostile.
- Email and username are citext-unique to prevent confusables.

### Tokens

- JWT with HS256. The secret lives in `JWT_SECRET` env var, never in code.
- Access tokens expire in 1 hour. Refresh tokens expire in 30 days.
- Refresh tokens are rotated on use (old one invalidated).
- Include `jti` (token ID) so individual tokens can be revoked via a small allow/deny table if we ever need it.
- Validate every claim on every request: `exp`, `iat`, `iss`, `sub`. Reject tokens with extra unexpected claims.

### Authorization

- Every authenticated endpoint pulls `userID` **from the validated token**, never from the request path, query, or body.
- When an action targets a resource owned by a user, verify ownership in the same query as the mutation, not as a separate check — `UPDATE towers SET level = level + 1 WHERE id = $1 AND user_id = $2`.
- Admin endpoints don't exist in the first release. When they do, they sit under `/v1/admin/...` behind an explicit role check.

## Input validation

All external input is untrusted. That includes:

- HTTP request bodies, query params, headers, path params.
- WebSocket message payloads.
- Data read back from the database **if it originated from a user** (e.g. chat messages being broadcast).

### What to validate

- **Type** — decode with `encoding/json` into concrete typed structs; reject unknown fields with `DisallowUnknownFields()`.
- **Length** — every string has a max. Display names: 32 chars. Chat messages: 500 chars. Alliance names: 24 chars. Reject longer.
- **Range** — every integer has a min/max. Tower level 1–10. Matchmaking bucket 0–50. Reject out-of-range.
- **Enum** — every "type" or "kind" field is checked against a whitelist of values.
- **Format** — emails with a conservative regex + domain sanity check. UUIDs parsed with `uuid.Parse`.
- **Semantic** — a player can only target a tower they own; a chat message can only be sent to a channel the user is a member of.

### Where to validate

Validate at the **edge** — the HTTP handler or WS message dispatcher. Domain services assume their inputs are well-formed.

## SQL injection

- Always use parameterized queries (`$1`, `$2`, …). **No exceptions.**
- Never format a query with `fmt.Sprintf("... WHERE id = %s", id)`. Ever.
- Schema names and table names are never derived from user input. If dynamic identifiers are unavoidable, whitelist them against a closed set.

## Output encoding

- JSON encoding via `encoding/json` handles escaping. Don't hand-roll JSON.
- HTML rendered in React uses JSX (safe by default). Never use `dangerouslySetInnerHTML` on user-generated content.
- Chat messages are rendered as plain text. Link detection happens in a controlled parser that emits `<a>` with `rel="noopener nofollow ugc"` and a whitelisted URL scheme.

## Rate limiting

- Per-IP rate limit on unauthenticated endpoints (`/register`, `/login`): 10 requests per minute, token bucket.
- Per-user rate limit on authenticated write endpoints: sensible per-endpoint defaults (e.g. chat: 10 messages / 10 seconds; matchmaking join: 3 per minute).
- Returned as `429 Too Many Requests` with a `Retry-After` header.
- Rate limiter is in-memory (stdlib-only); move to a shared store only when we horizontally scale beyond one server.

## CSRF / Origin

- Mobile clients send their token in an `Authorization` header, not a cookie. CSRF is not applicable there.
- If a web build ships, auth moves to secure cookies (`Secure`, `HttpOnly`, `SameSite=Lax`) and the API enforces a matching `Origin` header.

## TLS / Transport

- All traffic is HTTPS in production. The server does **not** listen on HTTP except for a single `/.well-known/` path for cert provisioning.
- HSTS on all responses in production.
- WebSocket upgrades happen over `wss://` only in production.

## Secrets management

- Secrets are loaded from env vars at startup. Never hard-coded, never committed.
- `.env.example` documents every required variable with a placeholder value; `.env` is in `.gitignore`.
- Production secrets come from the deployment platform's secret store (not the repo, not a sidecar file with a real value).
- Rotate any secret that might have been exposed, even briefly.

## Logging and telemetry

- **Never** log: passwords, password hashes, JWTs, refresh tokens, session cookies, full request bodies on auth endpoints, credit card numbers, or any secret.
- Log **user IDs**, not emails or usernames, unless there's a specific reason.
- Error messages returned to clients **never** include internal details (stack traces, SQL, file paths, env names).
- Request IDs are generated server-side per request and propagated via `X-Request-ID`.

## Game-specific anti-cheat

- **Server-authoritative simulation.** Multiplayer matches run on the server; clients send intents, the server executes them. Trust nothing the client claims about game state.
- **Single-player replay validation.** The client submits inputs + final state. The server reruns the simulation with the stored seed and verifies the final state matches. Mismatches → reject, no rewards.
- **Resource operations are transactional.** Gold/diamond mutations go through a single function that checks balance + writes atomically. No "check then write" race windows.
- **Rate limits on rewarded actions.** A player can't submit 100 match results per second.
- **Monotonic clocks for cooldowns.** Energy regeneration is computed from `energy_updated_at` stored in the DB. A client claiming "it's been 5 hours" gets checked against `now() - energy_updated_at`.

## Chat and UGC

- Display names and alliance names go through a profanity / confusable filter and a length check.
- Chat messages have a rate limit and a length cap.
- Reports + blocklist feature land with the first public build.
- Chat history retention policy documented in the privacy policy.

## Data handling

- PII (email) is stored only where necessary. Usernames are **not** PII in this game but email is.
- Deleting an account removes the email, password hash, display name → replaced with a tombstone user ("Deleted Player"). Historical match records keep the tombstone reference so stats remain consistent.
- Respect GDPR/CCPA deletion and export requests from day one, even if volume is tiny. Build the endpoint; don't do it manually.

## Dependency hygiene

- Backend dependencies are pinned in `go.sum` and reviewed on every update.
- `go mod tidy` is run before commit.
- `govulncheck ./...` is run periodically; vulnerabilities surfaced in production dependencies are triaged within one business day.
- Frontend deps are pinned (`package-lock.json` committed). `npm audit` is reviewed; high/critical issues get attention.
- **Any new dependency is a decision, not a reflex.** Justify it in the commit body. Prefer writing 50 lines over adding a 5 MB package. For the backend, the dependency allow-list is in `CLAUDE.md` and expanding it requires human approval.

## Security checklist per endpoint

Before marking an endpoint "done":

- [ ] Authenticated? Correct user ID source?
- [ ] Authorized? Ownership verified?
- [ ] Inputs validated (type, length, range, enum)?
- [ ] Rate limited?
- [ ] Parameterized queries?
- [ ] No sensitive data leaked in errors or logs?
- [ ] Tests cover the auth/authz/validation paths?

## Incident response

- If a vulnerability is found: **stop new commits on that area**, assess scope, decide rotation/disclosure needs, then fix. Don't silently patch.
- Post-mortem written in `docs/incidents/NNNN-<slug>.md` for anything user-visible or data-relevant.