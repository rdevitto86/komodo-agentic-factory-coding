---
name: workflow-complete
description: Publish the finished, committed band — push its branch and open or update the PR via generate-pr.
argument-hint: []
---

# Workflow Complete — Publish

**On a non-protected branch, with the band already committed** — if either isn't true, say so and stop. Committing is P2.3/P3's job (one commit per task, one for `workflow-consolidate`'s own delta); this phase never generates a commit message and never runs `git commit` itself.

## Order

1. **Push** — `git push -u origin <branch>` (`git rev-parse --abbrev-ref HEAD` for the branch name).
2. **Run `/generate-pr`.** No PR exists yet for this branch → opens one. A PR already exists (loop resumed, or this is an update) → `generate-pr`'s `gh pr edit` path updates it in place.

## Output

````markdown
## ✅ <band name> — published

<the generate-pr output — the PR URL>
````

Nothing else here — the per-task and per-band summaries already happened in P2.3/P3. This phase reports one thing: is it published, and where.
