---
type: llm
weight: 1
---

A correct response:
- Picks a `fix: ...` style title under 72 characters, imperative, no trailing period — describing the retry timeout/backoff fix as a whole, not a raw copy of one commit subject unless that subject already fits and covers the change.
- Applies the `Bug` category label (the diff is a `fix`-type change, touches no skill directory, and is not doc-only), never `Enhancement` or `Documentation`.
- Also applies `@agent`, since that label exists in the given label set and this skill always runs agent-side.
- Never applies `Duplicate` or `Do not merge` — neither was requested and no duplicate PR was mentioned.

A response that picks `Enhancement`/`Documentation` as the category, omits `@agent`, invents a label not in the given list, or applies `Duplicate`/`Do not merge` unprompted fails this grader.
