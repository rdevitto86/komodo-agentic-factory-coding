# Pull Request Standards

Universal PR standards for all Komodo projects. See `/git-flow` for branch naming and commit conventions.

---

## 1. PR size

- Target <400 lines changed per PR. Larger PRs must include a written justification in the description.
- Split by concern: refactors are separate PRs from features; dependency upgrades are separate from logic changes.
- A PR that does two things should be two PRs. If that's impractical, explain why in the description.

---

## 2. Description requirements

Every PR description must include all four sections:

```
## What
[Summary of what changed — 2–4 sentences]

## Why
[Motivation or link to Trello card / issue]

## How to test
[Steps or commands to verify the change works]

## Rollback
[How to undo this change if it breaks in production]
```

PRs missing any section may be returned without review.

---

## 3. Reviewer responsibilities

- Reviewers must actually read and understand the change — approval is a statement that you've done so
- Leave specific, actionable comments — "this looks wrong" without explanation is not useful feedback
- Mark comments as one of: **blocking** (must fix before merge) or **non-blocking** (address in follow-up)
- If you cannot review within 1 business day, say so — don't let PRs sit silently
- "LGTM" without reading is not a review

---

## 4. Author responsibilities

- Self-review your own diff before requesting review — catch your own debug code and commented-out blocks first
- No WIP PRs in the review queue — use draft PRs for work-in-progress
- Respond to review comments within 1 business day
- When resolving a comment, either fix it or leave a reply explaining why you didn't — don't silently dismiss

---

## 5. Merge criteria

All of the following must be true before merging:

- [ ] All CI checks pass (tests, lint, build, security scan)
- [ ] At least 1 approval from a reviewer who read the change
- [ ] 2 approvals required for changes touching: auth, payments, data schema migrations, security-relevant middleware
- [ ] No unresolved blocking comments
- [ ] PR description is complete
- [ ] Linked Trello card is updated to reflect the PR is in review

---

## 6. Post-merge

- Delete the source branch immediately after merge
- Verify the deployment succeeded if the branch triggered a deploy
- Close the linked Trello card or move it to the appropriate status
- If the change requires a follow-up, create the Trello card before closing this one
