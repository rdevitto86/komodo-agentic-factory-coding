---
name: standards-swift
description: Swift: optionals, value types, concurrency, and protocol design.
globs: ["**/*.swift", "**/Package.swift"]
---

# Swift

Principles live here; a version number never does — the language floor comes from the manifest (`swift-tools-version`, or the deployment target). Read those before assuming a feature or symbol exists.

## Memory & ownership

- **Value types by default.** A `struct` or `enum` is the starting point; reach for a `class` when identity matters (the thing *is* one object several holders observe), not when a value feels large. Copy-on-write means large is not automatically expensive.
- **Every closure that captures `self` on a stored reference declares its capture.** `[weak self]` when the closure outlives the call (stored, escaping, scheduled); no capture list when it does not. Risk is a property of the closure's lifetime, not of closures generally.
- **`unowned` is an assertion, not an optimization.** Use it only where the object provably outlives the closure, and accept the trap if wrong. When merely likely, `weak` is correct.
- **Reference-type lifetimes are explicit at the boundary.** A type handing out a delegate, an observer, or a continuation states which direction holds strongly — ARC will not surface the cycle.

## Optionals & errors

- **A force unwrap is a crash you chose.** `!` on an optional and `try!` both assert the `nil`/error path is impossible; if genuinely impossible, express that in the shape (a non-optional property, a failable initializer returning early); if not, handle it.
- **`guard let` for early exit, `if let` for a branch.** A function establishing preconditions reads top-down with `guard`; nesting `if let` pyramids is the pattern to refactor.
- **`throws` for a recoverable failure the caller must handle; an optional for absence.** Returning `nil` to mean "it failed for one of six reasons" discards the reason. Don't wrap a throwing call in `try?` unless the error truly does not matter there.
- **Never swallow an error silently.** A caught error is logged, converted to one the caller can act on, or discarded with a comment saying why it's safe.

## Concurrency

- **Structured concurrency over completion handlers.** `async`/`await` with `async let` and task groups for anything new; a completion-handler API is a boundary to wrap, not extend.
- **Actors own shared mutable state.** State reachable from more than one task lives inside an `actor` rather than behind a manual lock. The compiler's isolation checking is the guarantee; `@unchecked Sendable` discards it and needs a written justification.
- **`@MainActor` on anything touching UI**, declared at the type or property level, not hopped into at each call site.
- **Cancellation is cooperative.** A long-running task checks `Task.isCancelled` or calls a cancellation-aware API at its loop boundaries; a task that never checks cannot be cancelled.
- **An unstructured `Task { }` is an escape hatch.** It detaches from the surrounding scope's cancellation and priority, so it needs a reason — bridging from a non-async context is the usual one.

## API design & conventions

- **Clarity at the point of use beats brevity at declaration** — the language's own API Design Guidelines are the standard. Argument labels form a readable phrase at the call site; one repeating the parameter's type is removed.
- **Access control is stated, not defaulted into.** `public`/`open` are deliberate decisions about a surface someone else depends on; everything else is `internal` or tighter.
- **Protocol conformances live in their own extensions**, one per conformance, so members implementing a protocol sit together rather than scattered through the main declaration.
- **Prefer a protocol with an associated type or a generic constraint over a type-erased wrapper**; reach for `any` only where heterogeneous storage requires it.

## Build & toolchain

- **The manifest is the source of truth for the toolchain floor** — `swift-tools-version`, plus declared deployment targets.
- **Internal dependency graph** — SwiftPM ships it: `swift package describe` lists every target and what it depends on, `swift package show-dependencies` the package tree. Run it before scoring blast radius above low-med.
- **A formatter and a linter both run in CI**, configured by a committed file; a lint suppression is narrow, sits above the flagged line, and names the rule.
- **Dependencies are pinned by the resolved file, committed for an application target.** A library target does not commit it.

## Testing

- **The repo's declared test framework is the only one used** — read the manifest's test target rather than mixing frameworks. Tier definitions and coverage floors are owned by the sdlc standard; this covers mechanics only.
- **An async test awaits; it never sleeps.** A fixed delay to let work settle is a flaky test with a timer attached — await the actual signal, or inject a clock the test controls.
- **Test doubles are protocol-backed.** A type under test depends on a protocol the test can substitute, not a singleton reached through a static accessor.

## Security standards

- **Secrets live in the Keychain, never user defaults, a plist, or a source constant.** User defaults are an unencrypted file; anything held there is readable from a backup.
- **App transport security exceptions are individually justified.** A blanket allow-arbitrary-loads entry is a finding; a per-domain exception with a reason is a decision.
- **Nothing sensitive reaches the log stream** — tokens, credentials, personal data, full request bodies. Use the logging API's privacy annotations rather than assuming the default, which differs by type.
- **A biometric or device-owner prompt is authorization, not authentication** — it proves someone unlocked the device, so the secret it gates stays stored under access control requiring that authentication, not merely unlocked by app logic after the prompt.
- **Certificate pinning is a deliberate decision with a rotation plan**, taken at the boundary and written down — no rotation path is an outage waiting for the next renewal.
