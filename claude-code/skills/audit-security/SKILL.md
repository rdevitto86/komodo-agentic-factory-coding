---
name: audit-security
description: Read the diff against the OWASP baseline for injected security defects — new boundary, new query, new secret handling — and file them as BACKLOG.md stories. Model-agnostic finder; never a fixer. Pass --report to skip the write.
argument-hint: <task text or band summary> [standards-* skills that apply] [--report]
---

# Security audit

Reviewing: **$ARGUMENTS**

Model-agnostic finder — Read/Grep/Glob/Bash only, no host-specific tooling. Load `standards-security` first — it states the OWASP-benchmarked bar this reviews against. Never a fork of the session that wrote the code. Findings only, never fixes.

## Process

1. **Read `git diff` for the band.**
2. **Walk every touched external boundary** — an endpoint, a query, a file path, a shell command, a deserialization, a secret or credential, an auth check. A file with none of those is out of scope.
3. **Test each boundary against `standards-security`'s checklist**: injection (SQL/command/template), auth bypass, secret exposure (logged, committed, hardcoded), missing input validation, broken access control, insecure deserialization.
4. **A finding needs a concrete exploit path**, not a hypothetical. "Could theoretically" is not a finding; "input X reaches Y unvalidated, sink Z executes it" is.

## Report

| Sev | Where | Claim | Exploit path | OWASP |
|---|---|---|---|---|
| C | `handler.go:88` | unvalidated redirect target | attacker-supplied `next` reaches `http.Redirect` unchecked | A01 |

**Sev**: Critical (auth bypass, injection, secret exposure) · High (exploitable but needs a precondition) · Medium (defense-in-depth gap, no direct exploit path yet) · Low (hardening, not exploitable as written).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <claim> · S → \`/audit-security <file>\` reports it clear`. Append under the current target state (the first `##` heading) and the domain matching the file's area, or `Cross-Cutting` if none fits — full story-line rules live in `generate-backlog`. `--report` prints the table only; nothing is written.
