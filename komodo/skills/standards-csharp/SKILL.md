---
name: standards-csharp
description: C#: nullable reference types, async, LINQ, and exception discipline.
globs: ["**/*.cs", "**/Program.cs"]
---

# C#

Follow the comments standard: a doc line on every public function, a comment on a private one only when long or non-obvious.

## Conventions

- **The configured analyzer set must pass** (`.editorconfig` or ruleset). Suppress with `#pragma warning disable <rule>` paired with `restore` after the line, no prose; reasons go in `BACKLOG.md`. Prefer fixing the finding.
- **Naming**: `PascalCase` for types/methods/properties/public fields; `camelCase` for locals/parameters; `_camelCase` for private fields. No Hungarian notation, no `I` prefix beyond real interfaces.
- **No `Utils`/`Common`/`Helpers` grab-bag statics.** Split by domain; `internal` for anything not crossing the assembly boundary.
- **Static constructors and field initializers do no I/O.** Wire dependencies through DI, not a static singleton reaching out at load time.
- **Log once, at the top of the stack.** Never log-and-rethrow.
- **Avoid vague names** (`ProcessData`, `HandleStuff`, `DoWork`) — a name you cannot make specific usually marks a method that should not exist.
- **`var` only when the right side already shows the type** — a `new`, a cast, a literal. Otherwise use an explicit type.
- **Prefer a local function or lambda over a new class** when it captures state the caller already has. Skip on a hot path — each capturing closure allocates.

## Types and boundaries

- **Records for immutable value types**, classes for entities with identity and mutable state.
- **Parse at the edge**: bind requests into a DTO, convert once to the domain type; the service layer accepts only that type. Raw `JsonElement`/`dynamic`/`Dictionary<string, object>` has no boundary.
- **`<Nullable>enable</Nullable>` at the project level.** `T?` only for genuinely absent values; a member that must never be null stays non-nullable, set in the constructor.
- **Fallible construction returns `Result<T>` or throws a specific exception** — never a null the caller must remember to check.
- **Distinct types over bool flags** for mutually exclusive states — an enum or discriminated union, not a pair of nullable fields.
- **Interfaces stay small and live near the consumer** — one or two methods the caller needs, not a broad interface beside its only implementation.

## Async

- **`Async` suffix on every method returning `Task`/`Task<T>`/`ValueTask<T>`.** No sync-over-async (`.Result`, `.Wait()`, `.GetAwaiter().GetResult()`) outside a composition root or test harness.
- **`CancellationToken` is the last parameter** of I/O or blocking work, threaded through every awaited call — an accepted but unpassed token is dead.
- **`ConfigureAwait(false)` in library code**, omitted in ASP.NET Core where there is no synchronization context.
- **`IAsyncEnumerable<T>` for streaming results** rather than materializing a full `List<T>` first.
- **`Task.WhenAll` for fan-out**, letting the first faulted task propagate. A bare `async void` outside an event handler is a defect — its exceptions cannot be observed.

## Errors and resilience

- **Throw and catch specific exception types.** Never a bare `catch (Exception)` that swallows and continues; wrap with a descriptive phrase, no method name.
- **One project base exception type**, deriving framework and third-party exceptions into it where first caught.
- **`try`/`finally` or `using`/`await using` releases every `IDisposable`/`IAsyncDisposable`** in the acquiring scope.
- **Retries carry exponential backoff, jitter, and a cap** (Polly, not hand-rolled) — only idempotent, only transient errors.
- **Idempotency keys on state-changing endpoints**, so a retry converges, not double-writes.
- **Graceful shutdown**: honour `IHostApplicationLifetime`'s `ApplicationStopping`, drain in-flight work with a bound, dispose pools in reverse dependency order.
- **`CryptographicOperations.FixedTimeEquals`** for tokens, signatures, and secrets — never `==` or `string.Equals`.

## Performance

Measure first. An optimisation without a before/after number is unreviewable.

- **Profile with `dotnet-trace`/`dotnet-counters`** before optimising; benchmark hot paths with BenchmarkDotNet.
- **Pre-size** `List<T>`/`Dictionary<K,V>` with a known capacity — the largest low-effort win.
- **`ArrayPool<T>`/`ObjectPool<T>` only on measured hot paths** — they cost clarity elsewhere.
- **`StringBuilder`** for repeated concatenation in a loop; pass large structs `in`, not by value.
- **Batch queries and use `AsNoTracking()`** for read-only EF Core queries — N+1 is a bug, not tuning.

## Security standards

Language-specific insecure-usage patterns, beyond the api-security standard's generic OWASP checklist.

- **`FromSqlRaw`/`ExecuteSqlRaw` with an interpolated string is SQL injection** — use the parameterized overload instead.
- **`BinaryFormatter`/`ObjectStateFormatter`/`SoapFormatter` are unsafe on untrusted input** — use a data-only serializer (`System.Text.Json`) at a trust boundary.
- **`XmlDocument` with DTD processing and external entity resolution enabled is an XXE sink** — set `DtdProcessing.Prohibit` before parsing external XML.
- **`Process.Start` with an interpolated command-line string is command injection** — populate `ProcessStartInfo.ArgumentList` instead.
- **`RandomNumberGenerator`, never `System.Random`, for a token, key, or session ID.**

## Testing

- **xUnit only**, run via `dotnet test`; `[Theory]`/`[InlineData]` over hand-rolled loops.
- **Unit and component tests live in a sibling `*.Tests` project**, mirroring the namespace under test.
- **Fixtures via `IClassFixture<T>`/`ICollectionFixture<T>`**, not shared mutable static state; fluent assertions over bare `Assert.True`.
- **Mock at module boundaries** (an interface plus a fake or NSubstitute), never internals.
- **DB-touching code gets integration tests** against ephemeral instances (Testcontainers for .NET).
- **Tier definitions, merge/release gates, and coverage floors are owned by the sdlc standard**; this section covers C# mechanics only.
