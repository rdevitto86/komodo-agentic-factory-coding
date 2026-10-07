---
name: standards-dotnet
description: Project layout, target frameworks, dependency injection, and configuration.
globs: ["**/*.cs", "**/*.csproj", "**/*.sln", "**/Directory.Build.props", "**/Program.cs", "**/global.json"]
---

# .NET

For any .NET repo — the framework and runtime layer: hosting, configuration, packaging, deployment. the csharp standard owns C# language mechanics (idioms, typing, error handling, async, testing); this standard never restates its content.

**the csharp standard always loads alongside this standard for a `.csproj` or `Program.cs` file** — both are already matched by C#'s own glob, so no glob extension is needed there. The purely declarative files this standard also matches — `.sln`, `Directory.Build.props`, `Directory.Packages.props`, `appsettings*.json`, `global.json` — carry no C# code, so no cross-load applies to them.

## Conventions

- **`src/` and `tests/` at the solution root**, one project per assembly, referenced only through `<ProjectReference>` within the same solution — never a `<ProjectReference>` reaching outside it in place of a published NuGet package.
- **Composition root lives in `Program.cs`** using the minimal hosting model (`WebApplication.CreateBuilder`). One `AddXServices`/`AddXInfrastructure` extension method per feature area registered there, never scattered `services.AddX` calls with no grouping.
- **`appsettings.json` layers by environment** — `appsettings.json` holds defaults, `appsettings.{Environment}.json` holds overrides, environment variables override both. No environment-specific value belongs in the base file.
- **Bind configuration into strongly typed options** (`IOptions<T>`/`IOptionsSnapshot<T>` for values that can change without a restart) — a raw `IConfiguration["Some:Key"]` string lookup scattered through business logic has no compile-time check and no single place documenting what the app actually reads.
- **Secrets never live in a committed `appsettings*.json`.** Local development uses `dotnet user-secrets`; a deployed environment reads from the platform's secret manager (the aws standard or the repo's own chosen platform states which).

## Hosting

- **Kestrel sits behind a reverse proxy** (a load balancer or ingress) in every deployed environment — it is never the internet-facing edge on its own.
- **A dedicated health endpoint** (`/health`, `/ready`) registered through `IHealthChecksBuilder` is what a readiness/liveness probe targets — never the application root, which may depend on downstream state the probe shouldn't gate on.
- **Graceful shutdown honours the host's configured shutdown timeout.** In-flight requests and background work drain within that window; the csharp standard's `IHostApplicationLifetime`/`ApplicationStopping` guidance covers the code that listens for it, this section covers only the timeout that bounds it.

## Packaging & deployment

- **Publish framework-dependent by default.** Self-contained, trimmed, or ReadyToRun publishing is a measured opt-in for a specific deployment constraint (a minimal container image, a target with no shared runtime) — never a default reached for convenience.
- **A container base image's tag matches the `TargetFramework`'s runtime version exactly** (`mcr.microsoft.com/dotnet/aspnet:<version>`, never `:latest`) — the docker standard states the rest of the image contract.
- **`global.json` travels with the repo, not just the CI config** — pinning the SDK only in a pipeline YAML lets a local `dotnet build` drift onto a different SDK than CI uses.

## Security standards

Framework/runtime-layer insecure-usage patterns beyond the api-security standard — the csharp standard covers the language-mechanics half (deserialization, XXE, `Process.Start`) once it exists.

- **`AddCors` policies never combine `AllowAnyOrigin()` with `AllowCredentials()`** — that combination is rejected by the spec for a reason; a wildcard origin serving credentialed requests defeats CORS entirely.
- **Anti-forgery tokens (`[ValidateAntiForgeryToken]`/`IAntiforgery`) are required on every state-changing endpoint reachable from a browser session**, not just ones a form happens to post to.
- **A committed `appsettings*.json` never carries a real connection string, API key, or signing secret** — the Conventions section above states where those live instead; a leaked one is a security incident, not a config oversight.
- **`RequireHttpsMetadata` stays `true` outside local development** — disabling it lets a token be validated over a downgraded, unencrypted channel.

## Testing

- **Integration tests against the host use `WebApplicationFactory<T>`**, spinning up the real DI container and middleware pipeline against an in-memory test server — never a hand-rolled `HttpClient` pointed at a separately launched process.
- **A dedicated `appsettings.Testing.json` (or environment-variable overrides passed to the factory) swaps only what a test needs** — a test database connection string, a faked external endpoint — never the whole configuration file.
- **the csharp standard owns unit-tier mechanics** (xUnit, fixtures, mocking, DB-touching integration tests) — this section covers only what changes when the ASP.NET Core host itself is under test.
