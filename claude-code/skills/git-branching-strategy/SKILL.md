---
name: git-branching-strategy
description: Whether a change belongs on one short-lived branch off main or needs a longer-lived feature branch composed of several stacked PRs. Load before cutting a branch for anything that looks larger than a single PR.
user-invocable: false
---

# Branching Strategy

`git-pr-create` already owns branch naming, push mechanics, merge direction, and per-PR sizing — load it for how to name, push, merge, or size any one branch. This skill governs the layer above that: which shape of branch structure a piece of work should use in the first place, decided once, before the first branch is cut.

## Default: one small PR off the current base

Most changes should land as a single short-lived branch cut from `main` (or whatever the current base is), reviewed, merged, deleted. This toolkit's own `workflow-loop` follows this by construction — each band gets its own branch, one band is one PR — because a band is already scoped to land and review independently. Treat that as the default shape for any change that isn't explicitly ruled out below, not as a special case that only applies to this repo's own workflow.

## When a longer-lived feature branch is warranted

Reach for a feature branch composed of multiple stacked/sequential PRs only when both are true:

- The change is too large or too dense to land as one reviewable PR (see `git-pr-create`'s sizing table for what "too large" means in practice), **and**
- Its pieces have a genuine sequential dependency — a later piece cannot be written, reviewed, or merged sensibly without an earlier piece already in place (a schema migration before the code that reads it, a new interface before the callers that adopt it).

If the pieces don't actually depend on each other in that order, they are independent work, not a stack — cut separate branches off the same base and open separate PRs instead. Stacking work that isn't actually sequential just adds coordination overhead (rebase-tracking, merge-order babysitting) for no benefit.

## The tradeoff

- **A long-lived branch accumulates risk the longer it lives** — every commit that lands on the base after it was cut is a commit it doesn't have, so its merge-conflict odds and its staleness both grow with time. Mitigate by merging the base into it regularly, never the reverse — per `git-pr-create`'s merge convention (base merges into your branch, never your branch into the base; landing into the base itself is always a human action through the PR merge button).
- **Smaller PRs get faster review and faster CI signal per change**, and each one is independently revertable if it turns out wrong. The cost is discipline: when a real dependency exists between PRs, they must be sequenced and reviewed in the right order, and a downstream PR's diff will include its unmerged upstream dependency until that lands first.

## Telling entangled from independent

- **One entangled change** — a refactor that touches every caller of a renamed function, a type change that ripples through the same set of files, anything where reviewing one piece without the others leaves the tree in a state that doesn't build or doesn't make sense on its own. Ship this as one PR, even an oversized one, rather than cutting it apart artificially — `git-pr-create`'s sizing section covers this call directly; don't re-derive it here.
- **Independent work that merely landed in the same task list** — two features touching disjoint files, two fixes with no shared symbol or call path. These should be separate branches off the same base and separate PRs, reviewed and merged on their own schedules — never stacked, and never forced into one branch just because they were planned together.

## Never a substitute for the PR flow

Whichever shape a change takes — one small PR, or a feature branch's worth of stacked ones — every branch still goes through its own PR and its own review. Neither branch strategy is ever a reason to commit directly to a protected branch or skip opening a PR; `git-pr-create`'s protected-ref rules apply identically regardless of which strategy produced the branch.
