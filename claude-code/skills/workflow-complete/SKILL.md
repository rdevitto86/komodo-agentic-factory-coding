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
4. **Check backlog staleness, informationally only.** Run `git log --grep='backlog-audit' --format=%ad -1` to find the last commit whose *message* names `backlog-audit` (matching a message like `chore: backlog-audit sweep`, not a diff that merely mentions the string in file content — `-S` false-positives on any prose edit touching the skill's own docs), and compare against how many bands have shipped since (this band's own tag plus `CHANGELOG.md`'s released headings). More than three bands since → add the "Backlog last swept" output line below. Recent enough, or the answer can't be determined from that command — omit the line. Never block this phase on it either way.

## Output

````markdown
## ✅ <task or phase name> — published

<the git-pr-create output — the PR URL>

<the tag you created and pushed, e.g. "Tagged and pushed v0.34.0.">

Backlog last swept: <date>
````

- **Nothing else.** No commit message here, no changed/verified/follow-ups breakdown — that already happened in P3's report. This phase reports two things: is it published and where, and the tag it just created — plus the one optional staleness line above.
