# /fix-issue

Playbook for fixing a bug, regression, or reported issue.

## Inputs

The user invokes this with a short description, a stack trace, a failing test, or a reference to a ticket. Treat whatever they provide as the **symptom**, not the issue itself.

## Procedure

### 1. Reproduce before fixing

- Write a **failing test** that reproduces the bug. Commit it on its own: `test: reproduce <bug>`.
- If the bug can't be expressed as a unit test, explain why and write the narrowest integration test that reproduces it.
- If you can't reproduce it, stop and report this back to the user with your investigation notes. Do not "fix" a ghost.

### 2. Diagnose

- Trace the failing test down to a single root cause. Write the root cause in one sentence before writing any fix.
- If the root cause reveals a broader class of bugs (e.g. "every endpoint that takes a UUID is vulnerable to this"), note the scope but keep the current fix focused. File a follow-up in `docs/followups.md`.

### 3. Fix

- Make the **smallest change** that turns the failing test green.
- Do not take the opportunity to refactor nearby code. Refactors are separate commits, ideally separate branches.
- Run the full test suite after the fix: `go test ./... -race` and `npm test`.

### 4. Verify no regression

- Read the git log around the affected area. If the bug was introduced by a specific commit, note which one in your commit message.
- If fixing the root cause made other failing tests pass that weren't in your scope, investigate before celebrating — they may have been masking something else.

### 5. Commit shape

Two commits minimum:

```
test: reproduce <issue>          # the failing test, alone
fix: <one-sentence root cause>   # the minimal fix, making the test pass
```

If the fix requires a follow-up (better test coverage around the fix, documentation update), add a third commit:

```
docs: note <follow-up>
```

### 6. Report back

When the fix is complete, report:
- One-sentence root cause.
- The commits you made, in order.
- Any follow-ups you recorded in `docs/followups.md`.
- Anything that surprised you during the investigation (those are usually the real bugs).

## Things to avoid

- **Do not** "fix" a failing test by changing the test's assertions unless you can explain why the original assertion was wrong.
- **Do not** wrap the bug in a try/catch, `if err != nil { return nil }`, or similar suppression. Errors should be handled, not hidden.
- **Do not** add feature flags or config toggles just to disable the buggy path. Fix the path.
- **Do not** bundle unrelated cleanups into the fix commit.