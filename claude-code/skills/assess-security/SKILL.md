---
name: assess-security
description: Read the diff against the OWASP baseline for injected security defects — new boundary, new query, new secret handling — and return them as a findings table. Model-agnostic finder; never a fixer.
argument-hint: <task text or band summary> [standards-* skills that apply]
context: fork
agent: reviewer
background: false
---

# Security assessment

Reviewing: **$ARGUMENTS**

Model-agnostic finder — Read/Grep/Glob/Bash only, no host-specific tooling. Load `standards-api-security` first — it states the OWASP-benchmarked bar this reviews against for any server-side boundary. Add `standards-ui-security` when the diff touches a rendered surface (`.svelte`/`.vue`/`.tsx`/`.jsx`/`.html`) — it owns XSS, clickjacking, and dark-pattern findings. Load whichever `standards-<language>` skill(s) the touched files trigger — `standards-go`, `standards-python`, `standards-typescript`, `standards-java`, `standards-c`, `standards-dotnet`, `standards-shell`, and so on — and pull from each one's own Security standards section, where it carries one, alongside the checklist below: a language-specific insecure-usage pattern (Go's `text/template` vs `html/template`, Python's `pickle`/`yaml.load`, a Java deserialization boundary) is this same pass's job, not a second review. Findings only, never fixes.

## Process

1. **Read `git diff` for the band.**
2. **Walk every touched external boundary** — an endpoint, a query, a file path, a shell command, a deserialization, a secret or credential, an auth check, a rendered template. A file with none of those is out of scope.
3. **Test each boundary against the loaded skill(s)' checklist**: injection (SQL/command/template), auth bypass, secret exposure (logged, committed, hardcoded), missing input validation, broken access control, insecure deserialization, and — for a rendered surface — XSS, clickjacking, and dark patterns.
4. **A finding needs a concrete exploit path**, not a hypothetical. "Could theoretically" is not a finding; "input X reaches Y unvalidated, sink Z executes it" is.

## Report

| Sev | Where | Claim | Exploit path | OWASP |
|---|---|---|---|---|
| C | `handler.go:88` | unvalidated redirect target | attacker-supplied `next` reaches `http.Redirect` unchecked | A01 |

**Sev**: Critical (auth bypass, injection, secret exposure) · High (exploitable but needs a precondition) · Medium (defense-in-depth gap, no direct exploit path yet) · Low (hardening, not exploitable as written).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

The caller files these rows to `BACKLOG.md`; you never write.
