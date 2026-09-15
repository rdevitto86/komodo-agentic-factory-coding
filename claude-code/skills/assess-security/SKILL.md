---
name: assess-security
description: Read the diff against the OWASP baseline for injected security defects — new boundary, new query, new secret handling — and return them as a findings table. Model-agnostic finder; never a fixer.
argument-hint: a reviewer brief — Task (band summary) | Files | Context | Round | Standards | Out of scope (all six required)
context: fork
agent: reviewer
background: false
---

# Security assessment

Brief: **$ARGUMENTS** — a `reviewer` brief carrying `Task`, `Files`, `Context`, `Round`, `Standards`, and `Out of scope`. A missing or empty one of those is your standing stop, not something to infer from the diff.

Model-agnostic finder — Read/Grep/Glob/Bash only, no host-specific tooling. The `Standards` slot names which of the skills below the touched files load. Load `standards-api-security` first — it states the OWASP-benchmarked bar this reviews against for any server-side boundary. Add the UI skill for the rendered surface the diff touches and read its Security section — `standards-ui-web` for a browser surface (`.svelte`/`.vue`/`.tsx`/`.jsx`/`.html`/`.css`), `standards-ui-mobile` for a native mobile one, `standards-ui-desktop` for a desktop shell. Load whichever `standards-<language>` skill(s) the touched files trigger — `standards-go`, `standards-python`, `standards-typescript`, `standards-java`, `standards-c`, `standards-dotnet`, `standards-shell`, and so on — and pull from each one's own Security standards section, where it carries one, alongside the checklist below: a language-specific insecure-usage pattern (Go's `text/template` vs `html/template`, Python's `pickle`/`yaml.load`, a Java deserialization boundary) is this same pass's job, not a second review. Findings only, never fixes.

## Process

1. **Read `git diff` for the band.**
2. **Walk every touched external boundary** — an endpoint, a query, a file path, a shell command, a deserialization, a secret or credential, an auth check, a rendered template. A file with none of those is out of scope.
3. **Test each boundary against the loaded skill(s)' checklist**: injection (SQL/command/template), auth bypass, secret exposure (logged, committed, hardcoded), missing input validation, broken access control, insecure deserialization, and — for a rendered surface — XSS, clickjacking, and dark patterns.
4. **A finding needs a concrete exploit path**, not a hypothetical. "Could theoretically" is not a finding; "input X reaches Y unvalidated, sink Z executes it" is.

## Report

`reviewer.md`'s section structure governs — a near-miss goes to `## Considered and dismissed`, not here. This table's columns and `Sev` scale are this lens's own.

| Sev | Where | Claim | Exploit path | OWASP |
|---|---|---|---|---|
| C | `handler.go:88` | unvalidated redirect target | attacker-supplied `next` reaches `http.Redirect` unchecked | A01 |

**Diagram form**: a Mermaid `graph LR` — one path per finding, attacker-controlled entry point → each hop → the sink it reaches. `reviewer.md` says when it is drawn and where it sits.

**Sev**: Critical (auth bypass, injection, secret exposure) · High (exploitable but needs a precondition) · Medium (defense-in-depth gap, no direct exploit path yet) · Low (hardening, not exploitable as written).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

The caller files these rows to `BACKLOG.md`; you never write.
