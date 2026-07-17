# API Profile — the reproducible bones

Derived from `komodo-auth-api` (`~/komodo/platform/komodo-auth-api`), the canonical exemplar of a production-grade Komodo API. This file captures the **invariant shape** of a Komodo API and **how rigidly each part is held**. It is language-agnostic; the Go expression and a copy-me vertical slice live in `go-slice.md` — read that when working in Go.

Think of it as a house: the foundation, framing, electrical, and plumbing are reproduced to standard on every build; the interior — names, versions, business logic, specific routes — is free to vary. This file governs the bones, not the interior.

**Relationship to the other `api` files.** `blueprint.md` is the conformance *checklist* — pass/fail rows pointing at the standard that governs each, used by `/new-service` and `/api-audit`. This profile adds what the checklist does not carry: the **rigidity tiers**, the **SDK-first decision procedure and gap protocol**, and the **anti-patterns**. Where a bone maps to an existing standard, this file points at it rather than restating it.

**Precedence.** Additive to `~/.claude/standards/` (`principles.md`, `security.md`, `logging.md`, `comments.md`, `stack.md`). The immutable rules override the exemplar wherever the exemplar's own code diverges — in particular, the exemplar contains human-authored comments that are exempt for its author; **you reproduce its structure with zero comments.**

---

## 1. SDK-first — the first question on every task **[ENFORCED]**

The forge SDK (`komodo-forge-sdk-go`; `komodo-forge-sdk-ts` for infra) is deep and deepening. It is the default source for everything cross-cutting. As the SDK grows, the local surface of every API should **shrink**, not grow.

**Decision test — classify the capability before writing a line:**

- **Cross-cutting** — transport/server, middleware, error emission, request/response helpers, auth, secrets, cloud clients, logging, health, idempotency, infra constructs → **MUST come from the SDK.** Read the SDK source for the real signature; never guess it, never reimplement it.
- **This API's own domain logic** — its bounded business capability → local code, in a domain-scoped library (§3).

**SDK-gap protocol.** If a cross-cutting capability is missing or insufficient in the SDK: **stop — do not build a local workaround.** Surface `SDK gap: <capability>` to the advisor. The default resolution is to **extend the SDK**, not the API; a local implementation is an explicit advisor/user override, recorded in `TODO.md`. Every local reimplementation of a cross-cutting concern is a defect.

---

## 2. The bones, by rigidity tier

Three tiers govern how hard each bone is held:

- **[ENFORCED]** — non-negotiable. Deviation is a blocking defect: caught by a tool where one exists, otherwise by a mandatory review block.
- **[EXPECTED]** — the default. A reviewer pushes on deviations; a deviation needs a stated reason.
- **[FLEXIBLE]** — guidance; the agent chooses. Tooling and versions live here — prefer current/latest and keep the *usage* stable, even as the specific tool or version differs across APIs.

| Bone (invariant) | Tier | Held by / governed by |
|---|---|---|
| Zero comments anywhere (except test banners, machine directives) | ENFORCED | no-comments hook · `comments.md` |
| Every error wrapped with context; cause stays matchable; function name never in the string | ENFORCED | lint · `principles.md` §1 |
| Formatting/imports applied; lint config present with complexity budgets | ENFORCED | formatter + CI |
| Contract is the source of truth; downstream clients generated per-sibling; **no shared client package**; generated code kept fresh | ENFORCED | CI freshness gate · `blueprint.md` |
| Package-by-role layout; domain-scoped libraries (§3); one Service aggregate; interfaces declared at the consumer; clock injected | ENFORCED | review block · `principles.md` §3, `stack.md` §4 |
| SDK-first for all cross-cutting; SDK gaps escalated (§1) | ENFORCED | review block |
| Multi-stage, non-root container image; static build; healthcheck is the binary itself | ENFORCED | review block · devops docker standard |
| Single task-runner front door exposing build/run/test/codegen/deploy, with wrong-environment guards | ENFORCED | review block |
| Tiered tests — each tier its own directory + gate; component tests exercise the **real** aggregate | ENFORCED | review block · language test standard |
| Infra composed from SDK constructs; per-environment config extends SDK defaults | ENFORCED | review block |
| Uniform handler shape; public vs internal surface split across separate listeners | EXPECTED | review · `api/design.md` |
| Log messages are stable literals; values go in structured attributes, never interpolated | EXPECTED | review · `logging.md` |
| Security idioms — constant-time secret compares, body-size limits, response bodies drained+closed, no user enumeration | EXPECTED | review · `security.md` |
| Background work is disciplined — bounded, context-detached, timeout-bounded, panic-recovered | EXPECTED | review |
| Health vs readiness split; server timeouts set; deadlines/context propagated through the call chain | EXPECTED | review |
| Authorization by declaration — required scopes/audiences enforced in middleware, never in handlers | EXPECTED | review · `security.md` |
| Error-code namespace checked before defining new codes (codes are a shared cross-service namespace) | EXPECTED | review |
| Observability-sync (§4) — log↔alarm coupling bound by shared constants | EXPECTED | review |
| Docs shape — README section order, ADRs, runbooks, source-controlled diagrams | EXPECTED | review · `blueprint.md` D1 |
| Tool identities and versions (linter, base image, dependencies) | FLEXIBLE | prefer latest; keep usage stable |
| File/section naming, route names, TTLs, feature-flag names | FLEXIBLE | — |
| Which domain libraries exist and how many; pagination/versioning style | FLEXIBLE | — |

---

## 3. Domain-scoped library boundary **[ENFORCED shape]**

A domain-scoped library is the API's own bounded capability that the SDK deliberately does not host. In the exemplar these are the packages unique to that service; another API will have its own, named for its own concerns — the count and names are FLEXIBLE.

A capability earns its own package when **all** of these hold:
1. it is a bounded capability with a single responsibility;
2. it exposes its own interface and constructor;
3. it is mockable in isolation;
4. it is **not** cross-cutting (if it is, it belongs in the SDK — see §1).

Do not fragment one capability across packages, and do not dump unrelated logic into the HTTP/handler package.

---

## 4. Observability-sync **[EXPECTED]**

Log messages that infra alarms match on are a **contract, not free text.** When code emits a message an alarm keys on, bind the two with a shared constant (or a test that asserts the pairing), so editing a message cannot silently break an alarm. This is the coupling that ties the "stable log messages" bone (§2) to the infra bone (§2).

---

## 5. Anti-patterns — do not

- Reimplement a cross-cutting SDK capability locally (→ §1 gap protocol instead).
- Put domain logic in a handler, or authorization/business logic in middleware.
- Interpolate values into a log message — values go in structured attributes.
- Fire-and-forget a goroutine (unbounded, un-timed, un-recovered).
- Add a shared cross-service client package — services stay dependency leaves.
- Pin a tool or dependency version inside a standard — versions are FLEXIBLE.
- Reproduce the exemplar's comments — they are human-authored and exempt for its author; you are held to zero comments.
- Judge a high-risk change "small and safe" to skip review — smallness is never an exemption (the review trigger model lives in the advisor directive).

---

## 6. Reference

- **Exemplar:** `~/komodo/platform/komodo-auth-api`. Read it by reference for a concrete instance; never inline it into other work.
- **Go expression + copy-me vertical slice:** `go-slice.md`.
