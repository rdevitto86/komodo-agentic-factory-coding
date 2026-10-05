## [TG-15.20] A builder's sandbox runs the repo's tests [P: C] [BLOCKED]

```yaml
type: fix
depends_on: []
```

> **Blocked** 2026-10-05 14:44, run TG-15.20-1791226815, at Shipped.
> - gh pr view --json number,url,state,title,isDraft: exit status 1: could not determine current branch: failed to run git: not on any branch
> - Needs: Decide how to restore the branch ref: create/checkout local branch `fix/TG-15.20-a-builder-s-sandbox-runs-the-repo-s-test` at a4249e44 (matching `refs/komodo/...`) and push it to origin so the Shipped step can run `gh pr view`/`gh pr create`.
> - Saved: WIP commit `a4249e44` on `fix/TG-15.20-a-builder-s-sandbox-runs-the-repo-s-test`

