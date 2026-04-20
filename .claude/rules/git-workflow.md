# Git workflow

The human rebases branches personally. Your job is to make that rebase painless.

## Golden rules

1. **Never touch `main`.** Don't commit to it, don't merge into it, don't rebase it, don't delete it.
2. **Never force-push** a branch that has already been pushed, unless the only commits on the remote are your own and you know the human hasn't pulled yet. When in doubt, ask.
3. **Work on `claude/<short-kebab-topic>`.** One branch per logical unit of work (one milestone, one bug, one feature).
4. **Small commits.** A commit is a unit of review, not a unit of "when I remembered to save". Aim for < 200 lines of diff per commit.
5. **Every commit builds and tests green in isolation.** A reviewer should be able to `git checkout <any-commit>` and run the test suite successfully.

## Branch naming

```
claude/<topic>
```

- Lowercase, kebab-case, descriptive, short.
- Good: `claude/auth-jwt`, `claude/wave-spawner`, `claude/alliance-invite-flow`.
- Bad: `claude/fixes`, `claude/wip`, `claude/update`.
- If a branch grows beyond one milestone, stop and split it. The human shouldn't have to untangle a mega-branch.

## Commit messages

Conventional commits. The subject line is 72 characters or fewer, present tense, imperative mood, no trailing period.

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

| Type       | Use for                                                              |
| ---------- | -------------------------------------------------------------------- |
| `feat`     | A new capability visible to the user or another package.             |
| `fix`      | A bug fix.                                                           |
| `test`     | Adding or improving tests without changing production behaviour.     |
| `refactor` | Code restructuring with no behaviour change.                         |
| `docs`     | Documentation only.                                                  |
| `chore`    | Tooling, config, dependencies, build scripts.                        |
| `perf`     | A performance improvement.                                           |
| `style`    | Formatting/whitespace only. (Rare — usually `gofmt` handles this.)   |

### Scope

Optional, lowercase, matches a package or area: `auth`, `db`, `ws`, `towers`, `ui`, `shop`, `match`.

### Examples

```
feat(auth): add jwt signing and verification

Implements HS256 sign/parse without an external dependency. Tokens include
iss, sub, exp, and iat. Tests cover happy path, tampering, expiry, and
clock skew.
```

```
fix(towers): reject duplicate purchase when request arrives twice

Inserts into owned_towers now run inside a transaction with a unique
constraint so the second request returns ErrAlreadyExists instead of
silently debiting diamonds again.

Fixes: #42
```

```
test(resources): cover concurrent spend with goroutines
```

### Body

- Explain **why** the change was made. The diff shows what.
- Wrap at 80 columns.
- If the commit is a pure mechanical change (rename, move), a one-line subject is fine.

### Footers

- `Refs: <link or issue>` — for context.
- `Breaking-Change: <description>` — capitalized, only when there's a genuine breaking change to a public interface.

## What belongs in one commit

A commit is one **coherent** change. A useful heuristic: can you describe it in one conventional-commit subject line without using "and"? If yes, it's one commit. If no, split it.

Split when:

- You moved a file **and** changed its contents: two commits (move first, then edit).
- You added a feature **and** refactored something else: two commits.
- You fixed a bug **and** noticed a typo: two commits.

## Commit order within a branch

The branch should read as a story:

1. Mechanical prep (renames, file moves, extracting helpers).
2. Tests that reproduce the current behaviour or express the new requirement.
3. The production change that makes the tests green.
4. Documentation.

If you need to reorder later, use `git rebase -i` **before pushing**. After pushing, only the human reorders.

## What never goes in a commit

- Secrets (`.env`, API keys, tokens, private certs). They belong in `.gitignore` from day one.
- Generated artifacts (`node_modules/`, `build/`, `dist/`, `*.log`).
- IDE files (`.vscode/settings.json`, `.idea/`) — unless shared intentionally, in which case they live in a dedicated PR.
- Commented-out code blocks. Delete them; git remembers.
- `WIP` or `temp` commits. Squash them before pushing.

## Before every commit

```
# Backend
cd backend
go fmt ./...
go vet ./...
staticcheck ./...
go test ./... -race -count=1

# Frontend
cd frontend
npm run lint
npm run typecheck
npm test
```

If any of these fail, **do not commit**.

## Pushing

- Push the branch: `git push -u origin claude/<topic>`.
- **Do not open a pull request.** The human will rebase and merge locally.
- If you pushed and then realized you need to amend, prefer adding a new commit over `git commit --amend` + force-push. The human cares more about a clean rebase than a pristine commit history.

## When the human has already rebased

If `main` has moved since you branched and you need to pick up changes:

1. `git fetch origin`
2. `git rebase origin/main` (resolve conflicts carefully)
3. **Do not** push the rebased branch back. Ask the human how they want to proceed — they may already have your commits in a different form.

## If you mess up

- If you accidentally committed to `main`, **stop immediately** and surface the mistake. Don't try to fix it silently.
- If you accidentally committed a secret, **stop immediately**. The secret must be rotated even if the commit is later removed from history.
- If you force-pushed over shared history, **stop immediately** and tell the human exactly which SHAs were lost.