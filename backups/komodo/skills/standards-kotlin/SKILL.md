---
name: standards-kotlin
description: Kotlin: null safety, coroutines, data classes, and interop with Java.
globs: ["**/*.kt", "**/*.kts", "**/build.gradle.kts"]
---

# Kotlin

Principles live here; a version number never does — language and JVM floors come from the build's own declaration (the Kotlin plugin version, `jvmToolchain`, Android's `compileSdk`/`minSdk`). Read the build files before assuming a feature or API is available.

## Null safety & types

- **`!!` is a crash you chose.** It asserts non-null the compiler could not prove; express it in the type (`requireNotNull`), or handle the null.
- **Platform types are the real hazard, not declared nullables.** A value crossing from Java arrives with unknown nullability and the compiler will not stop a dereference. Annotate or wrap at the boundary.
- **Sealed classes and interfaces model closed hierarchies** — a result, a UI state, a parse outcome — so `when` over them is exhaustive and the compiler catches a forgotten branch later.
- **`data class` for values, `value class` for a wrapped primitive deserving its own type.** An identifier or a currency amount is cheaper to make type-safe than to debug later.
- **`val` by default; `var` is a decision.** Same for collections: read-only by default, mutable where mutation happens.

## Coroutines & concurrency

- **Every coroutine runs in a scope tied to a lifecycle.** `GlobalScope` detaches work from any cancellation signal; a component owns a scope cancelled when it goes away.
- **Structured concurrency is the default shape** — `coroutineScope`/`supervisorScope` for concurrent children, `async`/`await` for parallel decomposition. Whether a child's failure propagates to siblings is the choice between them.
- **A suspend function is main-safe.** It dispatches internally; the caller need not know which dispatcher it needs. Wrapping every call in `withContext` signals the function broke that contract.
- **Cancellation is cooperative.** A long-running loop checks `isActive` or calls a cancellable suspending function; cleanup goes in `finally`, using a non-cancellable context if it must suspend.
- **Cold `Flow` for a stream computed per collector, hot `StateFlow`/`SharedFlow` for shared state or events.** State needing a current value is a `StateFlow`; a one-shot event is not — state replays, events must not.

## Errors

- **Exceptions for the genuinely exceptional; a sealed result type for an expected failure.** A network call returns a modelled outcome the caller must handle; an invariant violation throws.
- **Never catch a broad `Throwable`/`Exception` without rethrowing cancellation.** Cancellation travels as an exception; a blanket catch swallows it, and the coroutine keeps running.
- **`runCatching` is a boundary tool, not a control-flow tool** — it has the broad-catch problem above. Use it only where a boundary converts anything into a result, and rethrow cancellation explicitly.

## Conventions

- **Scope functions follow one mapping** — `let` for a nullable transform, `apply` to configure the receiver, `also` for a side effect, `run`/`with` for a block computing over it. Nesting more than one deep is read-once code; break it up.
- **Extension functions add behavior to a type you do not own**, not to shorten a call you do. One used in one file's context belongs as a private function.
- **Visibility is declared.** `internal` is the module boundary, used deliberately; `public` (the default) on something never meant to leave is an accident to fix.
- **A `companion object` is not a namespace for unrelated helpers.** Only constants and factory functions genuinely belonging to the type live there; anything else is top-level.

## Build & toolchain

- **The build is the source of truth for every floor** — plugin version, JVM toolchain, and, for Android, declared SDK levels.
- **Gradle Kotlin DSL with a version catalog.** Dependency coordinates and versions live in the catalog, not as string literals scattered across build files.
- **Internal dependency graph** comes from the repo's own wrapper: `./gradlew projects` for the module list, `:dependencies` for what one depends on; `jdeps` reads compiled classes below module level. Run it before scoring blast radius above low-med.
- **A formatter and a static-analysis tool both run in CI**, configured by a committed file; a suppression is narrow, on the declaration, naming the rule.

## Testing

- **The repo's declared test framework is the only one used** — read the test source set rather than mixing frameworks. Tier definitions and coverage floors are owned by the sdlc standard; this covers mechanics only.
- **A coroutine test uses the coroutines test dispatcher and virtual time.** `Thread.sleep` in a test is a flake with a timer attached; advance the scheduler instead.
- **A `Flow` test asserts on an ordered sequence of emissions**, including completion, not by sampling whatever value is current when the assertion runs.

## Security standards

- **Secrets go to the platform keystore, never shared preferences or a build constant.** Preferences are an unencrypted file; a constant compiled into the artifact is readable by anyone who unpacks it.
- **Nothing sensitive reaches the system log** — tokens, credentials, personal data, full request/response bodies. A device log is readable far more widely than its API suggests.
- **Every exported component is exported on purpose.** An activity, service, receiver, or provider reachable from another app validates its input and enforces a permission; the export flag is explicit, never inherited.
- **A `WebView` enabling JavaScript and adding a native channel has published it to whatever the page loads.** Keep the channel minimal, constrain loaded origins, never give untrusted content one.
- **A biometric prompt is authorization, not authentication** — it proves someone unlocked the device, so the key it gates is generated in the keystore requiring user authentication, not retrieved after the prompt.
- **Certificate pinning is a deliberate decision with a rotation plan**, written down — no rotation path is an outage waiting for the next renewal.
