---
name: assess-vulnerabilities
description: Triage known-CVE dependency exposure — Dependabot alerts first, a language-native scanner otherwise — and file them as BACKLOG.md stories. Model-agnostic finder; never a fixer. Pass --report to skip the write.
argument-hint: <task text or band summary> [standards-* skills that apply] [--report]
---

# Vulnerability assessment

Reviewing: **$ARGUMENTS**

Model-agnostic finder — Read/Grep/Glob/Bash only, no host-specific tooling. Scoped to known-CVE dependency exposure, never a fixer. Complements `assess-security`: that skill reviews the diff for *injected* defects (new boundary, new query, new secret handling); this one never re-reads the diff for code-level bugs, only checks what a dependency already carries a published CVE for.

## Process

1. **Triage Dependabot first.** Try `gh api /repos/{owner}/{repo}/dependabot/alerts --jq '.[] | select(.state=="open")'`; if that 404s or errors (Dependabot not enabled, insufficient scope), fall back to `gh issue list --label dependabot --state open`. Either result standing in for step 2 — skip straight to Report.
2. **Scan the manifest/lockfile when Dependabot yields nothing usable.** Load whichever `standards-<language>` skill(s) this repo's languages trigger and run the exact tool it documents — never invent one: `govulncheck ./...` (Go), `npm audit --omit=dev --audit-level=high` (or the pnpm/yarn equivalent, TS/JS), `pip-audit` (Python). No matching language skill: fall back to `osv-scanner` against the lockfile if it's installed; otherwise report that no scanner was available and stop.
3. **Read each hit's severity and reachability**, not just its CVE score — `govulncheck` and `npm audit` both report whether the vulnerable symbol is actually called; an unreachable high-severity CVE is still worth recording but ranks lower than a reachable one.
4. **A finding needs the CVE/GHSA ID and the affected package@version**, not a vague "dependency X is old" — that belongs to `assess-dependencies`, not this skill.
5. **A finding needs a concrete reachable path**, not a hypothetical. "Could theoretically" is not a finding; "the scanner marks the vulnerable symbol reachable from the entry point" is.

## Report

| Sev | Where | Claim | Scenario |
|---|---|---|---|
| H | `go.mod: golang.org/x/net@0.17.0` | CVE-2023-45288 (HTTP/2 CONTINUATION flood) | `govulncheck` marks the vulnerable func reachable from `cmd/server` |

**Diagram form**: a Mermaid `graph LR` of each reachability chain the scanner reported — entry package → intermediate → the vulnerable symbol. Draw it when chains share a hop or the table runs past 5 rows; below both, the table alone is complete.

**Sev**: Critical (reachable, exploitable, no fix pending) · High (reachable, patch available) · Medium (unreachable per the scanner, or needs an unlikely precondition) · Low (dev-only dependency, or already flagged with a tracked upstream fix).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <claim> · S → \`/assess-vulnerabilities <file>\` reports it clear`. Append under the current target state (the first `##` heading) and the domain matching the file's area, or `Cross-Cutting` if none fits — full story-line rules live in `backlog-modify`. `--report` prints the table only; nothing is written.
