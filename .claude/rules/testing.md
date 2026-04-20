# Testing

**Every function ships with tests.** This is not aspirational — it's a check gate. A commit that adds a function without a test is an incomplete commit.

## Targets

- **Backend package coverage:** ≥ 85 % lines, ≥ 80 % branches per package.
- **Frontend unit coverage:** ≥ 80 % for `src/game/logic/` (pure code) and ≥ 70 % for hooks and state modules.
- **Critical paths have integration tests:** auth, purchase, match result submission, matchmaking, chat message delivery.

Coverage is a floor, not a goal. Don't write meaningless tests to hit a percentage.

## Go

### Tooling

- `go test` from the stdlib. No Ginkgo, no Testify (including `assert` and `require`). The `if got != want { t.Errorf(...) }` pattern is clear enough and forces honest comparison logic.
- `testing.T.Helper()` in test helpers.
- `t.Parallel()` wherever safe.
- `-race` is always on (CI + local pre-commit).

### What to test

Every exported function. Every unexported function with non-trivial logic. For each function:

1. **Happy path** — the normal case produces the expected output.
2. **Boundaries** — empty inputs, zero, negative, max int, unicode strings.
3. **Errors** — every error path the function can return.
4. **Side effects** — DB writes, logs, counters; verify they happened (or didn't) as expected.

### Structure

Table-driven tests are the default:

```go
func TestLevelUpCost(t *testing.T) {
    tests := []struct {
        name    string
        level   int
        want    int
        wantErr error
    }{
        {"level 1 -> 2", 1, 100, nil},
        {"level 9 -> 10", 9, 9000, nil},
        {"already max", 10, 0, ErrMaxLevel},
        {"invalid level", 0, 0, ErrInvalidLevel},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            got, err := LevelUpCost(tt.level)
            if !errors.Is(err, tt.wantErr) {
                t.Fatalf("err = %v, want %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("cost = %d, want %d", got, tt.want)
            }
        })
    }
}
```

### Mocks and fakes

- Prefer **fakes** (small in-memory implementations) over mocks.
- Define interfaces **at the point of consumption**, not alongside the implementation. This is idiomatic Go and makes fakes trivial.
- If you're tempted to reach for a mocking library, stop. Write a 30-line fake. It will last longer and read better.

### Integration tests

- Live in `*_integration_test.go` files, guarded by a build tag: `//go:build integration`.
- Hit a real PostgreSQL instance. Each test starts in a transaction that is rolled back on cleanup, or creates a schema with a random suffix that is dropped on cleanup.
- Run with `go test -tags=integration ./...`.

### Test helpers

- Put shared helpers in `*_test.go` files; they are compiled only for tests and are not exported outside the package.
- When a helper is used in multiple packages, put it in an `internal/testutil/` package.

### What not to test

- Generated code.
- Trivial getters (`func (u User) ID() uuid.UUID { return u.id }`) — but only if they stay trivial. Add a test the moment logic creeps in.
- Third-party library behaviour.

### HTTP tests

- `net/http/httptest.NewRecorder()` for unit-testing handlers.
- `httptest.NewServer` for full end-to-end tests including the router and middleware.
- Assert on status code, response body (decoded, never regex-on-JSON), and relevant headers.

### WebSocket tests

- Spin `httptest.NewServer` with the WS handler.
- Use `github.com/gorilla/websocket.Dialer` from the test.
- Assert on message sequence, types, and (critically) timing invariants that don't rely on wall-clock sleeps — use deterministic clocks injected into the server.

### Deterministic time

- Never use `time.Now()` directly in testable code. Inject a `Clock interface { Now() time.Time }` and provide a real implementation in production, a fake in tests.
- Same rule for random: inject a `*rand.Rand` or a `Rand interface`.

### Concurrency

- For code with goroutines and shared state, add tests that run under `-race` with high iteration counts (`-count=100`) locally when initially validating.
- Use `testing/synctest` (Go 1.24+) when simulating time-dependent coordination.

## TypeScript

### Tooling

- Vitest + React Testing Library.
- `jsdom` environment for component tests; `node` environment for pure logic tests.
- `@testing-library/user-event` for realistic interaction simulation.

### What to test

- **Pure logic (`src/game/logic`, `src/lib`):** everything, like Go — happy path, boundaries, errors.
- **Hooks:** behaviour under different inputs, cleanup on unmount, error states.
- **Components:** render output, user interaction (click, type, submit), integration with mocked hooks.
- **API client:** request shape, response parsing, error normalization, retry behaviour.

### Component tests

- Query by accessible role, not by test ID when possible (`getByRole('button', { name: /upgrade/i })`).
- Interact via `userEvent`, not `fireEvent`, unless you specifically need low-level control.
- Never assert on implementation details (CSS classes, internal state names). Assert on what the user sees and can do.

```ts
test('buy button is disabled when diamonds are insufficient', () => {
    render(<ShopItem price={500} diamonds={100} />);
    expect(screen.getByRole('button', { name: /buy/i })).toBeDisabled();
});
```

### Mocks

- Mock only at module boundaries (the API client, the WebSocket client, Capacitor plugins).
- Don't mock React itself. Don't mock the component you're testing.
- Use MSW for HTTP if the test needs realistic request/response behaviour; otherwise inject a typed fake client.

### Flakiness

- No `setTimeout`-based waits. Use `await waitFor(...)` with a clear assertion.
- No "just run it a few times" to make a flaky test pass. A flaky test is a bug.
- Tests must be order-independent and parallelizable.

## Shared principles

### Arrange / Act / Assert

Structure every test in three visible sections — with blank lines or comments. Reviewers should see immediately where each phase ends.

### One behaviour per test

A test has exactly one reason to fail. If you're tempted to assert five unrelated things, write five tests.

### Test names describe behaviour

```
// good
TestPurchase_RejectsWhenInsufficientDiamonds
TestSubmitResult_AwardsTrophiesOnWin

// bad
TestPurchase1
TestHandler
```

### Test fixtures

- Minimal data. Build the smallest object the test needs; don't paste realistic payloads with 40 irrelevant fields.
- Use helper constructors: `newTestUser(t, withGold(1000))`.

### Flakes are emergencies

A flaky test is quarantined within 24 hours and fixed or deleted within a week. Never suppress a flake with `retry(3)`.

### Do not mock what you can construct

If testing `UpgradeTower(user, template)` needs a user and a template, construct them. Don't mock a database just to give the function something to read.