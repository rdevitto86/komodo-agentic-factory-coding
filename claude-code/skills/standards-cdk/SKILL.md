---
name: standards-cdk
description: CDK and AWS — stack layout, SDK constructs, exports, safety rules. Load before writing any .ts CDK stack file.
user-invocable: false
paths: "**/cdk/**, **/infra/**, **/infrastructure/**, **/*-stack.ts, **/*.stack.ts"
---

# CDK

For any AWS CDK repo — the deliberate, opt-in exception to the Terraform standard. A repo chooses CDK for a specific reason (see its own AGENTS.md); Terraform stays the default everywhere else.

**`standards-typescript` always loads alongside this skill** — every file this skill's `paths:` matches is already a `.ts` file matched by typescript's own glob, so no glob extension is needed. Its conventions, toolchain, and Quick-reference fields apply here without restatement.

## Comment discipline

`standards-typescript`'s directives apply unchanged — a CDK stack file is still a `.ts` file the guard scans identically. No CDK-specific machine directive exists beyond what `standards-typescript` already lists.

## Toolchain

TypeScript can run natively on Node with no build step (`cdk.json` invoking `node bin/app.ts` directly). If a repo does this: internal imports need explicit `.ts` extensions, and type-only imports must use `import type` — nothing is transpiling first, so Node runs the source exactly as written.

**`Makefile`'s `verify` target is the merge gate**, delegating to `package.json` scripts (`lint`, `typecheck`, `test`, `build` — here `build` means `cdk synth`, asserting every stack synthesizes cleanly). `context_injector.py` reads this target directly; a repo without it has no gate.

## Stack layout

- One class per stack, one file per stack, kebab-case filename holding a PascalCase class.
- `bin/app.ts` is the one entrypoint: resolves the environment, builds the App, applies tags, instantiates each stack.
- Environment resolution fails closed — an unrecognized environment or a missing account aborts before anything synthesizes.
- No `lib/constructs/` folder for anything reusable — reusable belongs in the shared toolkit (see Construct reuse).

## Cross-stack references

- Never reach into another stack's resources by copying an ID string around. Pass references as explicit constructor arguments in `bin/app.ts`, so the whole dependency graph is visible in one file.
- Every stack publishes its own resource identifiers via `CfnOutput` with a stable `exportName` — that export is the contract; renaming one is a breaking change.

## Naming & exports

Every published export follows one pattern, so any consumer can look a resource up without reading the CDK source:

```
komodo-<service>-<env>-<resource>
```

`<service>` and `<env>` come from the resolved config; `<resource>` names the thing and its identifier kind — the table's ARN, the queue's URL.

- Names are derived from a single config file — never written inline in a stack. Adding an environment means one new entry there, not touching every stack.
- AWS itself refuses to delete an export while something still depends on it. That is the enforcement mechanism, not just a naming convention someone has to remember.

## Construct reuse

- Default to the shared CDK toolkit published by the TypeScript SDK — the `standards-typescript` skill names the package. A gap there gets fixed with an upstream PR, not a local workaround; the fix should benefit every repo, not just this one.
- **A local construct is allowed only when it is genuinely unique to this repo** — for example, a one-off Lambda-backed construct with no equivalent building block anywhere in the SDK. The test is reusability, not convenience: if the same shape would plausibly get built by a second team, it belongs upstream, not local.
- Never write a local version of something the SDK already provides, even if the SDK's version is inconvenient to use — fix the SDK instead.
- Raw `aws-cdk-lib`, unwrapped, is acceptable only where no SDK construct exists yet. Track the eventual SDK migration in `BACKLOG.md` rather than treating the raw usage as permanent.

## Config authority

- One config file resolves environment name → account, region, and every resource name. Every stack receives a fully resolved config object; no stack reads an environment variable or CLI flag on its own.
- A single flag (e.g. `isProtected`) can drive several safety settings at once — retention policy, deletion protection, point-in-time recovery together — so a protected environment can't end up with one turned on and another forgotten.

## Environment isolation

- One AWS account per environment is the default posture — a mistake in one environment cannot physically reach another. A single shared account with name-prefix isolation is a deliberate downgrade, not the default.
- Non-production environments are destroyable. Anything holding real data in a protected environment gets `RemovalPolicy.RETAIN` and deletion protection.

## Safety

- Never hardcode an account ID, ARN, or region — derive it from the config file.
- A destroy plan touching a stateful resource (database, bucket holding real data) stops and asks — never apply it as part of a routine change.
- Secrets are referenced by ARN from Secrets Manager, never embedded in stack code or committed config.

## Testing

- Assert against the **synthesized CloudFormation template** (`aws-cdk-lib/assertions`), never against CDK object properties in memory — the template is what AWS actually receives and acts on.
- A snapshot per environment catches an accidental resource replacement (a rename that looks safe in code but deletes-and-recreates a stateful resource in AWS) before it ships.
- Component-tier tests check the deployed blueprint's actual shape; unit-tier tests cover environment resolution — unknown environment, missing account, derived names, protection flags.

## Quick-reference fields

The field set a CDK repo's `AGENTS.md` Quick-reference table carries. Every value is read from the repo, never assumed.

| Field | Source on disk |
|---|---|
| CDK + runtime floor | `package.json` |
| App entrypoint | `bin/app.ts` |
| Stacks | `lib/` stack files |
| Environments | env resolver in `bin/` |
| Why CDK not Terraform | this repo's `AGENTS.md` |

Drop a row whose value the repo genuinely lacks. Never add a row for a fact this skill or `standards-typescript` already states by name.

## Repo layout — `cdk-infra`

```
bin/app.ts
lib/
config/
test/
cdk.json
package.json
tsconfig.json
Makefile
```

`bin/app.ts` is the sole entrypoint (see Stack layout). `lib/` holds one file per stack. `config/` holds the single environment-resolution file (see Config authority) — never more than one.

## Seed backlog — `cdk-infra`

Stories `generate-repo` splices into `Cross-Cutting` on Create, or appends if missing on Scaffold/Refresh.

- [H] Instantiate the first stack · M
- [M] Observability: wire logging/metrics/tracing (`standards-observability`) · S
- [M] Tests: unit + component coverage · S → `make test`
