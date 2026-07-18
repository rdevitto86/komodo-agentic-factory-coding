---
name: audit
description: Full-repo readiness audit. Severity-ranked, confidence-tagged findings against a per-run mission brief. Argument = mission brief (app purpose, target state, decision to render).
argument-hint: <mission brief — purpose, target state, decision to render>
---

# Codebase Audit

Mission brief: $ARGUMENTS

The brief states the app's purpose, its target state, and the decision to render (e.g. "GO/NO-GO for V1 prod", "top priorities for next sprint", "direction check on the current design"). If the brief names no decision, default to GO/NO-GO readiness for the stated target state.

## Ground rules

- Re-derive everything from the code as it is now. No prior run, score, or ledger carries forward.
- Review the code as-is. Do not assume it needs changes.
- Run the repo's own stated gate (build, vet, test, lint) before static review. A red gate is itself evidence, and any claim about build/test status — including from prior audit rounds — is re-verified by execution, not by reading.
- Before auditing, load `TODO.md` at the repo root and in relevant subdirectories — it is the planned-work docket. A finding that matches a docket item is tagged `[tracked]` and keeps its tier; it is known work, not a new discovery. Tracking never clears the bar: a tracked Blocker still blocks.
- Also load ADRs and decision logs. Designs recorded there as accepted or rejected are settled — do not resurface them as findings.
- Every finding carries `file:line` evidence. No pointer, no finding.
- Report only findings you hold at Medium confidence or higher. A Blocker requires High confidence; if evidence is incomplete, state exactly what would confirm it and keep it out of the verdict.
- Prerequisites outside this repo's control (DNS, certs, account provisioning, deploy roles, sibling services) never move the verdict. List them once under **External prerequisites**.

## The bar (fixed across runs)

A **Blocker** is a finding that alone forces a "no" on the brief's decision. Exactly four kinds qualify:

1. **Security defect exploitable in the target environment** — auth bypass, privilege escalation, injection, secret exposure, token forgery, tenant/plane crossing.
2. **Correctness bug on a critical path** — wrong result, data loss or corruption, or double-effect on retry, in a flow the target state depends on.
3. **Deploy-stopper this repo controls** — won't build, won't deploy, or fails closed in the target environment due to in-repo code or config.
4. **Blind spot** — a core failure mode (error spikes, dependency down, resource exhaustion, abuse) with no alarm or signal defined in this repo.

Everything else — performance tuning, maintainability, doc drift, missing-but-nice features, accepted tradeoffs — is non-blocking by definition and must not move the verdict. Hold this bar identical across runs; never redefine "ready".

## Lenses

Apply each lens only to surfaces that exist in this repo, using checks appropriate to the languages found (e.g. integer width/signedness and overflow where the language exposes them; allocation and lifetime where manual; async/await misuse where relevant).

- **Correctness & bugs** — logic errors, race conditions, unhandled edge cases, error-handling depth and propagation, resource leaks.
- **Security** — authn/authz, input validation, secrets handling, injection classes, SSRF, dependency risk, data exposure, abuse resistance.
- **Resilience** — idempotency of mutating operations, retries/backoff, timeouts, circuit breaking, rate limiting, queue + dead-letter handling (incl. stale/flush paths), graceful degradation.
- **Performance** — hot-path latency and allocation, N+1 access, caching posture (warming, invalidation, stampede), pagination, payload size. Rate each item L/M/H impact (~100ms+ = H, ~25ms = L).
- **Observability** — info/error log coverage, metrics, and alarm coverage: throughput, latency, 4xx/5xx, health checks, resource limits, failover, init/bootstrap failures, upstream/downstream dependencies.
- **Infra cohesion** — build/container/task-runner config alignment across the repo, IaC posture (load balancing, autoscaling, resource limits, failover), env parity, working local/dev setup.
- **Data** — schema and migration safety, numeric width/signedness/precision (money, time), consistency boundaries, retention.
- **Maintainability** — structure, duplication, dead code, spec/PRD/HLD drift vs code, test coverage of critical paths.
- **Compliance & privacy** — PII handling, audit trails, license red flags, regulatory exposure where applicable.
- **UI (when present)** — state-management correctness, error/loading/empty states, client+server validation parity, accessibility basics, bundle/render performance, drift from the API contract.

Missing business logic, features, or improvement ideas are welcome — file them as Low-severity findings unless they hide a real defect.

## Output

1. **Verdict** — one line answering the brief's decision, plus the single deciding reason.
2. **Blockers** — the finite list that must clear to change the verdict (empty if none). Each: what it is, `file:line`, and which of the four bar clauses it meets.
3. **Findings** — every non-blocking item, tiered **High / Medium / Low** severity, sorted by confidence within tier. One line each: `[Sev/Conf] what — file:line — why it matters`, with `[tracked]` appended when it matches a TODO.md item.
4. **Docket updates** — applied to TODO.md directly, not pasted for the user: merge new findings in place continuing the docket's existing numbering/phase scheme, and close items the code shows are already done with a dated annotation. Report the list of changes made.
5. **External prerequisites** — outside-repo launch needs, not counted in the verdict.
