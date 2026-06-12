# CHANGELOG.md

Standard for maintaining `CHANGELOG.md` in internal SDKs and libraries — `komodo-forge-sdk-*` and any other package consumed by other Komodo projects.

## When to update

In the **same change set** as the code change, not as a follow-up:
- New, changed, or removed public API
- Behavior change a consumer would notice
- Bug fix that affects consumer-visible behavior
- Deprecation or removal, with migration guidance

**Skip it for:** internal refactors with no surface change, test-only changes, doc typos, CI/tooling tweaks — anything that doesn't change what a consumer sees.

## Format

[Keep a Changelog](https://keepachangelog.com/) style — reverse-chronological, grouped under `## [Unreleased]` until a release is cut:

```
## [Unreleased]
### Added
- Short description of the new capability, from the consumer's view

### Changed
- What behavior changed and what a consumer needs to do differently

### Fixed
- What was broken and what now works

### Removed
- What's gone and what replaces it (or that nothing does)
```

Each entry is one line, plain language, written from the **consumer's** perspective — not implementation detail. Omit empty sections.

## Version bumps

Whenever you update `CHANGELOG.md`, also bump the package's version — same diff, same commit. Don't leave it stale for someone to notice later.

**Where it lives:** `VERSION` file at the package root, or the version field in whatever manifest the package already uses (`package.json`, `Cargo.toml`, etc). Match the existing convention — don't introduce a new one.

**How much to bump (semver `MAJOR.MINOR.PATCH`), driven by the changelog sections you just wrote:**
- **Patch** (`x.y.Z`) — `### Fixed` only: bug fixes with no API surface change. *Example: fixing a bug in a Go SDK takes `1.3.7` → `1.3.8`.*
- **Minor** (`x.Y.0`) — any `### Added`, or a `### Changed` that's backward compatible (no consumer code needs to change). A large internal rewrite that preserves the public contract — e.g. swapping the DynamoDB layer under a stable interface — is still a minor bump, not major: the version communicates compatibility, not effort.
- **Major** (`X.0.0`) — any breaking change: removed/renamed public API, changed signatures, behavior a consumer depended on now differs. Anything in `### Removed`, or a breaking `### Changed` entry.

When a change set spans multiple sections, bump by the **highest** category present (major > minor > patch).

**Pre-1.0 packages (`0.y.z`):** breaking changes bump MINOR, everything else bumps PATCH, per semver's pre-1.0 convention. If a `0.x` package looks stable enough to be someone's `1.0.0`, say so — don't cut it yourself.

**When the call is genuinely ambiguous** — boundary case, unclear whether something is breaking — say so and ask rather than guess. A wrong major/minor call is expensive for every downstream consumer; getting it wrong is worse than asking.

## Who maintains it

`software-engineer` updates `CHANGELOG.md` and the version in the same diff as the code — not a separate pass — as part of any change to a published SDK/library. `business-architect` flags when a story touches a published lib so neither is missed, and checks for both before calling the item done.
