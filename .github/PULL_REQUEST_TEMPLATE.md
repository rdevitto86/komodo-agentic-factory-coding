<!--
Title: <type>: <summary>  — max 72 chars, imperative, no trailing period.
Types: feat fix chore docs test refactor perf build ci
Risk tier is a label, not a section. Run /audit-change-risk --report and
post it as a comment.
-->

**Depends on:** <!-- PR # that must merge first, or "none" -->

## Summary

<!-- One or two sentences: what this PR is for. Never a file list. -->

## Changes

<!-- What changed, not how you got there. One bullet per area, not per file —
     ten files in one package is one bullet. Append the PRD requirement ID in
     parentheses where the story carried one. -->

- **<area>** — <what changed> (<req-id>)

## Validation Evidence

<!-- Only what a green CI run cannot show: a live-dependency happy path, a
     cURL against STG, a behaviour with no automated coverage yet. Unit,
     component, and contract results are Stage 2's job — do not paste them.
     "Covered by CI" is a complete answer. -->

---

- [ ] Every file in the diff belongs to a bullet under **Changes** — nothing rode along
- [ ] Scope matches the `BACKLOG.md` stories this PR closes
- [ ] No suppressed check, no `--no-verify`, no widened type to pass a gate
