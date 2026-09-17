# C#

Follow the comments standard: a doc line on every public function, a comment on a private one only when long or non-obvious.

## Conventions

- **The configured analyzer set must pass.** Config at `.editorconfig` (and any analyzer package's own ruleset file). A suppression is a bare `#pragma warning disable <rule>` paired with a `#pragma warning restore <rule>` immediately after the offending line, with no appended prose; the reason goes in `BACKLOG.md`, never in a comment. Prefer fixing the finding.
- **Naming**: `PascalCase` for types, methods, properties, and public fields; `camelCase` for locals and parameters; `_camelCase` for private fields. No Hungarian notation, no `I` prefix beyond genuine interfaces.
- **No `Utils` / `Common` / `Helpers` static grab-bag classes.** Split by domain. `internal` access modifier for anything not meant to cross the assembly boundary; minimise the public surface.
- **Static constructors and field initializers do no I/O.** Wire through dependency injection (`IServiceCollection` registrations, constructor injection) — never a static singleton reaching out at type-load time.
- **Log once, at the top of the stack.** Never log-and-rethrow.
- **Avoid vague names** — `ProcessData`, `HandleStuff`, `DoWork`. A name you cannot make specific usually marks a method that should not exist.
- **`var` only when the right-hand side already makes the type obvious** — a `new` expression, a cast, an explicit literal. An LLM-readable diff favours an explicit type on anything returned from a method call.
- **Prefer a local function or lambda over a new class** when it captures state a caller already has in scope — a LINQ predicate, a small callback passed to a single method. Skip it on a hot path: each closure that captures a variable allocates. Measure before choosing a lambda over a dedicated method in code the profiler already flags.

**A single-call-site method must earn its place** as one of: dependency wiring (`AddXServices`, a factory method), a cohesive subset of functionality, a concurrency unit (a `Task`-returning method run via `Task.Run` or awaited independently), or a deliberate abstraction seam (an interface swapped for a test double). Sequencing a handful of statements is not a subset of functionality — it is the call site.

## Types and boundaries

- **Records for immutable value types**, classes for entities with identity and mutable state. A `record` with a `with`-expression beats a hand-written copy constructor.
- **Parse at the edge.** Bind an incoming request into a DTO, convert once into the domain type, and let the service layer accept only the domain type. A method taking a raw `JsonElement`, `dynamic`, or `Dictionary<string, object>` has no boundary.
- **`<Nullable>enable</Nullable>` at the project level.** A reference type that can genuinely be absent is `T?`; a member that must never be null after construction stays non-nullable and is set in the constructor, not defaulted and checked later.
- **Fallible construction returns a `Result<T>` or throws a specific exception type** — never a null return the caller must remember to check.
- **Distinct types over bool flags** and optional fields only meaningful in combination. Two mutually exclusive states are two types, an enum, or a discriminated union pattern (a sealed base record with case subtypes), not a pair of nullable fields.
- **Interfaces stay small and live near the consumer.** One or two methods scoped to what the caller actually needs; avoid publishing a broad interface beside its only implementation.

## Async

- **`Async` suffix on every method returning `Task`/`Task<T>`/`ValueTask<T>`.** A public API exposes only async methods for I/O; no sync-over-async wrapper (`.Result`, `.Wait()`, `.GetAwaiter().GetResult()`) outside a composition root or test harness.
- **`CancellationToken` is the last parameter** of anything doing I/O or blocking work, threaded through to every awaited call. A token accepted but never passed downstream is dead.
- **`ConfigureAwait(false)` in library code**, omitted in ASP.NET Core code where there is no synchronization context to avoid.
- **`IAsyncEnumerable<T>` for streaming results** rather than materializing a full `List<T>` before returning.
- **`Task.WhenAll` for fan-out**, and let the first faulted task's exception propagate — a bare `async void` outside an event handler is a defect, since its exceptions cannot be observed by the caller.

## Errors and resilience

- **Throw specific exception types, catch specific exception types.** Never a bare `catch (Exception)` that swallows and continues; wrap with a descriptive phrase carrying no method name: `throw new OrderException("failed to query order by id", ex)`.
- **One project base exception type**, deriving framework and third-party exceptions into it at the boundary where they're first caught.
- **`try`/`finally` or `using`/`await using` releases every `IDisposable`/`IAsyncDisposable`** in the same scope that acquires it — a connection, a stream, a lock.
- **Retries carry exponential backoff, jitter, and a cap** (a library such as Polly, not a hand-rolled loop). Only idempotent operations, only genuinely transient errors — never a 4xx-equivalent or a validation failure.
- **Idempotency keys on state-changing endpoints**, so a retry converges instead of double-writing.
- **Graceful shutdown**: honour `IHostApplicationLifetime`'s `ApplicationStopping` token, drain in-flight work with a bound, dispose pools in reverse dependency order.
- **`CryptographicOperations.FixedTimeEquals`** for tokens, signatures, and secrets — never `==` or `string.Equals`.

## Performance

Measure first. An optimisation without a before/after number is unreviewable.

- **Profile with `dotnet-trace`/`dotnet-counters`** before optimising; benchmark hot paths with BenchmarkDotNet.
- **Pre-size** `List<T>`/`Dictionary<K,V>` with a known capacity — the largest low-effort win.
- **`ArrayPool<T>`/`ObjectPool<T>` only on measured hot paths.** They cost clarity everywhere else.
- **`StringBuilder`** for repeated concatenation in a loop. Pass large structs `in`, not by value.
- **Batch queries and use `AsNoTracking()` for read-only EF Core queries.** N+1 is a bug, not a tuning opportunity.

## Security standards

Language-specific insecure-usage patterns a security review pulls from, beyond the api-security standard's generic OWASP checklist.

- **`FromSqlRaw`/`ExecuteSqlRaw` with an interpolated string is SQL injection**; use the parameterized overload (`FromSqlInterpolated` or explicit `SqlParameter`s) instead.
- **`BinaryFormatter`/`ObjectStateFormatter`/`SoapFormatter` are unsafe on untrusted input** — .NET's own deserialization advisories name them directly; use a data-only serializer (`System.Text.Json`) for anything crossing a trust boundary.
- **`XmlDocument`/`XmlReaderSettings` with `DtdProcessing.Parse` and external entity resolution enabled is an XXE sink** — set `DtdProcessing.Prohibit` (or `XmlResolver = null`) before parsing externally sourced XML.
- **`Process.Start` with a single interpolated command-line string is command injection** — populate `ProcessStartInfo.ArgumentList` instead.
- **`RandomNumberGenerator`, never `System.Random`, for a token, key, or session ID.**

## Testing

- **xUnit only**, run via `dotnet test`. `[Theory]`/`[InlineData]` over hand-rolled loops for parameterised cases.
- **Unit and component tests live in a sibling `*.Tests` project** (`Foo/` paired with `Foo.Tests/`), mirroring the namespace under test, never inside the shipped assembly.
- **Fixtures via `IClassFixture<T>`/`ICollectionFixture<T>`**, not shared mutable static state. Fluent assertions (a library such as `Shouldly` or the project's chosen assertion package) over bare `Assert.True` on a hand-built boolean.
- **Mock at module boundaries** (an interface plus a fake or a mocking library such as NSubstitute), never internals — a `sealed` class with no interface is a signal the boundary belongs one layer up.
- **DB-touching code gets integration tests** against ephemeral instances (Testcontainers for .NET).
- **Tier definitions, merge/release gates, and coverage floors are owned by the sdlc standard** — this section covers C# mechanics only.
