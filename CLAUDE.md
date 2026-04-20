# Instructions for Claude

You are working on a multiplayer tower-defense mobile game. Read `README.md` for the product overview and `DEVELOPMENT.md` for the phased roadmap. This file is your operating manual; obey it over any conflicting instinct.

## Before you do anything

1. Re-read the relevant rule files under `.claude/rules/`. At minimum, always read:
   - `.claude/rules/git-workflow.md`
   - `.claude/rules/code-style.md`
   - `.claude/rules/testing.md`
2. Identify which phase/milestone of `DEVELOPMENT.md` the current task belongs to. If the task isn't covered, pause and ask.
3. Make sure you're on a `claude/<topic>` branch. If you're on `main`, stop and create one. Never touch `main`.

## Tech stack — non-negotiable

- **Backend:** Go (latest stable), PostgreSQL. Use the standard library aggressively. The **only** external dependencies permitted in production code are:
  - `github.com/jackc/pgx/v5` (and its subpackages)
  - `github.com/gorilla/websocket`
  - `golang.org/x/crypto/bcrypt`

  If you think you need anything else, **stop and ask**. This is a hard rule, not a preference. No web frameworks (`gin`, `echo`, `fiber`, `chi`), no ORMs (`gorm`, `ent`, `sqlx`, `sqlc` — well, `sqlc` is a code generator and can be reconsidered if explicitly approved), no logging frameworks (`zap`, `logrus`), no validation libs (`validator`, `ozzo`). Use `net/http`, `database/sql`, `encoding/json`, `log/slog`.

- **Frontend:** React + TypeScript (Vite), Capacitor, Pixi.js. The frontend has looser library rules since the ecosystem requires it, but prefer the stdlib / Web APIs when possible (e.g. `fetch` over `axios`, native `Intl` over `date-fns` when you can).

## Workflow you must follow

1. **Branching.** All work happens on `claude/<short-kebab-topic>`. Branch from the current `main`. Never rebase, force-push, or touch `main`.
2. **Commits.** Small, logically atomic, conventional-commit subject lines. Each commit should build and test green on its own.
3. **Tests.** Every function gets a test. Coverage target is 85 %+ for backend packages. See `.claude/rules/testing.md`.
4. **Before committing:** run the full test suite (`go test ./... -race` and `npm test`) plus formatters and linters.
5. **Never open a PR.** The human rebases branches personally.

## House rules

- **Ask when uncertain.** If a task is ambiguous, you require information you don't have, or a decision has product implications, stop and surface the question. Don't guess.
- **Small changes.** Prefer 5 commits of 50 lines each over 1 commit of 250 lines.
- **Read before you write.** Check whether the helper you're about to build already exists.
- **Don't reinvent the stdlib.** If you find yourself writing a utility that `slices`, `maps`, `strings`, `sort`, or `cmp` already provides, use theirs.
- **No dead code.** Don't commit commented-out blocks, `TODO` without an owner, or speculative abstractions.
- **No silent drift from the plan.** If a task drags you outside its milestone, note it, complete the minimum, and open a follow-up note in `docs/followups.md`.

## Available slash commands

- `/fix-issue <description>` — see `.claude/commands/fix-issue.md` for the playbook.
- `/review` — see `.claude/commands/review.md`. Use this on yourself before declaring a task done.

## Rules index

| File                                           | When it applies                              |
| ---------------------------------------------- | -------------------------------------------- |
| `.claude/rules/code-style.md`                  | Every time you write code.                   |
| `.claude/rules/project-structure.md`           | When creating files or moving code.          |
| `.claude/rules/database.md`                    | Any schema, migration, or query work.        |
| `.claude/rules/error-handling.md`              | Anywhere errors can happen.                  |
| `.claude/rules/testing.md`                     | Every function you touch.                    |
| `.claude/rules/git-workflow.md`                | Every commit, every branch.                  |
| `.claude/rules/security.md`                    | Auth, input handling, data exposure, crypto. |

## When a human message is ambiguous

Default to asking a focused clarifying question instead of making an assumption that might cost a rebase. State the choices you see and request a decision.