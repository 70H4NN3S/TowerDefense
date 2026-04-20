# /review

Self-review checklist. Run this against your own branch before telling the human a task is done.

## How to run

Walk down each section and answer honestly. If an item fails, **fix it before reporting the task complete**. If an item doesn't apply, say so and why.

Output the results as a short report:

```
/review <branch-name>

Code quality:     ✅
Tests:            ⚠️  (see notes)
Git hygiene:      ✅
Security:         ✅
Plan alignment:   ✅

Notes:
- Missing test for the error branch in profileService.updateAvatar;
  added it in commit abc1234.
```

---

## 1. Code quality

- [ ] Every function has a docstring / Go doc comment explaining what it does and what it returns.
- [ ] No dead code, no commented-out blocks, no `TODO` without a follow-up ticket.
- [ ] Naming: exported identifiers read well; unexported ones are scoped tightly.
- [ ] No `panic` in production paths. Panics exist only in `main` setup code or truly unrecoverable states.
- [ ] No `interface{}` / `any` where a concrete type would work.
- [ ] Imports are grouped: stdlib, external, internal. No unused imports.
- [ ] Line length is reasonable (~100 cols soft limit).

## 2. Tests

- [ ] Every new or modified function has at least one unit test.
- [ ] Tests cover: happy path, boundary conditions, error paths.
- [ ] Table-driven tests are used where cases share structure.
- [ ] No test depends on ordering, clock time (use injected clocks), or network.
- [ ] `go test ./... -race -count=1` passes locally.
- [ ] `npm test` passes locally.
- [ ] Coverage for touched packages is ≥ 85 %.

## 3. Error handling

- [ ] Errors are wrapped with `fmt.Errorf("context: %w", err)` when they cross a layer.
- [ ] Sentinel errors are declared at package level and matched with `errors.Is`.
- [ ] HTTP handlers translate errors into the standard error envelope (see `.claude/rules/error-handling.md`).
- [ ] No error is silently swallowed. If you *intentionally* discard one, there is a comment explaining why.

## 4. Database

- [ ] Queries use parameterized statements; no string concatenation of user input.
- [ ] New tables have primary keys, `created_at`/`updated_at` where relevant, and sensible indexes.
- [ ] Migrations have a matching `.down.sql`.
- [ ] Transactions are used for multi-statement writes.
- [ ] No N+1: joins or batched queries for list endpoints.

## 5. Security

- [ ] Every authenticated endpoint pulls the user ID from context and scopes queries to it (no trusting path/body user IDs).
- [ ] All user input is validated (length, type, range, enum membership).
- [ ] Passwords, tokens, and secrets never appear in logs or error messages.
- [ ] Rate limits exist on auth and write-heavy endpoints.
- [ ] See `.claude/rules/security.md` for the full matrix.

## 6. Git hygiene

- [ ] Branch name is `claude/<topic>`.
- [ ] Commits are small, logically atomic, and each one builds green.
- [ ] Subject lines follow conventional-commit format (`feat:`, `fix:`, `test:`, `docs:`, `refactor:`, `chore:`).
- [ ] Commit bodies explain **why** when it isn't obvious from the diff.
- [ ] No merge commits on the branch. No force-pushes over shared history.
- [ ] `main` is untouched.

## 7. Plan alignment

- [ ] The work corresponds to a specific milestone in `DEVELOPMENT.md`.
- [ ] If you deviated (added scope, skipped a step), it's documented in the commit body or `docs/followups.md`.
- [ ] Nothing in this branch should be a surprise to the human reviewer.

## 8. Documentation

- [ ] If you changed a public interface, you updated its doc comment.
- [ ] If you changed the setup or run instructions, you updated `README.md`.
- [ ] If you introduced a new architectural decision, you added a short ADR in `docs/adr/`.

---

## Final question

> "If the human pulled this branch right now and tried to rebase it, would anything surprise them?"

If the answer is yes, flag it explicitly at the top of your report.