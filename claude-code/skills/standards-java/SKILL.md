---
name: standards-java
description: Java standards — idioms, typing, error handling, concurrency, testing. Load before reading or writing any .java file, pom.xml, or build.gradle(.kts) file.
user-invocable: false
paths: "**/*.java, **/pom.xml, **/build.gradle, **/build.gradle.kts"
---

# Java

Zero comments, zero Javadoc. Exception messages lead with a verb phrase and never name the method.

## Comment discipline

`rules-commenting` carries the shared template contract. No Java-specific machine directive is registered in the guard's own exempt-prefix list — a `// CHECKSTYLE:OFF` line, a `@SuppressWarnings` annotation (not a comment, so the guard never sees it), or a `/** */` Javadoc block on a public member all prompt for approval the same as any other comment.

## Toolchain

- **Version floor is whatever the build file declares** — `<maven.compiler.release>` (or `<java.version>`) in `pom.xml`, or `sourceCompatibility`/`toolchain.languageVersion` in `build.gradle`/`build.gradle.kts`. Read it; never assume a release.
- **`Makefile`'s (or `Taskfile`'s) `verify` target is the merge gate** — `mvn -q verify` (or `./gradlew check`) chaining formatter check, static analysis, build, and test in that order. `context_injector.py` reads this target directly; a repo without it has no gate.
- **Formatting and linting** — `google-java-format` (or Spotless wrapping it) on commit, Checkstyle plus SpotBugs (with the `findsecbugs` plugin) as the gate. These are the tools the pre-commit hook runs; `standards-cicd` defines when.
- **Vulnerability scanning** — `mvn org.owasp:dependency-check-maven:check` (or the OWASP Dependency-Check Gradle plugin) against the resolved dependency tree is the gate; triage by whether the vulnerable path is actually reachable, not by CVE score alone. `standards-cicd` defines the gate; the `standards-api-security` skill states the bar.
- **Dependencies** via Maven `<dependency>` coordinates or Gradle `implementation`/`api` at a pinned version — never a local multi-module hack standing in for a published artifact outside the build's own modules.

## Conventions

- **Checkstyle and SpotBugs must both pass.** Config at `checkstyle.xml` and `spotbugs-exclude.xml`. A suppression is a bare `@SuppressWarnings("<rule>")` or a scoped SpotBugs `<Match>` exclusion with no appended prose; the reason goes in `BACKLOG.md`, never in a comment. Prefer fixing the finding.
- **Naming**: `PascalCase` for classes, interfaces, records, and enums; `camelCase` for methods, fields, and locals; `SCREAMING_SNAKE_CASE` for `static final` constants. No Hungarian notation, no `I` prefix on interfaces.
- **No `Utils` / `Common` / `Helpers` grab-bag classes.** Split by domain. Package-private (no modifier) for anything not meant to cross the package boundary; minimise the public surface.
- **Static initializers and field initializers do no I/O.** Wire through constructor injection (Spring `@Autowired` on the constructor, or plain manual wiring) — never a static singleton reaching out at class-load time.
- **Log once, at the top of the stack.** Never log-and-rethrow.
- **Avoid vague names** — `processData`, `handleStuff`, `doWork`. A name you cannot make specific usually marks a method that should not exist.
- **`var` only when the right-hand side already makes the type obvious** — a `new` expression, a cast, an explicit literal. An LLM-readable diff favours an explicit type on anything returned from a method call.
- **Prefer a lambda or method reference over a new class** when it captures state a caller already has in scope — a `Comparator`, a `Stream` predicate, a small callback passed to a single method. Skip it on a hot path: each capturing lambda allocates. Measure before choosing a lambda over a dedicated method in code the profiler already flags.

**A single-call-site method must earn its place** as one of: dependency wiring (a `@Bean` factory method, a builder step), a cohesive subset of functionality, a concurrency unit (a `Runnable`/`Callable` submitted independently, a virtual-thread task), or a deliberate abstraction seam (an interface swapped for a test double). Sequencing a handful of statements is not a subset of functionality — it is the call site.

## Types and boundaries

- **Records for immutable value types**, classes for entities with identity and mutable state. A `record` with a compact constructor beats a hand-written value class with generated equals/hashCode.
- **Parse at the edge.** Bind an incoming request into a DTO, convert once into the domain type, and let the service layer accept only the domain type. A method taking a raw `Map<String, Object>` or `JsonNode` has no boundary.
- **`sealed` interfaces with a fixed set of `permits` subtypes** for a closed set of variants; `switch` over them is exhaustive and the compiler enforces it.
- **`Optional<T>` only as a return type**, never as a field or a method parameter. A member that must never be absent after construction is set in the constructor, not defaulted and null-checked later.
- **Fallible construction throws a specific checked or unchecked exception** — never a null return the caller must remember to check.
- **Distinct types over boolean flags** and optional fields only meaningful in combination. Two mutually exclusive states are two types, an enum, or a sealed hierarchy, not a pair of nullable fields.
- **Interfaces stay small and live near the consumer.** One or two methods scoped to what the caller actually needs; avoid publishing a broad interface beside its only implementation.

## Concurrency and resources

- **Virtual threads (`Executors.newVirtualThreadPerTaskExecutor()`) for I/O-bound fan-out** on a version floor that supports them; a bounded platform-thread pool otherwise — never an unbounded `Executors.newCachedThreadPool()`.
- **`try-with-resources` releases every `AutoCloseable`** in the same scope that acquires it — a connection, a stream, a lock.
- **`CompletableFuture` composition (`thenCompose`, `allOf`) for async fan-out**, and let the first failed stage's exception propagate through `.join()`/`.get()` — a `Runnable` submitted with no result and no exception handler is a leak.
- **Every blocking call carries a timeout.** A `Future.get()`, an HTTP client call, or a connection-pool checkout with no deadline crossing a network boundary is a defect.
- **Do not mutate a caller-owned `List`/`Map`.** Return `List.copyOf(...)`/an unmodifiable view when ownership must not transfer.
- **`synchronized`, `java.util.concurrent.atomic`, or a `java.util.concurrent` collection — never "it is just one field."** A shared mutable field with no visibility guarantee is a race even when tests pass under low contention.
- **Guard the lock, not the method.** Never make a network or disk call while holding a `synchronized` block or a `Lock`.

## Errors and resilience

- **Catch specific exception types, never a bare `catch (Exception e)`** that swallows and continues; wrap with a descriptive phrase carrying no method name: `throw new OrderException("failed to query order by id", e)`.
- **One project base exception type**, deriving framework and third-party exceptions into it at the boundary where they're first caught.
- **Prefer unchecked exceptions for programmer error and unrecoverable failure**; reserve checked exceptions for conditions a caller can genuinely recover from and is expected to catch.
- **Retries carry exponential backoff, jitter, and a cap** (a library such as Resilience4j, not a hand-rolled loop). Only idempotent operations, only genuinely transient errors — never a 4xx-equivalent or a validation failure.
- **Idempotency keys on state-changing endpoints**, so a retry converges instead of double-writing.
- **Graceful shutdown**: register a JVM shutdown hook or honour the framework's own lifecycle callback (Spring's `SmartLifecycle`), drain in-flight work with a bound, close pools in reverse dependency order.
- **`MessageDigest.isEqual`** for tokens, signatures, and secrets — never `==` or `String.equals`.

## Performance

Measure first. An optimisation without a before/after number is unreviewable.

- **Profile with async-profiler or JFR (`jcmd <pid> JFR.start`)** before optimising; benchmark hot paths with JMH.
- **Pre-size** `ArrayList`/`HashMap` with a known capacity — the largest low-effort win.
- **Avoid boxing on a measured hot path** — primitive arrays and `IntStream`/`LongStream` over `List<Integer>`/`List<Long>`.
- **`StringBuilder`** for repeated concatenation in a loop.
- **Batch queries.** `EXPLAIN ANALYZE` before adding an index. N+1 is a bug, not a tuning opportunity.

## Testing

- **JUnit 5 (Jupiter) only**, run via `mvn test`/`./gradlew test`. `@ParameterizedTest` with `@MethodSource`/`@CsvSource` over hand-rolled loops for parameterised cases.
- **Unit and component tests live under `src/test/java`**, mirroring the package under test — the Maven/Gradle convention, never a sibling top-level test module.
- **AssertJ's fluent assertions** (`assertThat(...)`) over bare JUnit `assertEquals`/`assertTrue` on a hand-built boolean.
- **Mock at module boundaries** (an interface plus Mockito), never internals — a `final` class with no interface is a signal the boundary belongs one layer up.
- **DB-touching code gets integration tests** against ephemeral instances (Testcontainers).
- **Tier definitions, merge/release gates, and coverage floors are owned by `standards-sdlc`** — this section covers Java mechanics only.

## Quick-reference fields

The field set a Java repo's `AGENTS.md` Quick-reference table carries. Every value is read from the repo, never assumed.

| Field | Source on disk |
|---|---|
| Language + version floor | `pom.xml` (`<maven.compiler.release>`) or `build.gradle(.kts)` (`toolchain`/`sourceCompatibility`) |
| Build tool | `pom.xml` (Maven) or `build.gradle(.kts)` (Gradle), whichever is present |
| Artifact / group ID | `pom.xml` `<groupId>`/`<artifactId>` or `build.gradle(.kts)` `group`/`version` |
| Entrypoint | class carrying `public static void main` |
| Run + test | `Makefile` / `mvn`-`gradle` commands |

Drop a row whose value the repo genuinely lacks. Never add a row for a fact this skill or `standards-sdlc` already states by name.

## Repo layout

This skill carries no `Repo layout — <token>` section. **Create is unsupported for Java** in `repo-init` — no repo type token maps here yet. Use `repo-init`'s Scaffold path (doc-pair-only) instead, or add a `Repo layout` section here once a repo type is confirmed.
