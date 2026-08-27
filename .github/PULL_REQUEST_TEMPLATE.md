<!--
Title: <type>: <summary>  — max 72 chars, imperative, no trailing period.
Types: feat fix chore docs test refactor perf build ci
Risk tier is a label, not a section. Run /audit-change-risk --report and
post it as a comment.
-->

## Summary

<!-- One or two sentences: what this PR is for. Never a file list. -->

## Changes

<!-- What changed, not how you got there. One bullet per area, not per file —
     ten files in one package is one bullet. -->

- **<area>** — <what changed>

## Validation Evidence

<!-- Only what a green CI run cannot show: a live-dependency happy path, a
     cURL against STG, a behaviour with no automated coverage yet. Unit,
     component, and contract results are Stage 2's job — do not paste them.
     "Covered by CI" is a complete answer. -->

## Dependencies

<!-- Numbered, in the order they must land or land first. Internal: another
     PR in this repo or a sibling repo. External: a package, service, or API
     version this PR requires. Omit the section entirely if there are none —
     never write "none" as a list item. -->

1. <dep>
