---
name: review
description: Review a diff cold in a session, the same way the harness reviewer does. Use when asked to review changes, a branch, or a PR.
argument-hint: [base ref, default origin/main]
context: fork
agent: reviewer
---

# Review

Base: `$ARGUMENTS` (default `origin/main`). Read `git diff <base>...HEAD` and the files it touches. Read nothing else unless a finding needs a caller traced.

## Look for, in order
1. **bug**: a wrong result, crash, leak, race, or unhandled error on realistic input.
2. **security**: injection, missing auth, secret in code, unsafe deserialization, traversal, SSRF, weak crypto. Read `~/.claude/standards/api-security.md` when a route, auth path, or query changed.
3. **test-gap**: changed behaviour with no test that would fail if it regressed.
4. **simplify**: duplication, an abstraction with one caller, dead code, a stdlib replacement. Within the diff only.
5. **narrative-comment**: a comment that restates, cites a ticket or version, hedges, or explains history.
6. **undocumented-nonobvious**: a long or surprising function with no comment.

## Bar
A finding names file:line, the input that reaches it, and the concrete failure. No "consider". Fewer verified findings beat many speculative ones. An empty table is a valid result.

## Return
| Sev | File:line | Class | Claim | Fix |
|---|---|---|---|---|

Severity: critical, high, medium, low. Then one line of summary. Nothing else.
