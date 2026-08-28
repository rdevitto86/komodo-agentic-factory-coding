---
name: assess-dependencies
description: Sweep the dependency manifest for staleness — outdated pins, deprecated APIs, EOL runtimes — and file them as BACKLOG.md stories. Model-agnostic finder; never a fixer. Pass --report to skip the write.
argument-hint: <task text or band summary> [standards-* skills that apply] [--report]
---

# Dependency assessment

Reviewing: **$ARGUMENTS**

Model-agnostic finder — Read/Grep/Glob/Bash only, no host-specific tooling. Scoped to staleness, deprecation, and EOL, never a fixer. Complements `assess-vulnerabilities`: that skill covers known-CVE dependency exposure; this one never re-checks for a published vulnerability, only whether a pin, an API, or a runtime has gone stale.

## Process

1. **Load whichever `standards-<language>` skill(s) this repo's languages trigger** and read its Toolchain section for the manifest location and the version-floor rule (`go.mod`, `package.json`, `pyproject.toml`) — never guess the manifest path.
2. **Run each language's native "list outdated" subcommand of the toolchain that skill already names** — `go list -u -m all` (Go), `npm outdated` (or the pnpm/yarn equivalent, TS/JS), `pip list --outdated` (or `uv pip list --outdated`, Python). No matching language skill: report that no outdated-listing tool applies and move to the next check.
3. **Grep for the language's own deprecation marker** on symbols this repo actually calls — Go's `// Deprecated:` doc convention, TS/JS's `@deprecated` JSDoc tag, Python's `DeprecationWarning`/`@deprecated` decorator. A marker on a symbol this repo never calls is not a finding.
4. **Read the version floor each manifest declares** (`go.mod`'s directive, `package.json`'s `engines`, `pyproject.toml`'s `requires-python`) and flag one that the language's own current release cycle has already dropped support for — cite the manifest line, never a CVE ID or advisory, that belongs to `assess-vulnerabilities`.

## Report

| Sev | Where | Claim | Scenario |
|---|---|---|---|
| M | `go.mod: golang.org/x/net@0.17.0` | 4 minor releases behind per `go list -u -m all` | no reachable exploit, but the pin blocks picking up unrelated fixes |

**Sev**: Critical (runtime past EOL, no security patches ship for it at all) · High (major version behind with a breaking-change migration overdue) · Medium (minor/patch versions behind, or a deprecated API this repo still calls) · Low (deprecated API with a stable no-op stub, or a dev-only dependency).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <claim> · S → \`/assess-dependencies <file>\` reports it clear`. Append under the current target state (the first `##` heading) and the domain matching the file's area, or `Cross-Cutting` if none fits — full story-line rules live in `backlog`. `--report` prints the table only; nothing is written.
