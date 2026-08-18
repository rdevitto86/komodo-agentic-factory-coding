---
name: coding-principles
description: Engineering doctrine: reuse order, DI, domain modelling, decomposition, resilience, API evolution.
---

# Principles

Language-independent doctrine. The language skills express these in their own idioms.

## Code reuse order

Before writing any non-trivial logic, check in this order:

1. **The language's shared SDK first.** If it covers the concern, use it. A claimed gap must be verified against the SDK's actual source before it is treated as real — then fixed directly in the SDK's own repo, not quietly worked around downstream and not merely flagged for someone else to fix later. Edit that repo like any other; the user handles its commit, merge to main, and release. The language skill names the package.
2. **A well-vetted library second.** Broad adoption, active maintenance, clear licence.
3. **Custom code last.** If new custom code is general-purpose, name it as a candidate for SDK extraction.
4. **Never add a dependency the project doesn't already use without asking first.**

## Structure and dependencies

- **Idiomatic over invented.** The right pattern is the one the ecosystem converged on. A custom abstraction replacing a language convention costs cognitive overhead with no payoff.
- **Read neighbouring code before writing new code.** Match the codebase's actual patterns — naming, error handling, wiring — not a generic reference shape.
- **Dependency injection over global state.** Inject HTTP clients, DB handles, clocks, loggers, and config through constructors. A struct handed its client takes a mock; one calling a package-level default cannot be tested without real I/O.
- **Accept interfaces, return concrete types.** Define the interface where it is consumed.
- **Contracts before implementation.** Settle the boundary — interface, request/response shape, error cases — before writing the logic behind it. A boundary discovered afterwards is shaped by the implementation's accidents.
- **Explicit wiring, zero magic.** Prefer passed parameters over `init()` side effects, auto-registration, and reflection wiring. Hidden coupling becomes a debugging liability.
- **Composition over inheritance.** Embed small focused types rather than building hierarchies.

## Domain modelling

- **Parse, don't validate.** Convert raw input into a typed domain value once, at the outer boundary. Downstream code receives something that cannot be invalid.
- **Make illegal states unrepresentable.** Required fields non-optional; mutually exclusive states as distinct variants, not co-existing nullable fields.
- **No primitive obsession.** `UserID`, `Milliseconds`, `Currency` — bare primitives make argument-order mistakes compile cleanly.
- **Orthogonal, single-responsibility units** that compose, rather than wide functions with mode flags.

## When a function is worth creating

A function is justified by the work it encapsulates, not by the existence of lines to move. A **single-call-site** function must be one of:

- **Dependent setup or wiring** — assembling dependencies so the consumer receives them ready to use.
- **A specific subset of functionality** — a cohesive unit describable without reference to its caller.
- **A concurrency unit** — a task body, worker loop, or parallel stage needing its own lifecycle.
- **A deliberate abstraction seam** — a boundary meant to be substituted or mocked.

If it is none of those, inline it. **Sequencing a handful of statements is not a subset of functionality — it is the call site.** Reuse across multiple call sites justifies extraction on its own; a single caller does not, and neither does the length of the enclosing function.

## Testability is a design constraint

If a design needs monkey-patching, real I/O, or elaborate environment setup to test a unit, the design is wrong. Raise it before implementation — retrofitting DI into global state is expensive; getting it right at the boundary costs nothing.

- **Keep the business core pure.** No disk, network, clock, or randomness in domain logic — those arrive injected or already read.
- **Push side effects to the edges.** I/O belongs in a thin outer layer that calls the core.

## State and concurrency

- **Default to immutability.** A function that quietly rewrites its caller's data is a bug waiting for a second caller.
- **Never share mutable state without explicit synchronisation.** Chosen deliberately, not assumed safe because only one task touches it today.
- **Every blocking operation carries a deadline.** An unbounded wait is an outage that has not happened yet.
- **Own every concurrent task's lifecycle.** Whoever starts it knows how it stops and who waits for it.

## Resilience

- **Scope-bound resource lifecycles.** Acquire and release in the same scope. Never rely on a later path to close what an earlier one opened.
- **Bound every queue, pool, and buffer** — and define the policy at the cap: reject, block, or shed. An unbounded queue turns a slow downstream into an out-of-memory crash.
- **Retry with exponential backoff and jitter; break the circuit on sustained failure.** Un-jittered retries synchronise into a thundering herd.
- **Make state-changing operations idempotent.** A retry with the same payload produces the same end state, not a duplicate effect.
- **Degrade gracefully.** Define the fallback when a non-critical dependency fails.

## Performance

- **Measure before optimising.** A speculative optimisation is unreviewable complexity.
- **Batch instead of looping.** N+1 is a defect, not a style preference.
- **Favour flat, contiguous data on measured critical paths.** Elsewhere prefer clarity.

## Errors and observability

- **Lead the message with a verb phrase**, never the function name — `failed to process order: %w`. The stack already knows where it came from.
- **Context belongs in structured fields**, not interpolated into the message string.
- **Wrap errors with context as they propagate**, preserving the cause so callers can still match on it.
- **Fail fast and loudly.** Never return a silent zero value in place of a real error — a swallowed failure surfaces later as corrupted data with no trace to its origin.
- **Log once, at the top of the stack.** Log-and-return fragments one failure into several.
- **Structured logs with key-value fields and a trace ID** — never formatted prose.

## Security at the design level

- **Least privilege by default.** Private, immutable, tightly scoped. Minimise exported surface.
- **Pass authority explicitly.** Identity, tokens, scopes, and trace IDs travel as parameters, never as ambient state. Implicit authority is authority nobody can audit.
- **Sanitise at every outbound boundary.** Parameterised queries, escaped shell arguments, encoded HTML. Never assemble a query or command by concatenation.
- **Constant-time comparison for secrets.** Normal equality leaks length and content through timing.

## Evolution

- **Change APIs and schemas additively.** New optional fields and endpoints are safe; renaming, removing, retyping, or tightening an existing field is breaking and needs a version.
- **Tolerate unknown and missing fields on internal wire formats.** Absent optional fields become a defined default, not a crash.
- **This does not loosen boundary validation.** At an untrusted external edge, unexpected fields are rejected. Reject at the perimeter; tolerate behind it.
