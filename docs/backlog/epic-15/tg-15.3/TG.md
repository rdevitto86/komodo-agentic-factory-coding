## [TG-15.3] Nothing stale loads into a session [P: C] [BLOCKED]

```yaml
type: fix
depends_on: [TG-15.1]
```

> **Blocked** 2026-10-05 12:38, run TG-15.3-1791216196, at Shipped.
> - gh pr view --json number,url,state,title,isDraft: exit status 1: could not determine current branch: failed to run git: not on any branch
> - Needs: A person must decide whether TG-15.3 already landed (HEAD at ffcf41e9 matches main's tip in the snapshot, so the branch may have been merged and deleted already) or whether the branch needs recreating from ffcf41e9 so a PR can be opened — recreating or retargeting a ref is a critical-ref decision outside this escalation's edit scope.
> - Saved: WIP commit `ffcf41e9` on `fix/TG-15.3-nothing-stale-loads-into-a-session`

