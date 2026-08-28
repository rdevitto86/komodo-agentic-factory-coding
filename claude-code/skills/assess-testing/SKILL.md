---
name: assess-testing
description: Assess the test suite against standards-sdlc's tiers, layout, and coverage floors — misplaced tests, tier-dependency violations, floor breaches, coverage padding — filed as BACKLOG.md stories. Pass --report to skip the write.
argument-hint: [optional: path or diff scope] [--report]
disable-model-invocation: true
---

# Testing assessment

Scoping: **$ARGUMENTS** (default: the repo's full test suite)

Findings only, never writes a test — a coverage or placement gap routes back to implementation. Load `standards-sdlc` and every language skill the repo's manifests indicate first — detect each one present (`go.mod`, `package.json`, `pyproject.toml`, `pom.xml`/`build.gradle`, `*.csproj`, and so on), not just the first found, so a polyglot repo gets assessed in one pass across every language it actually contains; every check below tests against tiers, layout, and coverage rules they own, not rules restated here. If an SDD exists (`docs/spec/SDD.md`, see `standards-specs`), its §6 Testing Strategy table is a citation of the same rules, not a second source — flag it as drift if the numbers disagree with `standards-sdlc`.

## Process

1. **Locate the suite** — unit tests colocated beside their source; every other tier under the language's declared test root, one subfolder per tier. A unit test filed away from its source, or a tier test outside the root, is a placement finding.
2. **Check tier dependency** — a unit test that needs network, disk, a clock, or a container is misplaced; no lower tier may depend on a higher one.
3. **Run the repo's actual coverage tool** (`.claude/verify.sh` or the Makefile/Taskfile/justfile test target) — never claim a coverage number without executing it, same re-verified-by-execution rule `assess-readiness` holds for build/test status.
4. **Check the floor** — new/changed code in scope below 85%; any SDK, shared library, or security-critical package (auth, payments, PII) below 100%.
5. **Check for padding** — a case that asserts only "no error" or "not nil" with no assertion on the actual result inflates the number without covering the behavior; flag it as a finding, not a pass.
6. **Check "skipped never reports as passed"** — a test suite that swallows a skip-on-outage and reports green is itself a finding.
7. **Check helper placement** — a helper used by one file sitting in a shared test-utility package, or a per-case hook doing heavy setup that could run once at file/suite scope, is drift from `standards-sdlc`'s placement and hook rules.

## Report

| Sev | Where | Finding | Evidence |
|---|---|---|---|
| H | `pkg/auth` | 62% coverage, security-critical | coverage run output |
| M | `internal/foo/foo_test.go` | asserts `err == nil` only, 4 branches unchecked | `file:line` |

**Sev**: Critical (a security-critical or SDK package below its 100% floor, or a suite reporting a skipped test as passed) · High (new/changed code below the 85% floor, a lower tier depending on a higher one) · Medium (misplaced test file, a stale §6 citation against `standards-sdlc`, a heavy per-case hook) · Low (coverage padding, a shared helper that should be file-local).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report.**

## Findings → backlog

Each row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <finding> · S → \`/assess-testing <scope>\` reports it clear`. Append under the current target state (the first `##` heading) and the domain matching the file's area, or `Cross-Cutting` if none fits — full story-line rules live in `backlog`. `--report` prints the table only; nothing is written.
