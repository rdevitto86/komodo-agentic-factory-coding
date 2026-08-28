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
3. **Tag the release, now that the push has landed.** Invoke `git-commit-tag` by name against the section this band just released — the commit it needs is `workflow-consolidate`'s own commit from P3, the one that introduced that band's `## [X.Y.Z]` heading.

## Output

````markdown
## ✅ <task or phase name> — published

<the git-pr-create output — the PR URL>

<the tag you created and pushed, e.g. "Tagged and pushed v0.34.0.">
````

- **Nothing else.** No commit message here, no changed/verified/follow-ups breakdown — that already happened in P3's report. This phase reports two things: is it published and where, and the tag it just created.
