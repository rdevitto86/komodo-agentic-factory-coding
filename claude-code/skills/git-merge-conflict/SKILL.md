---
name: git-merge-conflict
description: How to resolve a merge conflict once git surfaces one — what to resolve unattended, what to escalate, and what never gets silently dropped. Load before resolving any conflict marker.
user-invocable: false
---

# Merge Conflict Rules

`rules-source-control` already covers *when* a merge is allowed and *how* to close it out (`git add` each resolved file, `git commit --no-edit`). This skill governs the resolution itself, once `<<<<<<<`/`=======`/`>>>>>>>` markers land in a file.

## Never silently drop a side

Resolving a conflict is reconciling two changes, not choosing a winner. Never delete a hunk from either side just to make the file parse or the build pass without first understanding why both sides touched the same lines. If the two sides are genuinely incompatible — not complementary — that is a decision only the user can make; say so and stop rather than guessing which one "wins."

A deleted-vs-modified conflict (one branch removed a file, the other kept editing it) is the sharpest form of this: never resolve it by silently accepting the deletion. Surface it and ask, unless the file's removal is independently confirmed elsewhere (e.g. `CHANGELOG.md` records the file's replacement).

## Resolve without asking

- **Mechanical conflicts** — both sides touch the same region but the changes are disjoint and additive (two new imports, two new struct fields, two new test cases appended near each other). Merge both, in the order that keeps the file's existing ordering convention.
- **Formatting-only conflicts** — whitespace, line-wrap, or import-sort differences with no semantic change on either side. Take the version that matches the file's existing style; run the formatter after resolving, per `rules-source-control` and the language skill's toolchain section.
- **Identical intent expressed differently** — both sides renamed the same symbol to the same name, or both independently fixed the same bug the same way. Collapse to one copy; do not keep a duplicate.
- **Generated or lockfiles** — never hand-merge conflict markers inside a lockfile or other generated artifact. Resolve by deleting the file's markers and regenerating it with the tool that produced it (per the language skill's toolchain section), then re-add.

## Escalate instead of guessing

- **Semantic divergence** — both sides changed the same business logic in different, non-additive ways (different validation rule, different return value, different error handling). Ask which behavior is correct; do not average or pick arbitrarily.
- **Security-sensitive code** — auth, permission checks, secrets handling, input validation. A wrong guess here is not reversible by re-reading the diff later; escalate every time, even a conflict that looks mechanical at a glance.
- **Test files where both sides changed the same assertion** to different expected values — this usually means the underlying behavior itself diverged; resolving the test alone without resolving that first hides the real conflict.
- **Anything where the merged result cannot be inferred from the two hunks and their surrounding context alone.** Guessing and moving on is how a silent regression ships; asking costs one turn.

## Closing out

No conflict marker (`<<<<<<<`, `=======`, `>>>>>>>`) may reach a commit — grep the resolved files before `git add` if there is any doubt. `comment_guard.py` still applies to whatever the resolution writes: a leftover "// keep ours" or "// merged" note is a banned implementation narrative, not an exempt directive — see `rules-commenting`.
