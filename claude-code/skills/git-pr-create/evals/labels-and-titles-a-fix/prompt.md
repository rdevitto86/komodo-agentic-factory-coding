---
max_turns: 10
allowed_tools: [Read, Grep, Glob, Skill]
---

You are about to open a PR for the current branch. Here is everything you need instead of running git or gh yourself:

- Base branch: `main`. Current branch: `fix/retry-timeout`, three commits ahead.
- Commit subjects, oldest to newest:
  1. `fix: retry request timeout instead of failing immediately`
  2. `test: cover the retry timeout path`
  3. `fix: correct retry backoff off-by-one`
- `git diff main..HEAD --stat` touches only `internal/client/retry.go` and `internal/client/retry_test.go`.
- The repo has `.github/PULL_REQUEST_TEMPLATE.md` with sections `Summary`, `Changes`, `Validation Evidence`, `Dependencies`.
- `gh label list` returns: `Bug`, `Enhancement`, `Documentation`, `Skill`, `@agent`, `Duplicate`, `Do not merge`.
- No `--labels` was passed to you.

State the PR title you would use, and which label(s) you would apply and why. Do not run any git or gh command — you already have everything above.
