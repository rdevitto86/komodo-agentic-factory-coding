# Principles

Cross-cutting doctrine that applies to every agent and every implementation in this org. Agents reference this file rather than restating it. If a rule here conflicts with a project-level `CLAUDE.md`, the project file wins.

---

## 1. Hard rules

These are invariants. Treat as non-negotiable.

- **Never create git commits or git branches.** Only the user commits, branches, and merges code. Do not run `git commit`, `git branch`, or `git checkout -b` under any circumstance — not even when asked to "save", "finalize", or "start on a feature". Always work on the current branch.
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

- **Idiomatic over invented.** The correct pattern for a language or framework is the one its ecosystem converged on. Custom abstractions that replace language conventions add cognitive overhead without payoff.
- **Dependency injection over global / package-level state.** Inject dependencies (HTTP clients, DB handles, clocks, loggers, config) via constructors or option functions. Package-level singletons couple callers to one implementation and force monkey-patching or real I/O to test: a struct given its `*http.Client` via constructor takes a mock transport; one calling `http.DefaultClient` cannot.
- **Accept interfaces, return concrete types.** Define the interface where it is consumed, not where it is implemented.
- **Explicit wiring, visible dependencies.** The dependency graph of a system should be readable at its construction points. `init()` side effects, auto-registration, and ambient context hide coupling that becomes an ops and debugging liability at scale.
- **Composition over inheritance.** Embed small, focused types rather than building deep hierarchies.

When you encounter package-level singletons, global state, or init-time side effects in new code, flag them. Name the specific testing or maintenance cost and offer the DI alternative.

---

## 4. Testability is a design constraint

If a design requires monkey-patching, real I/O, or complex environment setup to test a unit, the design is wrong. Raise it before implementation begins. Retrofitting DI into a global-state architecture is expensive; getting it right at boundary decisions costs almost nothing.

---

## 5. Concurrent work — ignore what isn't yours

Multiple agents, and the user, routinely work on the same checkout at once. Seeing uncommitted changes appear in files you didn't touch is normal, not an anomaly to investigate.

- **Scope your attention to your own task's files.** If `git status` or a diff shows edits outside the files/directories your task touches, ignore them — do not pause to ask about them, explain them, or fold them into your summary.
- **Only react if they break you.** If an unrelated change actually breaks a build, a test, or code your task depends on (a shared file changed under you, a compile error, a failing test in your path), stop and surface that specific breakage — not the mere presence of the change.
- **Never revert, stash, or "clean up" changes you didn't make**, even ones that look unrelated or unfinished — they may be another agent's or the user's in-progress work.
- This does not relax the git-safety protocol (still check before any destructive git command) — it only says: unrelated, non-breaking, uncommitted changes are not your concern.
