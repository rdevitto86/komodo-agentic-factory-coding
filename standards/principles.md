# Principles

Cross-cutting doctrine that applies to every agent and every implementation in this org. Agents reference this file rather than restating it. If a rule here conflicts with a project-level `CLAUDE.md`, the project file wins.

---

## 1. Hard rules

These are invariants. Treat as non-negotiable.

- **Never commit, push, branch, or merge.** Only the user does those. Do not run `git commit`, `git push` (in any form, not just `--force`), `git branch`, `git checkout -b`/`switch -c`, or `git merge` under any circumstance — not even when asked to "save", "finalize", "ship it", or "start on a feature", and never ask permission to do so either. Reading history (`log`, `diff`, `show`, `blame`, `status`) is fine. Always work on the current branch.
- **Never spawn agents into an isolated worktree.** Do not pass `isolation: "worktree"` (or any equivalent that gives a spawned agent its own git worktree/branch). Every agent — spawned or not — edits files directly on the user's currently checked-out branch. Isolated worktrees fragment one piece of work into parallel trees that are painful to merge and prone to conflicts. Structured branches and real PRs are a deliberate, future, user-driven step — not an agent default.
- **Error strings must start with a verb phrase (`"failed to X"`, `"invalid X"`, `"X not found"`) — never a colon-delimited prefix of any kind.** No function name, no package name, no noun-only prefix. Function context belongs in structured metadata or stack traces, not the message string. This is an enterprise logging standard; any prefix pattern creates noise and coupling in CloudWatch / Splunk / NewRelic outputs.
  - Bad (function/method name prefix): `"GetUserCredentials: unmarshal: %w"`, `"otp: GenerateAndStore: %w"`
  - Bad (noun-only prefix — no verb, still wrong): `"otp lookup: %w"`, `"otp: max attempts exceeded"`, `"cache get: %w"`
  - Good: `"failed to read user credentials: %w"`, `"failed to store OTP: %w"`, `"failed to look up OTP: %w"`, `"max OTP attempts exceeded"`
  - The same rule applies to logger message strings: `logger.Error("otp: ...")` and `logger.Error("cache get failed")` are both wrong forms — write `logger.Error("failed to look up OTP", ...)` and pass the error as a structured attribute.
- **All comments follow `~/.claude/standards/comments.md` exactly.** It is the single source of truth for comment rules across every language and file — zero comments, including function docs, with no exceptions. Do not restate comment rules or show comment examples here or in any other file.
- **Never resolve a conflict or capability gap by unverified judgment call.** Before concluding a library, SDK, or system "doesn't support X," verify against the actual source or docs — do not rely on recall. If verification still leaves you without what you need, stop and escalate (agent → advisor → user) before implementing a workaround. Full rule, including the escalation path and SDK-gap handling: `~/.claude/standards/assumptions.md`.

---

## 2. Code reuse priority

Before writing any non-trivial logic, check in this order:

1. **`komodo-forge-sdk-*` first** — `komodo-forge-sdk-go` / `komodo-forge-sdk-ts`. If the SDK covers it, use it. Do not reimplement SDK functionality. A claimed gap must be verified against the SDK's actual source/docs before it's treated as real (`assumptions.md`); once confirmed, flag it so it can be filled upstream instead of quietly working around it.
2. **Well-vetted open-source library second** — if the SDK doesn't cover it, a proven library beats custom code. Prefer broad adoption, active maintenance, clear licensing.
3. **Custom code last** — only when neither the SDK nor a suitable library exists. If new custom code is general-purpose, surface it as a candidate for SDK extraction.

Reviewing agents (advisor, software-engineer on review) must enforce this order. Reinforce it when it wasn't followed.

---

## 3. Design — idiomatic, testable, explicit

These apply at every layer: system, service, and component.

### 3.1 Structure and dependencies

- **Idiomatic over invented.** The correct pattern for a language or framework is the one its ecosystem converged on. Custom abstractions that replace language conventions add cognitive overhead without payoff.
- **Dependency injection over global / package-level state.** Inject dependencies (HTTP clients, DB handles, clocks, loggers, config, environment) via constructors or option functions. Package-level singletons couple callers to one implementation and force monkey-patching or real I/O to test: a struct given its `*http.Client` via constructor takes a mock transport; one calling `http.DefaultClient` cannot.
- **Accept interfaces, return concrete types.** Define the interface where it is consumed, not where it is implemented.
- **Contracts before implementation.** Settle the boundary — the interface, the request/response shape, the error cases — before writing the logic behind it. A boundary discovered after the implementation is a boundary shaped by the implementation's accidents.
- **Explicit wiring, visible dependencies, zero magic.** The dependency graph should be readable at its construction points. Prefer explicit calls and passed parameters over implicit framework behavior — `init()` side effects, auto-registration, runtime reflection wiring, and ambient context hide coupling that becomes an ops and debugging liability at scale.
- **Composition over inheritance.** Embed small, focused types rather than building deep hierarchies.

When you encounter package-level singletons, global state, or init-time side effects in new code, flag them. Name the specific testing or maintenance cost and offer the DI alternative.

### 3.2 Domain modeling

- **Parse, don't validate.** Convert raw input into a strongly-typed domain value at the outer boundary, once. Downstream code receives a type that cannot be invalid, so it never re-checks and never guesses whether an earlier layer already did.
- **Make illegal states unrepresentable.** Shape types so an invalid combination cannot be constructed — required fields non-optional, mutually exclusive states as distinct variants rather than co-existing nullable fields. A state a caller can't build is a class of bug that can't ship.
- **No primitive obsession.** Give domain meaning its own type (`UserID`, `Milliseconds`, `Currency`) instead of passing bare `string`/`int` around. Bare primitives make argument-order mistakes compile cleanly.
- **Orthogonal, single-responsibility units.** Build small pieces that compose predictably rather than wide functions with mode flags and interacting options.

### 3.3 Decomposition — when a function is worth creating

A function is justified by the work it encapsulates, not by the existence of lines to move. This is the default test for **any** function, and the binding test for one with a single call site.

A single-call-site function must be one of these:

- **Dependent setup or wiring** — assembling and injecting dependencies so the thing that uses them receives them ready to use.
- **A specific subset of functionality** — a cohesive unit of work that stands on its own and could be described without reference to its caller.
- **A concurrency unit** — a goroutine/task body, worker loop, or parallel stage that needs its own scope and lifecycle.
- **An abstract subset of functionality** — a deliberate seam: a boundary meant to be substituted, mocked, or swapped, per §3.1 and §4.

If it is none of those, inline it. Sequencing a handful of statements is not a subset of functionality — it is the call site. Naming a conversion, an assignment, or an operation together with the log line that reports it does not create an abstraction; it adds a layer of indirection a reader must open to learn nothing. Reuse across multiple call sites justifies extraction on its own; a single caller does not, and neither does the length of the enclosing function.

Do not describe this rule to others with illustrative snippets. Apply the four tests above directly.

---

## 4. Testability is a design constraint

If a design requires monkey-patching, real I/O, or complex environment setup to test a unit, the design is wrong. Raise it before implementation begins. Retrofitting DI into a global-state architecture is expensive; getting it right at boundary decisions costs almost nothing.

- **Keep the business core pure.** Domain logic performs no disk, network, clock, or randomness calls — those arrive as injected dependencies or as already-read values. A pure core tests deterministically without fixtures, containers, or sleeps.
- **Push side effects to the edges.** I/O belongs in a thin outer layer that calls the core, not interleaved through it.

---

## 5. Concurrent work — ignore what isn't yours

Multiple agents, and the user, routinely work on the same checkout at once. Seeing uncommitted changes appear in files you didn't touch is normal, not an anomaly to investigate.

- **Scope your attention to your own task's files.** If `git status` or a diff shows edits outside the files/directories your task touches, ignore them — do not pause to ask about them, explain them, or fold them into your summary.
- **Only react if they break you.** If an unrelated change actually breaks a build, a test, or code your task depends on (a shared file changed under you, a compile error, a failing test in your path), stop and surface that specific breakage — not the mere presence of the change.
- **Never revert, stash, or "clean up" changes you didn't make**, even ones that look unrelated or unfinished — they may be another agent's or the user's in-progress work.
- This does not relax the git-safety protocol (still check before any destructive git command) — it only says: unrelated, non-breaking, uncommitted changes are not your concern.

---

## 6. State, concurrency & memory

- **Default to immutability.** Return new values rather than mutating parameters or shared state in place. A function that quietly rewrites its caller's data is a bug waiting for a second caller.
- **Never share mutable state without explicit synchronization.** Locks, channels, or atomics — chosen deliberately, not assumed safe because "only one goroutine touches it today." Unsynchronized sharing that races is a defect even when it passes.
- **Every blocking operation carries a deadline.** Network calls, DB queries, lock acquisition, queue reads: explicit timeout plus a propagated cancellation signal. An unbounded wait is an outage that hasn't happened yet.
- **Own every concurrent task's lifecycle.** Whoever starts it knows how it stops, how its failure surfaces, and who waits for it. No orphaned background work.
- **Predictable allocation on hot paths.** Pre-size slices, maps, and buffers when the size is known; reuse buffers rather than reallocating per iteration. Applies to measured hot paths only — elsewhere it is noise.

---

## 7. Resilience & operational safety

- **Scope-bound resource lifecycles.** Acquire and release in the same scope using the language's cleanup construct (`defer`, `using`, `try-with-resources`). Never rely on a later code path to close what an earlier one opened.
- **Bound every queue, pool, and buffer.** Caps are mandatory, and so is the policy for what happens at the cap — reject, block, or shed. An unbounded queue converts a slow downstream into an out-of-memory crash.
- **Retry with exponential backoff and jitter; break the circuit on sustained failure.** Un-jittered retries synchronize clients into a thundering herd; retrying into a downed dependency extends its outage. Fail fast once a dependency is known bad.
- **Design state-changing operations to be idempotent.** A retried request with the same payload must produce the same end state, not a duplicate effect. Retries are a certainty at every layer, not an edge case.
- **Degrade gracefully.** When a non-critical dependency fails, define the fallback — cached value, static default, reduced feature — instead of failing the whole request.

---

## 8. Performance & I/O

- **Measure before optimizing.** Profile to find the real hot path; a speculative optimization is unreviewable complexity.
- **Batch instead of looping.** Group database and network calls into batch or bulk operations. An N+1 query pattern is a defect, not a style preference.
- **Favor flat, contiguous data on critical paths.** Deeply nested pointer graphs cost cache locality where it matters; elsewhere, prefer clarity.

---

## 9. Errors & observability

- **Wrap errors with context as they propagate**, preserving the cause so callers can still match on it. Error-string format is fixed by §1.
- **Fail fast and loudly.** Never return a silent zero value, empty result, or null in place of a real error — a swallowed failure surfaces later as corrupted data with no trace back to its origin.
- **Log once, at the top of the stack.** Logging and returning the same error duplicates noise and fragments the story of a single failure.
- **Structured, machine-readable logs** with key-value fields and a trace ID — never formatted prose strings. Full rules: `~/.claude/standards/logging.md`.

---

## 10. Security & boundary safety

Full baseline: `~/.claude/standards/security.md`. The design-level rules that shape code structure:

- **Least privilege by default.** Private, immutable, tightly scoped — widen only when a caller genuinely needs it. Minimize exported surface.
- **Pass authority explicitly.** Auth identity, tokens, scopes, and trace IDs travel as parameters through the call chain, never via globals or ambient state. Implicit authority is authority nobody can audit.
- **Sanitize at every outbound boundary.** Parameterized queries, escaped shell arguments, encoded HTML, validated external payloads. Never assemble a query or command by string concatenation.
- **Constant-time comparison for secrets**, tokens, signatures, and hashes. A normal equality check leaks length and content through timing.

---

## 11. Evolution & compatibility

- **Change APIs and schemas additively.** New optional fields and new endpoints are safe; renaming, removing, retyping, or tightening an existing field is a breaking change that requires an explicit version.
- **Tolerate unknown and missing fields on internal wire formats.** A consumer must not break when a producer adds a field it doesn't recognize, and must handle absent optional fields as a defined default rather than a crash.
- **This does not loosen boundary validation.** At an untrusted external edge — public API, upload, webhook — unexpected fields are rejected per `security.md` §2. Toleration applies to internal service-to-service and versioned payloads, where forward compatibility is the goal. Reject at the perimeter; tolerate behind it.
