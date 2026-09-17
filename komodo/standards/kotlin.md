# Kotlin

Conventions for engineering in Kotlin. Principles live here; a version number never does — the language and JVM floors come from the build's own declaration (the Kotlin plugin version, the `jvmToolchain`, and for an Android target the `compileSdk`/`minSdk` it sets), never from a release named here. Read the build files before assuming a language feature, a standard-library API, or a platform capability is available.

## Null safety & types

- **`!!` is a crash you chose.** It asserts a value is non-null at a point the compiler could not prove it; if the assertion holds, express it in the type instead (a non-nullable property, an early `return`/`throw` via `requireNotNull`), and if it does not, handle the null.
- **Platform types are the real hazard, not declared nullables.** A value crossing from Java arrives with unknown nullability and the compiler will not stop you dereferencing it. Annotate or wrap at the boundary rather than letting a platform type flow inward.
- **Sealed classes and interfaces model closed hierarchies** — a result, a UI state, a parse outcome — so a `when` over them is exhaustive and the compiler catches the branch you forgot when a case is added later. An open hierarchy of data holders with a type flag is the shape this replaces.
- **`data class` for values, `value class` for a wrapped primitive that deserves its own type.** An identifier, a currency amount, and a raw `String` that means one specific thing are all cheaper to make type-safe than to debug later.
- **`val` by default; `var` is a decision.** The same applies to collections — the read-only interface is the default declaration, and a mutable one is used where mutation is the point.

## Coroutines & concurrency

- **Every coroutine runs in a scope tied to a lifecycle.** `GlobalScope` detaches work from any cancellation signal and is a finding; a component launching work owns a scope that is cancelled when that component goes away.
- **Structured concurrency is the default shape** — `coroutineScope` and `supervisorScope` for concurrent children, `async`/`await` for parallel decomposition. A child's failure propagating to its siblings is a design choice made explicit by which of those two you pick.
- **A suspend function is main-safe.** It does its own dispatching internally rather than requiring the caller to know which dispatcher it needs; a caller wrapping every call in `withContext` is a symptom that the function broke that contract.
- **Cancellation is cooperative.** A long-running loop checks `isActive` or calls a suspending function that is itself cancellable; cleanup on cancellation goes in a `finally` block that uses a non-cancellable context if it must suspend.
- **Cold `Flow` for a stream computed per collector, hot `StateFlow`/`SharedFlow` for shared state or events.** State that must have a current value is a `StateFlow`; a one-shot event delivered to whoever is listening is not, because state replays and events must not.

## Errors

- **Exceptions for the genuinely exceptional; a sealed result type for an expected failure.** A network call that can plausibly fail returns a modelled outcome the caller must handle; an invariant violation throws.
- **Never catch a broad `Throwable` or `Exception` without rethrowing cancellation.** A coroutine's cancellation travels as an exception, so a blanket catch silently swallows it and the coroutine keeps running after it was cancelled.
- **`runCatching` is a boundary tool, not a control-flow tool.** It has exactly the broad-catch problem above; use it where a boundary genuinely needs to convert anything into a result, and rethrow cancellation explicitly there.

## Conventions

- **Scope functions follow one consistent mapping** — `let` for a nullable transform, `apply` for configuring the receiver, `also` for a side effect on it, `run`/`with` for a block that computes over it. Chains of them nested more than one deep are read-once code; break them up.
- **Extension functions add behavior to a type you do not own**, not to shorten a call you do. An extension that only ever applies in one file's context belongs as a private function.
- **Visibility is declared.** `internal` is the module boundary and is used deliberately; `public` (the default) on something never meant to leave its module is an accident to correct.
- **A `companion object` is not a namespace for unrelated helpers.** Constants and factory functions that genuinely belong to the type live there; anything else is a top-level declaration in its own file.

## Build & toolchain

- **The build is the source of truth for every floor** — the Kotlin plugin version, the JVM toolchain, and for an Android target its declared SDK levels. Read them rather than assuming.
- **Gradle Kotlin DSL with a version catalog.** Dependency coordinates and versions live in the catalog, not scattered across module build files as string literals.
- **Internal dependency graph** — module-to-module edges come from the wrapper the repo already commits: `./gradlew projects` for the module list, `./gradlew :<module>:dependencies` for what one module depends on. Below module level the Kotlin toolchain ships nothing of its own; the JDK's `jdeps` reads the compiled classes. Run it when a change crosses module boundaries; the edge count is the blast radius a review has to cover.
- **A formatter and a static-analysis tool both run in CI**, configured by a committed file; a suppression is narrow, sits on the declaration it applies to, and names the rule.

## Testing

- **The repo's declared test framework is the only one used** — read the test source set rather than mixing frameworks in one suite. Tier definitions and coverage floors are owned by the sdlc standard; this section covers mechanics only.
- **A coroutine test uses the coroutines test dispatcher and virtual time.** `Thread.sleep` in a test is a flake with a timer attached; advance the test scheduler instead.
- **A `Flow` test asserts on an ordered sequence of emissions**, including terminal completion, rather than sampling whatever value happens to be current when the assertion runs.

## Security standards

- **Secrets go to the platform keystore, never to shared preferences or a build constant.** Shared preferences are an unencrypted file; a constant compiled into the artifact is readable by anyone who unpacks it.
- **Nothing sensitive reaches the system log.** Tokens, credentials, personal data, and full request or response bodies stay out of it — a device log is readable far more widely than its API surface suggests.
- **Every exported component is exported on purpose.** An activity, service, receiver, or provider reachable from another app is an entry point that validates its input and enforces a permission; the manifest's export flag is stated explicitly rather than inherited from a default.
- **A `WebView` that enables JavaScript and adds a native bridge has published that bridge to whatever the page loads.** The bridge surface is minimal, the loaded origins are constrained, and untrusted content never gets one.
- **A biometric prompt is an authorization signal, not an authentication one** — it proves someone unlocked the device, so the key it gates is generated in the keystore with user-authentication required, rather than merely retrieved by app logic after a successful prompt.
- **Certificate pinning is a deliberate decision with a rotation plan**, declared at the network boundary and written down — pinning with no rotation path is an outage waiting for the next certificate renewal.
