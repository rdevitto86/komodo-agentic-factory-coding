# Skill: /git-flow

Komodo git branching and PR workflow reference.

## Branch naming

| Type | Pattern | Example |
|------|---------|---------|
| Feature | `feature/<ticket-id>-short-description` | `feature/KOM-42-user-auth` |
| Bug fix | `fix/<ticket-id>-short-description` | `fix/KOM-91-null-session` |
| Chore | `chore/<short-description>` | `chore/update-deps` |
| Hotfix | `hotfix/<ticket-id>-short-description` | `hotfix/KOM-105-checkout-crash` |
| Release | `release/v<semver>` or `release/YYYY-MM-DD` | `release/v1.4.0` |

Branch from `main` for features, fixes, and chores. Branch from the relevant `release/*` branch for hotfixes.

---

## Commit messages

Conventional commits — required on all commits:

```
<type>(<scope>): <subject>

[optional body]
```

**Types:** `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`

**Rules:**
- Subject line ≤72 characters, lowercase, no trailing period
- Scope is the affected service or module (optional but encouraged)
- Body explains *why*, not *what* — the diff shows the what

**Examples:**
```
feat(auth): add refresh token rotation
fix(checkout): handle nil cart on guest session
chore: upgrade golangci-lint to v1.57
refactor(orders): extract pricing logic into service layer
```

---

## Pull requests

**Title:** Match commit format — `type(scope): subject`.

**Description must include:**
- **What** — summary of changes (2–4 sentences)
- **Why** — motivation or link to Trello card/issue
- **How to test** — steps or test commands to verify the change
- **Rollback** — how to undo if it breaks in production

**Rules:**
- No self-merges — every PR requires at least 1 approval (2 for auth, payments, or data schema changes)
- All CI checks must pass before merge
- No unresolved blocking review comments
- Squash-merge to `main` by default; preserve history only on `release/*` merges
- Link the Trello card in the description

---

## Release branching

1. Cut `release/v<semver>` from `main`
2. Only hotfixes and documentation changes merge into a release branch post-cut
3. Merge release branch back to `main` after shipping
4. Tag the release commit: `git tag v<semver>`

---

## Hotfix flow

1. Branch from the affected `release/*` branch: `hotfix/KOM-xxx-description`
2. Fix, test, PR → merge into `release/*`
3. Cherry-pick the fix commit to `main`
4. Tag if the release branch is already shipped

---

## Protected branches

`main` and all `release/*` branches are protected — no direct push. All changes via PR.

---

## Checklist before opening a PR

- [ ] Self-reviewed the diff — no debug code, no commented-out blocks, no TODOs without a ticket
- [ ] Tests written and passing locally
- [ ] CI passing on the branch
- [ ] Trello card linked in description
- [ ] Branch name follows convention
