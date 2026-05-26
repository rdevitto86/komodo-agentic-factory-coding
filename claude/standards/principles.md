# Principles

Cross-cutting doctrine that applies to every agent and every implementation in this org. Agents reference this file rather than restating it. If a rule here conflicts with a project-level `CLAUDE.md`, the project file wins.

---

## 1. Hard rules

These are invariants. Treat as non-negotiable.

- **Never create git commits or git branches.** Only the user commits, branches, and merges code. Do not run `git commit`, `git branch`, or `git checkout -b` under any circumstance — not even when asked to "save", "finalize", or "start on a feature". Always work on the current branch.
- **Error strings must start with a verb phrase (`"failed to X"`, `"invalid X"`, `"X not found"`) — never a colon-delimited prefix of any kind.** No function name, no package name, no noun-only prefix. Function context belongs in structured metadata or stack traces, not the message string. This is an enterprise logging standard; any prefix pattern creates noise and coupling in CloudWatch / Splunk / NewRelic outputs.
  - Bad (function/method name prefix): `"GetUserCredentials: unmarshal: %w"`, `"otp: GenerateAndStore: %w"`
  - Bad (noun-only prefix — no verb, still wrong): `"otp lookup: %w"`, `"otp: max attempts exceeded"`, `"cache get: %w"`
  - Good: `"failed to read user credentials: %w"`, `"failed to store OTP: %w"`, `"failed to look up OTP: %w"`, `"max OTP attempts exceeded"`
  - The same rule applies to logger message strings: `logger.Error("otp: ...")` and `logger.Error("cache get failed")` are both wrong forms — write `logger.Error("failed to look up OTP", ...)` and pass the error as a structured attribute.
- **Doc comments must not open with the function/method name, and must not be verbose multi-paragraph blocks.** Every exported function/method/type gets **1–3 sentences**: (1) what the function does, (2) any non-obvious contract (errors, side effects, preconditions), (3) why — only when rare and relevant. Lead with a verb (`Returns`, `Validates`, `Fetches`).
  - Bad (name-leading + restates signature): `// VerifyOTP looks up the stored OTP for the given email and compares it to the submitted code.\n// On a match, deletes the key immediately — each code is single-use.`
  - Bad (contract-only, says nothing about what the function does): `// Returns ErrInvalidOTP if the code does not match.`
  - Good: `// Validates the submitted OTP against the stored value. Returns ErrInvalidOTP on mismatch and deletes the key on success — codes are single-use.`
  - Good (single sentence): `// Fetches an order from DynamoDB by ID; returns ErrNotFound if no order exists.`

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
