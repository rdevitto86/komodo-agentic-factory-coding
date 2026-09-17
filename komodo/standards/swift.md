# Swift

Conventions for engineering in Swift. Principles live here; a version number never does — the language floor comes from the target's own manifest (`Package.swift`'s `swift-tools-version`, or the project's deployment target), and the platform SDK floor from the deployment target, not from a release named here. Read those before assuming a language feature, a concurrency API, or a standard-library symbol exists.

## Memory & ownership

- **Value types by default.** A `struct` or `enum` is the starting point; reach for a `class` when identity matters (the thing *is* one object that several holders observe), not when a value merely feels large. The standard library's copy-on-write collections mean a large value type is not automatically an expensive one.
- **Every closure that captures `self` on a stored reference declares its capture.** `[weak self]` when the closure outlives the call (stored, escaping, scheduled); no capture list is needed when it does not. The retain-cycle risk is a property of the closure's lifetime, not of closures in general — a capture list on a non-escaping closure is noise.
- **`unowned` is an assertion, not an optimization.** Use it only where the captured object provably outlives the closure, and accept that it traps if that assertion is wrong. When the lifetime relationship is merely likely, `weak` is the correct choice.
- **Reference-type lifetimes are explicit at the boundary.** A type handing out a delegate, an observer, or a continuation states which direction holds strongly, because ARC will not surface the cycle at compile time.

## Optionals & errors

- **A force unwrap is a crash you chose.** `!` on an optional and `try!` are both assertions that the `nil` or the error path is impossible; if it is genuinely impossible, prefer a shape that expresses that (a non-optional stored property, a failable initializer that returns early), and if it is not, handle it.
- **`guard let` for early exit, `if let` for a branch.** A function establishing its preconditions reads top-down with `guard`; nesting `if let` pyramids instead is the pattern to refactor.
- **`throws` for a recoverable failure the caller must handle; an optional for absence.** Returning `nil` to mean "it failed for one of six reasons" discards the reason — that is what an error type is for. Don't wrap a throwing call in `try?` unless the specific error genuinely does not matter at that call site.
- **Never swallow an error silently.** A caught error is logged, converted to one the caller can act on, or explicitly discarded with a comment saying why that path is safe.

## Concurrency

- **Structured concurrency over completion handlers.** `async`/`await` with `async let` and task groups for anything new; a completion-handler API is a boundary to wrap, not a pattern to extend.
- **Actors own shared mutable state.** State reachable from more than one task lives inside an `actor` rather than behind a manual lock. The compiler's isolation checking is the guarantee; a `@unchecked Sendable` conformance discards it and needs a written justification.
- **`@MainActor` on anything that touches UI**, declared at the type or property level rather than hopped into at each call site.
- **Cancellation is cooperative.** A long-running task checks `Task.isCancelled` or calls a cancellation-aware API at its loop boundaries; a task that never checks cannot be cancelled no matter who asks.
- **An unstructured `Task { }` is an escape hatch.** It detaches from the surrounding scope's cancellation and priority, so it needs a reason — bridging from a non-async context is the usual legitimate one.

## API design & conventions

- **Clarity at the point of use beats brevity at the point of declaration** — the language's own API Design Guidelines are the standard here, not a house dialect. Argument labels form a readable phrase at the call site; a label that repeats the parameter's type is removed.
- **Access control is stated, not defaulted into.** `public` and `open` are deliberate decisions about a surface someone else depends on; everything else is `internal` or tighter. A framework target's public surface is reviewed as an API, not as an implementation detail.
- **Protocol conformances live in their own extensions**, one per conformance, so the members implementing a protocol sit together rather than scattered through the main declaration.
- **Prefer a protocol with an associated type or a generic constraint over a type-erased wrapper**; reach for `any` only where heterogeneous storage genuinely requires it.

## Build & toolchain

- **The manifest is the source of truth for the toolchain floor** — `Package.swift`'s `swift-tools-version` line, plus the platform deployment targets it declares. Read them rather than assuming a feature is available.
- **Internal dependency graph** — SwiftPM ships it: `swift package describe` lists every target and the targets it depends on, `swift package show-dependencies` the package-level tree. Below target level the toolchain ships nothing. The review's blast-radius score reads this output: run it before scoring anything above low-med, and say so when the fan-out was judged instead of walked.
- **A formatter and a linter both run in CI**, configured by a file committed to the repo; a lint suppression is narrow, sits directly above the flagged line, and names the rule.
- **Dependencies are pinned by the resolved file, and the resolved file is committed** for an application target. A library target does not commit it.

## Testing

- **The repo's declared test framework is the only one used** — read the manifest's test target rather than mixing frameworks in one suite. Tier definitions and coverage floors are owned by the sdlc standard; this section covers mechanics only.
- **An async test awaits; it never sleeps.** A fixed delay to let work settle is a flaky test with a timer attached — await the actual signal, or inject a clock the test controls.
- **Test doubles are protocol-backed.** A type under test depends on a protocol the test can substitute, rather than on a concrete singleton reached through a static accessor.

## Security standards

- **Secrets live in the Keychain, never in user defaults, a plist, or a source constant.** User defaults are an unencrypted file; anything held there is readable from a backup.
- **App transport security exceptions are individually justified.** A blanket allow-arbitrary-loads entry is a finding; a per-domain exception with a written reason is a decision.
- **Nothing sensitive reaches the log stream.** Tokens, credentials, personal data, and full request bodies stay out; the logging API's privacy annotations are used rather than assumed, since the default for interpolated values differs by type.
- **A biometric or device-owner authentication prompt is an authorization signal, not an authentication one** — it proves someone unlocked the device, so the secret it gates is still stored under access control that requires that authentication, rather than merely unlocked by app logic after a successful prompt.
- **Certificate pinning is a deliberate decision with a rotation plan**, taken at the boundary and written down — pinning with no rotation path is an outage waiting for the next certificate renewal.
