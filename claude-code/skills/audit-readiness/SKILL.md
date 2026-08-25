---
name: audit-readiness
description: Full-repo readiness audit against a mission brief. Severity-ranked, evidence-backed findings and a single verdict, filed as BACKLOG.md stories. Pass --report to skip the write.
argument-hint: <mission brief — purpose, target state, decision to render> [--report]
disable-model-invocation: true
---

# Readiness audit

Mission brief: **$ARGUMENTS**

The brief states the app's purpose, its target state, and the decision to render. If it names no decision, default to readiness for the stated target state.

## Ground rules

- **Re-derive everything from the code as it is now.** No prior run, score, or ledger carries forward.
- **Review the code as-is.** Do not assume it needs changing.
- **Run the repo's own gate first** — build, vet, test, lint. A red gate is itself evidence, and **any claim about build or test status is re-verified by execution, never by reading.**
- **Load `BACKLOG.md` first.** A finding matching an open story is tagged `[tracked]` and keeps its tier — never filed twice. Tracking never clears the bar — a tracked Blocker still blocks.
- **Every finding carries `file:line` evidence.** No pointer, no finding.
- **Report only findings you hold at medium confidence or higher.** A Blocker needs high confidence; if evidence is incomplete, state what would confirm it and keep it out of the verdict.
- **Prerequisites outside this repo never move the verdict.** List them once, separately.

## The bar — fixed across runs

A **Blocker** alone forces a "no". Exactly four kinds qualify:

1. **Security defect exploitable in the target environment** — auth bypass, privilege escalation, injection, secret exposure, token forgery.
2. **Correctness bug on a critical path** — wrong result, data loss or corruption, or a double effect on retry, in a flow the target state depends on.
3. **Deploy-stopper this repo controls** — will not build, will not deploy, or fails closed because of in-repo code or config.
4. **Blind spot** — a core failure mode with no alarm or signal defined in this repo.

Everything else — performance tuning, maintainability, doc drift, missing-but-nice features — **is non-blocking by definition and must not move the verdict.** Hold this bar identical across runs. Never redefine "ready".

## Lenses

Apply each only to surfaces that actually exist here.

- **Correctness** — logic errors, races, unhandled edges, error propagation, resource leaks.
- **Security** — authn/authz, input validation, secrets, injection, SSRF, dependency risk, data exposure.
- **Resilience** — idempotency, retries and backoff, timeouts, circuit breaking, rate limiting, dead-letter handling, degradation.
- **Performance** — hot-path latency and allocation, N+1 access, caching posture, pagination, payload size. Rate impact L/M/H — roughly 100ms+ is H, 25ms is L.
- **Observability** — log, metric, and alarm coverage: throughput, latency, 4xx/5xx, health checks, resource limits, bootstrap failures.
- **Infra cohesion** — build and container config alignment, IaC posture, environment parity, a working local setup.
- **Data** — schema and migration safety, numeric width and precision for money and time, consistency boundaries, retention.
- **Maintainability** — structure, duplication, dead code, drift between docs and code, test coverage of critical paths.
- **UI** — state correctness, error/loading/empty states, validation parity, accessibility basics, drift from the API contract.

Missing features and improvement ideas are welcome as low-severity findings unless they hide a real defect.

## Output

```markdown
## 🔍 Verdict
**<GO / NO-GO>** — <the single deciding reason>

## 🚫 Blockers
- **<what>** — `file:line` — meets bar clause <n>

## 📋 Findings
| Sev | Conf | What | Where | Why it matters |
|---|---|---|---|---|

## 🌐 External prerequisites
- <outside this repo, not counted in the verdict>
```

Cap the findings table at 15 rows. Past that, report the top 15 by severity and state how many were omitted — an unbounded finding dump is noise, not an audit.

## Findings → backlog

Every Blocker and every `📋 Findings` row becomes one `BACKLOG.md` story unless `--report` is in `$ARGUMENTS`: `- [Sev] <what> · S → \`file:line\` no longer holds`. A row already `[tracked]` against an open story is skipped, not duplicated. Append under the current target state (the first `##` heading) and the domain matching the finding's area, or `Cross-Cutting` if none fits — full story-line rules live in `generate-backlog`. `--report` prints the verdict and tables only; nothing is written.
