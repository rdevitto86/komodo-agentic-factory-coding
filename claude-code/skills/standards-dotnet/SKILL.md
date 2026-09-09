---
name: standards-dotnet
description: .NET framework and runtime conventions — hosting, configuration, packaging, deployment. Load before writing or reviewing any .csproj, .sln, or ASP.NET Core hosting/configuration file.
user-invocable: false
paths: "**/*.csproj, **/*.sln, **/Directory.Build.props, **/Directory.Packages.props, **/appsettings*.json, **/global.json, **/Program.cs"
---

# .NET

For any .NET repo — the framework and runtime layer: hosting, configuration, packaging, deployment. `standards-csharp` owns C# language mechanics (idioms, typing, error handling, async, testing); this skill never restates its content.

**`standards-csharp` always loads alongside this skill for a `.csproj` or `Program.cs` file** — both are already matched by C#'s own glob, so no glob extension is needed there. The purely declarative files this skill also matches — `.sln`, `Directory.Build.props`, `Directory.Packages.props`, `appsettings*.json`, `global.json` — carry no C# code, so no cross-load applies to them.

## Comment discipline

You must strictly limit code comments. **A non-compliant comment prompts the user for approval before the write lands — it does not fail outright.** That is deliberate while these directives are still being tuned: write only a comment you actually believe is warranted, since every miss costs the user a decision. **Deleting a comment you did not add always prompts too**, regardless of shape — moving or refactoring code is not licence to drop someone else's note. **Applies to every comment syntax**, not just `//` — block comments (`/* */`) and XML comments (`<!-- -->`) are scanned the same way.

Banned: a name echo (the comment's first word repeats the identifier or element below it); an implementation narrative (explaining *what* the file is doing, or describing standard syntax).

Allowed only: a compiler/linter directive (always allowed); a step marker (indented, <= 80 chars); a banner/section break (<= 40-char label); an intent/WHY comment using the `WHY:`, `NOTE:`, or `TODO(author/issue):` prefix.

`standards-csharp` states the exempt machine directive for `.cs` files (`#pragma warning disable/restore <rule>`). The other files this skill matches carry a different comment syntax, or none: the guard scans an XML comment (`<!-- -->`) in a `.csproj`, `Directory.Build.props`, or `Directory.Packages.props` under the same rules above — no XML-specific directive is exempt. `appsettings*.json` and `global.json` carry no native comment syntax at all, so nothing to guard there.

## Toolchain

- **SDK version floor is whatever `global.json`'s `version` pins.** A repo without one floats to whatever SDK the build machine happens to have installed — never reliable for CI, and never assume a version from memory.
- **The `.sln` file is the build unit CI invokes** (`dotnet build`/`dotnet test` against the solution, not project-by-project) — a project absent from the `.sln` silently never builds or tests in CI even though `dotnet build` on its own directory would succeed locally.
- **`Directory.Packages.props` centralizes NuGet package versions** across every project in the solution (`ManagePackageVersionsCentrally`) — the default. A project-level `<PackageReference Version="...">` alongside central package management is a deliberate override, not routine; `standards-csharp`'s own `<PackageReference>` guidance assumes this file resolves the version.
- **`Directory.Build.props`/`Directory.Build.targets`** hold MSBuild settings shared across every project in the solution (`<TargetFramework>`, `<Nullable>`, analyzer settings) — a setting duplicated in every individual `.csproj` instead belongs here.
- **The merge gate itself is owned by `standards-csharp`** (`dotnet format --verify-no-changes`, the analyzer run, `dotnet build`, `dotnet test`) — this skill adds nothing to that sequence, only the solution-wide scope it runs at.

## Conventions

- **`src/` and `tests/` at the solution root**, one project per assembly, referenced only through `<ProjectReference>` within the same solution — never a `<ProjectReference>` reaching outside it in place of a published NuGet package.
- **Composition root lives in `Program.cs`** using the minimal hosting model (`WebApplication.CreateBuilder`). One `AddXServices`/`AddXInfrastructure` extension method per feature area registered there, never scattered `services.AddX` calls with no grouping.
- **`appsettings.json` layers by environment** — `appsettings.json` holds defaults, `appsettings.{Environment}.json` holds overrides, environment variables override both. No environment-specific value belongs in the base file.
- **Bind configuration into strongly typed options** (`IOptions<T>`/`IOptionsSnapshot<T>` for values that can change without a restart) — a raw `IConfiguration["Some:Key"]` string lookup scattered through business logic has no compile-time check and no single place documenting what the app actually reads.
- **Secrets never live in a committed `appsettings*.json`.** Local development uses `dotnet user-secrets`; a deployed environment reads from the platform's secret manager (`standards-aws` or the repo's own chosen platform states which).

## Hosting

- **Kestrel sits behind a reverse proxy** (a load balancer or ingress) in every deployed environment — it is never the internet-facing edge on its own.
- **A dedicated health endpoint** (`/health`, `/ready`) registered through `IHealthChecksBuilder` is what a readiness/liveness probe targets — never the application root, which may depend on downstream state the probe shouldn't gate on.
- **Graceful shutdown honours the host's configured shutdown timeout.** In-flight requests and background work drain within that window; `standards-csharp`'s `IHostApplicationLifetime`/`ApplicationStopping` guidance covers the code that listens for it, this section covers only the timeout that bounds it.

## Packaging & deployment

- **Publish framework-dependent by default.** Self-contained, trimmed, or ReadyToRun publishing is a measured opt-in for a specific deployment constraint (a minimal container image, a target with no shared runtime) — never a default reached for convenience.
- **A container base image's tag matches the `TargetFramework`'s runtime version exactly** (`mcr.microsoft.com/dotnet/aspnet:<version>`, never `:latest`) — `standards-docker` states the rest of the image contract.
- **`global.json` travels with the repo, not just the CI config** — pinning the SDK only in a pipeline YAML lets a local `dotnet build` drift onto a different SDK than CI uses.

## Security standards

Framework/runtime-layer insecure-usage patterns for `/assess-security` to pull from, beyond `standards-api-security`'s generic OWASP checklist — `standards-csharp` covers the language-mechanics half (deserialization, XXE, `Process.Start`) once it exists.

- **`AddCors` policies never combine `AllowAnyOrigin()` with `AllowCredentials()`** — that combination is rejected by the spec for a reason; a wildcard origin serving credentialed requests defeats CORS entirely.
- **Anti-forgery tokens (`[ValidateAntiForgeryToken]`/`IAntiforgery`) are required on every state-changing endpoint reachable from a browser session**, not just ones a form happens to post to.
- **A committed `appsettings*.json` never carries a real connection string, API key, or signing secret** — the Conventions section above states where those live instead; a leaked one is a security incident, not a config oversight.
- **`RequireHttpsMetadata` stays `true` outside local development** — disabling it lets a token be validated over a downgraded, unencrypted channel.

## Testing

- **Integration tests against the host use `WebApplicationFactory<T>`**, spinning up the real DI container and middleware pipeline against an in-memory test server — never a hand-rolled `HttpClient` pointed at a separately launched process.
- **A dedicated `appsettings.Testing.json` (or environment-variable overrides passed to the factory) swaps only what a test needs** — a test database connection string, a faked external endpoint — never the whole configuration file.
- **`standards-csharp` owns unit-tier mechanics** (xUnit, fixtures, mocking, DB-touching integration tests) — this section covers only what changes when the ASP.NET Core host itself is under test.

## Quick-reference fields

The field set a .NET repo's `AGENTS.md` Quick-reference table carries, in addition to what `standards-csharp` already states. Every value is read from the repo, never assumed.

| Field | Source on disk |
|---|---|
| .NET SDK floor | `global.json` |
| Solution | `*.sln` |
| Hosting model | `Program.cs` |
| Central package management | `Directory.Packages.props` |

Drop a row whose value the repo genuinely lacks. Never add a row for a fact `standards-csharp` or this skill already states by name.

## Repo layout

This skill carries no `Repo layout — <token>` section. **Create is unsupported for .NET** in `git-repo-init` — no repo type token maps here yet, the same gap `standards-csharp` records. Use `git-repo-init`'s Scaffold path (doc-pair-only) instead, or add a `Repo layout` section here once a repo type is confirmed.
