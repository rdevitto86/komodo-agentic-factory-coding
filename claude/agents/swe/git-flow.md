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

Single-line, ≤256 characters total. No body, no footer — the diff is the detail and the ticket lives in the PR, not the commit.

```
<type>[(<scope>)]: <short change> [+ <type>[(<scope>)]: <short change> ...]
```

- **Type prefix is optional** — use it when it adds signal (a larger or mixed change benefits from labeling each part); omit it for small, single-purpose changes where it'd just be noise.
- **Scope** is the affected service/module — optional, in parens, only on the segments where it helps.
- **Multiple distinct changes in one commit** are joined with ` + `, each getting its own type/scope if useful. Prefer one logical change per commit when you can — the `+` form is for when a commit unavoidably bundles a few small, related changes.
- **Types** follow the common convention: `feat`, `fix` (or `bug`), `chore`, `docs`, `refactor`, `test`, `ci`. Lowercase, no trailing period.
- **No issue/ticket links here** — link the Trello card / JIRA / issue number in the PR title or description, not the commit message.

**Examples:**
```
fix(checkout): handle nil cart on guest session
chore: bump golangci-lint to v1.57 + bump go-sdk to v2.3
update lint config + remove dead helper
feat(auth): add refresh rotation + fix: session leak on logout + chore(deps): bump jwt lib
```

**When an agent drafts one:** agents never commit (see hard rules) — they draft a message and hand it to the user to commit. Draft one proactively when wrapping up a long, planned multi-step task (the kind tracked across `TaskCreate`/`TODO.md`/`MEMORY.md`) so the whole arc lands as one coherent commit instead of overlapping piecemeal ones. For small asks, give a quick one-liner in this same format on request — don't over-produce them.

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
