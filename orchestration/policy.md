# Backend + Model Selection Policy

Single source for "Claude or local, and which model." Backend and model are a pure function of objective task properties, checked in order below — the advisor classifies what kind of task it is (routing it already does) and applies this table; it does not pick a backend freely. Companion file: `orchestration/models.yaml` (the tier → model map, editable data).

## Ordered conditions

Evaluate top to bottom. First match wins.

| # | Condition | Result |
|---|---|---|
| 1 | Manual override present (`runtime.json` per-agent override, or an explicit user instruction) | Wins over everything below |
| 2 | High-stakes trigger — auth, money, crypto, secrets, irreversible effect, security, concurrency, new schema/API (the advisor's cross-review MUST-list) | **Claude**, tier bumped up one level (safety floor) |
| 3 | Profile or task is heavy-tier (architecture, deep design/debug) | **Claude** |
| 4 | Everything else (standard / light work) | **Local**, Claude as automatic backup |
| 5 | Local model disabled (`enabled: false` in `models.yaml`) or unreachable | **Claude** (backup) |
| 6 | Genuine ambiguity about stakes | **Escalate to Claude / up-tier** — fail safe, never guess local on a maybe-sensitive task |

The only residual judgment is condition 1's "what kind of task is this" — the same classification the cross-review gate already performs. Failure mode is always safe: on ambiguity, resolution goes up (more capable, more expensive), never down.

## Tier resolution

A tier (`small` / `medium` / `large` / `heavy`) resolves to a concrete model per backend via `models.yaml`:

- `claude.<tier>` — always available, the universal fallback.
- `local.<tier>` — `{ model, enabled }`. `enabled: false` or an unloadable local model falls through to the Claude value for that tier automatically — no rule change needed, no error surfaced to the user.
- `overrides` — per-profile or per-task-class model pins (e.g. `security-review: fable`) that win over plain tier lookup. This is where a fixed floor like "security review never resolves below heavy" lives.

## Tier floors

A profile may declare a floor tier it never resolves below regardless of the task's apparent size — e.g. `quality-assurance`'s security-review function floors at `heavy` even though its frontmatter default tier is `medium`. Floors are expressed as `overrides` entries in `models.yaml`, not as a frontmatter field on the agent — the frontmatter `tier:` is the *default*, not a ceiling or floor.

## Native vs. bridge routing

- **Same-backend call** (Claude orchestrator calling a Claude profile) → native `Agent` tool spawn.
- **Cross-backend call** (Claude orchestrator calling a local profile, or vice versa) → route through the MCP bridge (`~/.komodo/bridge`). The bridge exposes profiles as a registry so backend is hidden from the caller; see `platforms/komodo-bridge/registry.generated.json`.

## Relationship to `runtime.json`

`runtime.json` at the repo root is the manual-override mechanism referenced in condition 1 above — `default_runtime` plus per-agent `overrides`, set via `scripts/set-runtime.sh`. It answers "which backend for this agent right now"; this policy file answers "absent an override, which backend and tier for this task." `runtime.json` wins when both apply.
