# Error handling

Errors are first-class values. Treat them as you would any other data: name them, inspect them, pass them around deliberately.

## Go

### Return errors; don't panic

- Production code does not panic. Recover panics only at the outermost server boundary (a middleware) so one bad request doesn't kill the process.
- `log.Fatal` is banned outside `main`.
- `panic` inside a library is a bug, not a feature.

### Wrap with context

Every time an error crosses a function boundary, wrap it with context:

```go
if err := repo.GetUser(ctx, id); err != nil {
    return fmt.Errorf("fetch user %s: %w", id, err)
}
```

- Always use `%w` so `errors.Is` and `errors.As` work up the stack.
- The wrapping message answers "what was I doing?", not "what went wrong?". The wrapped error says what went wrong.
- Don't double-wrap. If a caller already said "fetch user", don't say it again.

### Sentinel errors

Package-level sentinel errors for conditions callers need to branch on:

```go
var (
    ErrNotFound        = errors.New("not found")
    ErrAlreadyExists   = errors.New("already exists")
    ErrInsufficientFunds = errors.New("insufficient funds")
)
```

Consumers match with `errors.Is(err, auth.ErrNotFound)`. Never match by string.

### Typed errors for data

When an error carries fields (a validation error listing field names), use a struct implementing `error`:

```go
type ValidationError struct {
    Field string
    Reason string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}
```

Extract with `errors.As`.

### HTTP error envelope

All API errors serialize to the same shape:

```json
{
    "error": {
        "code": "insufficient_funds",
        "message": "Not enough gold to upgrade this tower.",
        "details": { "required": 1200, "available": 800 }
    },
    "request_id": "01HVZJ..."
}
```

- `code` is a machine-readable `snake_case` identifier that matches a sentinel error in the backend.
- `message` is human-readable and safe to show to end users. Localization happens client-side.
- `details` is optional and always a JSON object (never an array or scalar) so clients can rely on the shape.
- **Never** leak internal error messages, SQL snippets, stack traces, or file paths in `message`. Log those server-side instead.

Mapping happens in one place (`internal/httpserver/respond.go`):

```go
func RespondError(w http.ResponseWriter, r *http.Request, err error) {
    switch {
    case errors.Is(err, auth.ErrNotFound):
        writeJSON(w, http.StatusNotFound, envelope{Code: "not_found", Message: "Resource not found."})
    case errors.Is(err, game.ErrInsufficientFunds):
        writeJSON(w, http.StatusConflict, envelope{Code: "insufficient_funds", Message: "Not enough resources."})
    default:
        slog.ErrorContext(r.Context(), "unhandled error", "err", err)
        writeJSON(w, http.StatusInternalServerError, envelope{Code: "internal", Message: "Something went wrong."})
    }
}
```

### Don't swallow errors

- Every `_ = foo()` call has a comment explaining why the error is being discarded.
- `defer someCloser.Close()` is acceptable for reads that have already consumed their data; for writes, check the close error.
- `if err != nil { return nil }` is almost always a bug. If you really mean "ignore this error", say so explicitly with a comment.

### Validation errors

- Validate at the edge (HTTP handler or WebSocket message decoder). Return `400 Bad Request` with a `validation_failed` code and a `details.fields` array listing offenders.
- Services return typed errors for domain invariants (e.g. `ErrInsufficientFunds`), not validation errors.

## TypeScript

### Errors are exceptional, but predictable

- Throw only for **unexpected** conditions (programmer errors, invariant violations).
- For expected failure modes (network failure, validation errors from the server), return a `Result<T, E>` or use a discriminated union:

  ```ts
  type Result<T, E = ApiError> =
      | { ok: true; value: T }
      | { ok: false; error: E };
  ```

- Prefer this over throw/catch for domain-level errors. Throw/catch is for genuine exceptions.

### API client

- The API client (`src/api/client.ts`) converts HTTP errors into typed `ApiError` objects preserving `code`, `message`, and `details` from the envelope.
- Never let a raw `Response` escape the client layer.

### React error boundaries

- Wrap each top-level screen in an error boundary. The boundary shows a recovery UI and reports the error.
- Do not let React errors crash the whole app.

### Async rules

- Every `async` call that could fail has a handler. `await` without a surrounding `try/catch` or a `Result` pattern is suspect.
- `.catch()` chains must end with either re-throwing or handling. No naked `.catch(() => {})`.

## Logging errors

- Errors are logged **once**, at the layer that handles them (usually the HTTP response layer). Don't log-then-return.
- Log with structured fields: `slog.Error("purchase failed", "err", err, "user_id", uid, "template_id", tid)`.
- **Never** log user passwords, session tokens, email addresses (unless specifically needed and approved), or payment details.

## User-facing error messages

- Short. Specific. Actionable when possible.
- "Not enough gold to upgrade this tower." — good.
- "Internal server error (500)" — bad. Use our code + a friendly string.
- Never reveal whether a user exists during login. "Invalid email or password" regardless of which was wrong.