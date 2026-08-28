---
name: workflow-complete
description: Publish the finished, committed band — push the branch and open or update its PR via git-pr-create.
argument-hint: []
---

# Workflow Complete — Publish

**Inside a git repository, on a non-default branch, with the band already committed** — if any of those isn't true, say so and stop. Committing is P3's job (`workflow-loop`'s P3 runs `/git-commit-message` and commits once `workflow-consolidate` returns); this phase never generates a commit message or runs `git commit`.

## Order

1. **Push** — `git push -u origin <branch>` (`git rev-parse --abbrev-ref HEAD` for the branch name).
2. **Run `/git-pr-create`**, forwarding P3's already-decided labels as `--labels`. If a PR already exists for this branch, that skill's `gh pr edit` path updates it in place instead of opening a new one.
3. **Check the tag, now that the push has landed.** Run `changelog`'s own "Tag sync" check against the section this band just released: `git tag -l` (or read `.git/refs/tags/` / `.git/packed-refs`) for whether it already has a matching tag. If not, include the exact `git tag -a vX.Y.Z <hash> -m "<summary>"` command in this phase's output — tag creation itself stays the user's, unchanged.

## Output

````markdown
## ✅ <task or phase name> — published

<the git-pr-create output — the PR URL>

<the git tag command, only if the released section has no matching tag yet>
````

- **Nothing else.** No commit message here, no changed/verified/follow-ups breakdown — that already happened in P3's report. This phase reports two things: is it published and where, and whether the release still needs a tag.
