# Principles

Cross-cutting doctrine that applies to every agent and every implementation in this org. Agents reference this file rather than restating it. If a rule here conflicts with a project-level `CLAUDE.md`, the project file wins.

---

## 1. Hard rules

These are invariants. Treat as non-negotiable.

- **Never create git commits.** Only the user commits and merges code. Do not run `git commit` under any circumstance, even when asked to "save" or "finalize" work.
- **Error strings must not contain the function name.** Operation noun phrases only — function context belongs in metadata objects or stack traces, not the message string.
- **Doc comments must not open with the function/method name.** Write what the thing does, not "FuncName does X."

---

## 2. Code reuse priority

Before writing any non-trivial logic, check in this order:

1. **`komodo-forge-sdk-*` first** — `komodo-forge-sdk-go` / `komodo-forge-sdk-ts`. If the SDK covers it, use it. Do not reimplement SDK functionality. Flag SDK gaps so they can be filled upstream.
2. **Well-vetted open-source library second** — if the SDK doesn't cover it, a proven library beats custom code. Prefer broad adoption, active maintenance, clear licensing.
3. **Custom code last** — only when neither the SDK nor a suitable library exists. If new custom code is general-purpose, surface it as a candidate for SDK extraction.

Reviewing agents (advisor, swe on review) must enforce this order. Reinforce it when it wasn't followed.

---

## 3. Design — idiomatic, testable, explicit

These apply at every layer: system, service, and component.

- **Idiomatic over invented.** The correct pattern for a language or framework is the one its ecosystem converged on. Custom abstractions that replace language conventions add cognitive overhead without payoff.
- **Dependency injection over global / package-level state.** Inject dependencies (HTTP clients, DB handles, clocks, loggers, config) into structs via constructors or option functions. Package-level singletons couple callers to a specific implementation and require monkey-patching or real I/O to test. A struct that receives its `*http.Client` via constructor can be tested with a mock transport; one that calls `http.DefaultClient` cannot.
- **Accept interfaces, return concrete types.** Define the interface where it is consumed, not where it is implemented.
- **Explicit wiring, visible dependencies.** The dependency graph of a system should be readable at its construction points. `init()` side effects, auto-registration, and ambient context hide coupling that becomes an ops and debugging liability at scale.
- **Composition over inheritance.** Embed small, focused types rather than building deep hierarchies.

When you encounter package-level singletons, global state, or init-time side effects in new code, flag them. Name the specific testing or maintenance cost and offer the DI alternative.

---

## 4. Testability is a design constraint

If a design requires monkey-patching, real I/O, or complex environment setup to test a unit, the design is wrong. Raise it before implementation begins. Retrofitting DI into a global-state architecture is expensive; getting it right at boundary decisions costs almost nothing.
