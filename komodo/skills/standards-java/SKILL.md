---
name: standards-java
description: Java: nullability, immutability, streams, checked exceptions, and build layout.
globs: ["**/*.gradle", "**/*.java", "**/pom.xml"]
---

# Java

Follow the comments standard.

## Conventions

- **Checkstyle and SpotBugs must both pass** (`checkstyle.xml`, `spotbugs-exclude.xml`). Suppress with `@SuppressWarnings("<rule>")` or a scoped SpotBugs `<Match>`, no prose; reasons go in `BACKLOG.md`. Prefer fixing the finding.
- **Naming**: `PascalCase` for classes/interfaces/records/enums; `camelCase` for methods/fields/locals; `SCREAMING_SNAKE_CASE` for `static final` constants. No Hungarian notation, no `I` prefix.
- **No `Utils`/`Common`/`Helpers` grab-bag classes.** Split by domain; package-private for anything not crossing the package boundary.
- **Static initializers and field initializers do no I/O.** Wire through constructor injection, not a static singleton reaching out at class-load time.
- **`var` only when the right side already shows the type** — a `new`, a cast, a literal. Otherwise use an explicit type.
- **Prefer a lambda or method reference over a new class** capturing state the caller already has. Skip on a hot path — each capturing lambda allocates.

## Types and boundaries

- **Records for immutable value types**, classes for entities with identity and mutable state.
- **Parse at the edge**: bind requests into a DTO, convert once to the domain type; the service layer accepts only that type. A raw `Map<String,Object>`/`JsonNode` has no boundary.
- **`sealed` interfaces with a fixed `permits` set** for a closed group of variants; `switch` over them is exhaustive and compiler-enforced.
- **`Optional<T>` only as a return type**, never a field or parameter. A member that must never be absent is set in the constructor.
- **Fallible construction throws a specific checked or unchecked exception** — never a null the caller must check.
- **Distinct types over boolean flags** for mutually exclusive states — an enum or sealed hierarchy, not nullable fields.
- **Interfaces stay small and live near the consumer** — one or two methods the caller needs, not a broad interface beside its only implementation.

## Concurrency and resources

- **Virtual threads (`Executors.newVirtualThreadPerTaskExecutor()`) for I/O-bound fan-out** where supported, a bounded pool otherwise — never unbounded `newCachedThreadPool()`.
- **`try-with-resources` releases every `AutoCloseable`** in the acquiring scope — a connection, a stream, a lock.
- **`CompletableFuture` composition (`thenCompose`, `allOf`) for async fan-out**, letting the first failed stage propagate via `.join()`/`.get()`. An unhandled `Runnable` submission leaks.
- **Every blocking call carries a timeout** — `Future.get()`, an HTTP call, or a pool checkout with no deadline is a defect.
- **Do not mutate a caller-owned `List`/`Map`.** Return `List.copyOf(...)` or an unmodifiable view.
- **`synchronized`, `java.util.concurrent.atomic`, or a concurrent collection — never "it is just one field."** A field with no visibility guarantee races.
- **Guard the lock, not the method.** Never make a network or disk call while holding a `synchronized` block or a `Lock`.

## Errors and resilience

- **Catch specific exception types, never a bare `catch (Exception e)`** that swallows and continues; wrap with a descriptive phrase, no method name.
- **One project base exception type**, deriving framework/third-party exceptions into it where first caught.
- **Prefer unchecked exceptions for programmer error and unrecoverable failure**; reserve checked exceptions for what a caller can genuinely recover from.
- **Retries carry exponential backoff, jitter, a cap** (Resilience4j, not hand-rolled) — only idempotent, transient errors.
- **Idempotency keys on state-changing endpoints**, so retries converge, not double-write.
- **Graceful shutdown**: a JVM shutdown hook or framework callback, draining work with a bound, closing pools in reverse order.
- **`MessageDigest.isEqual`** for tokens, signatures, and secrets — never `==` or `String.equals`.

## Performance

Measure first. An optimisation without a before/after number is unreviewable.

- **Profile with async-profiler or JFR** before optimising; benchmark hot paths with JMH.
- **Pre-size** `ArrayList`/`HashMap` with a known capacity.
- **Avoid boxing on a measured hot path** — primitive arrays and `IntStream`/`LongStream` over `List<Integer>`.
- **`StringBuilder`** for repeated loop concatenation.
- **Batch queries; `EXPLAIN ANALYZE` before an index.** N+1 is a bug, not tuning.

## Security standards

Language-specific insecure-usage patterns beyond the api-security standard.

- **`PreparedStatement` with bound parameters, never `Statement` with a concatenated string.**
- **`ObjectInputStream.readObject()` on untrusted input is Java's classic deserialization sink** — restrict to a trusted allowlist (`ObjectInputFilter`) or avoid native serialization.
- **`DocumentBuilderFactory`/`XMLInputFactory`/`SAXParserFactory` process external entities by default** — disable DTDs before parsing external XML, or it's an XXE sink.
- **`Runtime.exec`/`ProcessBuilder` with an interpolated command string is command injection** — pass the argument list instead.
- **`SecureRandom`, never `java.util.Random`, for a token, key, or session ID.**
- **A template engine's expression language (OGNL, SpEL, Thymeleaf) on request-derived text is expression injection**, not a formatting convenience.

## Testing

- **JUnit 5 (Jupiter) only**, run via `mvn test`/`./gradlew test`; `@ParameterizedTest` with `@MethodSource`/`@CsvSource` over hand-rolled loops.
- **Unit and component tests live under `src/test/java`**, mirroring the package under test.
- **AssertJ's fluent assertions** (`assertThat(...)`) over bare JUnit `assertEquals`/`assertTrue`.
- **Mock at module boundaries** (an interface plus Mockito), never internals.
- **DB-touching code gets integration tests** against ephemeral instances (Testcontainers).
- **Tier definitions, merge/release gates, and coverage floors are owned by the sdlc standard**; this section covers Java mechanics only.
