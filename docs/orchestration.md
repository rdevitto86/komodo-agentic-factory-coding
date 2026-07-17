# Orchestration Redesign — Spec

Status: converged design, pre-implementation. Captures the decisions from the design session so they don't have to be re-derived. This describes the target state; the current repo still runs the legacy "agent = role + welded model" model until migrated.

## Goals (the reasons every decision below exists)

1. Reduce redundant sessions — small/medium work runs in-context, not as cold-start spawns.
2. Save token cost — model is chosen per task, not welded to a role.
3. Optimize for local + Claude — one scheme spans both; local is opt-in per model.
4. More accurate tasks — right model per task, and independent review preserved where blind spots are expensive.

## Core move: decouple cost from expertise

Today an "agent" welds two independent things: *what expertise* (role) and *how much compute* (model). That weld is the root cause of the waste — SWE-means-Sonnet drags a mid model onto a rename; role-means-separate-spawn spins a cold context for trivial work. The redesign splits them into two orthogonal axes selected per task, and retires "agent" as a noun.

## Taxonomy

| Concept | Is | Loading |
|---|---|---|
| **Tier** | cost axis — which model | picked per task |
| **Profile** | expertise — SWE, QA, PM, DevOps, Security, … (today's "agents" minus the welded model) | lazy, injected |
| **Mode** | sub-specialty inside a profile — go/ts/api/db | lazy, within a profile |
| **Skill** | invokable procedure — audit, story | on `/invoke` |
| **advisor** | the always-on primary: orchestrator + advisor. Wears profiles inline or spawns them | always present |

There is no "SWE agent" — there is *a tier-`large` runtime wearing the SWE profile*. The advisor swaps profiles, never stacks them (context stays lean).

> Name: kept as `advisor` (trigger `[ADV]`) for now — a personal name (Alfred / Jeeves / Friday style) is deferred until the user is ready; swap is a one-pass rename.

## The advisor

Primary agent, default session persona. Advisor + orchestrator with a defined edit leash — a trusted counselor with authority to act, not an errand-runner and not a rubber stamp.

- **Inline (no spawn):** small / low-risk / SKIP-tier work — README, config tweak, one-file low-risk edit, small refactor. It already holds the context and capability.
- **Spawn a tiered profile:** large / multi-file / heavy-reasoning work.
- **Spawn a separate oversight profile:** any high-stakes work requiring independent review (see Duty classes).

The leash boundary is objective — classified by size + stakes, the same properties the review gate already uses — not the advisor's opinion of its own risk.

## Duty classes and separation of duties

Independence is a property of **context lineage**, not profile identity: the reviewer of an artifact must run in a context that did not author it. Enforced mechanically, not by discretion.

Every profile carries a duty class:

| Class | Meaning | Rule |
|---|---|---|
| `producer` | authors artifacts — SWE, DevOps, Hardware, CX, Marketing | may be worn inline or spawned |
| `oversight` | checks others' artifacts — QA, security-review | **`spawn-only`**: the loader refuses to inject it into any existing context; it can only instantiate fresh |
| `advisory` | counsels, doesn't author or check code — advisor, PM/business, Data, Legal, Tax, Botany, Logistics | may be worn inline or spawned |

Two enforcement points, no NxN matrix needed:

1. **`oversight` = `spawn-only`.** Co-location with a producer is structurally impossible — the reviewer is always a fresh context, so it can never be the author of what it reviews.
2. **Narrow handoff.** A spawned reviewer receives only the artifact + the one relevant standard — never the producer's reasoning/context. Independence isn't leaked back at handoff.

The only hard conflict is (producer-of-X, oversight-reviewing-X), and `spawn-only` already prevents it. If a genuine second forbidden pair ever appears (e.g. a compliance separation), it's one more duty-class tag — not a matrix to maintain.

This aligns with the advisor leash: it only edits low-risk / SKIP-tier work inline — exactly the class that needs no independent review — so giving it edit access creates zero conflict of interest. The leash and the SoD rule are the same boundary from two directions.

## Tier → model map

Model is explicit and fully editable — cost is tethered to model choice, so the map is data, not a hardcoded ladder.

```yaml
# orchestration/models.yaml
claude:
  heavy:    fable      # top reasoning / high-stakes
  large:    opus       # complex build, deep debug
  medium:   sonnet     # standard implementation
  small:    haiku      # lookups, renames, drafts
local:
  heavy:    { model: <tbd>, enabled: false }
  large:    { model: <tbd>, enabled: false }
  medium:   { model: <tbd>, enabled: false }
  small:    { model: <tbd>, enabled: false }
overrides:             # per profile or task-class — your judgment, made explicit
  security-review: fable
  api-design:      opus
  readme-edit:     haiku
```

- A tier resolves to a model per backend. `enabled: false` or an unloadable local model → Claude fallback, automatically, no rule change.
- `overrides` is where per-task/per-profile model pinning lives — the "I switch by perceived complexity" instinct, enforceable.
- Profiles may declare a **tier floor** (e.g. security-review never resolves below `heavy`) — a safety floor, not a preference.

## Firm backend + model selection (no discretion)

Backend and model are a pure function of objective task properties, checked in order. The advisor classifies (what kind of task is this) and applies the table; it does not decide backend freely.

| Condition (in order) | Result |
|---|---|
| Manual override present | wins over everything |
| High-stakes trigger (auth, money, crypto, secrets, irreversible, security, concurrency, new schema/API — the MUST-review list) | **Claude**, tier bumped up (safety floor) |
| Profile/task is heavy-tier (architecture, deep design/debug) | **Claude** |
| Everything else (standard / light) | **Local**, Claude as backup |
| Local model disabled or unavailable | **Claude** (backup) |
| Genuine ambiguity about stakes | **escalate to Claude / up-tier** (fail safe — never guess local on a maybe-sensitive task) |

The only residual judgment is the initial "what kind of task is this," which the advisor already performs for routing and review. It fails safe: on ambiguity it goes up, never down.

## Review gate (fixes the QA over-spawn)

Independent review is gated, not blanket. An `oversight` profile spawns **only** on a MUST trigger. SKIP-list work (docs, markdown, config, mechanical refactor, dependency bump) gets **zero** review spawns — the advisor self-reviews inline. This is the direct fix for QA being spawned on trivial changes and burning tokens.

- **Gate it** — MUST trigger only.
- **Parallelize it** — N high-stakes artifacts → up to N fresh oversight contexts at once, independent by construction.
- **Tier it** — routine review runs `medium`, security review runs `heavy`. Scaling ≠ paying top tier every time.

## Local + Claude interop

- `models.yaml` spans both backends under one tier scheme.
- Same-backend calls use the native spawn path; cross-backend calls route through the MCP bridge. Two transports, one dispatch shape.
- The bridge exposes agents as a registry so a Claude advisor can call a local profile and vice versa; backend is hidden from the caller.
- Bridge/local specifics (which models, bridge capability for the 9 currently Claude-only profiles) are a follow-up — this spec is backend-agnostic.

## Modes optimization (carried forward)

- **`modes/INDEX.md` manifest** — one line per mode, so a profile picks modes without opening full files.
- **Per-backend mode budget** — local (smaller) models load fewer / brief-only modes; Claude loads full. Slots into tier resolution.
- Future: two-tier mode files (`brief.md` always + `deep.md` on request); composable lens modes (security, perf, a11y, review) any profile can activate.

## Enhancements (carried forward)

- **`build.sh doctor`** — verifies every `~/.claude` link resolves (the dangling-symlink breakage the folder rename caused is exactly this class).
- **Generated bridge registry** — the agent/profile list the bridge exposes is generated from the profile set, so the registry and the profiles can't drift.

## Migration outline

**Status:** steps 2–6 implemented. Step 1 (folder rename + `~/.claude` re-point) is owned by the advisor, tracked separately — see Implementation notes below.

1. Rename repo folder `komodo-agentic-config-claude` → `komodo-agentic-config` (the `-claude` suffix is obsolete under the monorepo model). Re-point `~/.claude` (currently dangling from the rename).
2. Remap the 15 agents to profiles + duty classes:

| Legacy agent | Profile | Duty class | Default tier |
|---|---|---|---|
| advisor | **advisor** | advisory (primary) | medium |
| software-engineer | SWE | producer | small→heavy per task |
| quality-assurance | QA | oversight (`spawn-only`) | medium, security floor heavy |
| cyber-security | Security | oversight for review; producer for pentest/arch | heavy |
| devops | DevOps | producer | medium |
| data-analyst | Data | advisory | medium |
| hardware-engineer | Hardware | producer | large |
| business-architect | PM/Business | advisory | medium |
| lawyer | Legal | advisory | medium |
| customer-servicing | CX | producer | small |
| marketing | Marketing | producer | medium |
| tax-advisor | Tax | advisory | medium |
| botanist | Botany | advisory | small |
| logistics | Logistics | advisory | small |
| summarizer | (utility transform) | producer, no judgment | small |

3. Replace each `agent.md` frontmatter `model:` with `reasoning`/tier + duty class; profiles become lazy-loadable.
4. Add `orchestration/models.yaml` and `orchestration/policy.md` (the firm-rules table, single source).
5. Rewrite the advisor directive: edit leash, dispatch decision flow, native-vs-bridge routing, gated oversight spawns.
6. Add `modes/INDEX.md`, `build.sh doctor`, generated bridge registry.

## Implementation notes (steps 2–6, done)

- **Step 3 deviation, deliberate:** `agent.md` frontmatter `model:` was **kept**, not replaced — it's the fallback default for a plain spawn (no `reasoning`/tier substitution). `tier`, `duty_class`, and (oversight only) `spawn_only: true` were **added alongside it**, not in place of it. Tier→model resolution happens at spawn time via the advisor passing the `Agent` tool's `model` param from `orchestration/models.yaml`, never by rewriting frontmatter per task. This preserves current spawn behavior — every agent still resolves to a valid Claude model with zero orchestration changes upstream.
- **Directory rename deferred, not skipped.** The taxonomy (profile / duty class / tier) is expressed via frontmatter fields and advisor directive logic layered on the existing `agents/<name>/agent.md` layout — the physical `agents/` → `profiles/` rename from the Taxonomy table was **not** done. Claude Code resolves spawnable agents from `agents/<name>/agent.md` frontmatter; renaming the directories would break spawning for zero behavioral gain. Revisit only if a future runtime stops keying off this path.
- **`cyber-security` duty class:** frontmatter carries a single `duty_class: oversight` (not `spawn_only: true`) plus a body note explaining the mixed reality — its security-review function is oversight/spawn-only in practice, its pentest/architecture-design function is producer and may run inline. A single frontmatter tag can't express a per-function split; this was a judgment call, flagged for the advisor to confirm.
- **`software-engineer` tier:** frontmatter default is `medium` (matches its current `model: sonnet`), with a body note that the advisor actually resolves `small`→`heavy` per task. Same treatment for `quality-assurance`'s security-review floor (`heavy`), expressed as a `models.yaml` `overrides` entry (`security-review: fable`), not a frontmatter field.
- **`orchestration/models.yaml`** written without inline comments (repo-wide zero-comments rule applies to YAML/bash/JSON authored here) — the local tier stub uses `model: TBD` as a literal placeholder rather than the spec's `<tbd>` comment-adjacent notation.
- **Generated bridge registry** lands at `platforms/komodo-bridge/registry.generated.json` (not a new top-level `bridge/` dir) — `platforms/komodo-bridge/` is this repo's existing bridge-adapter directory (see `platforms/komodo-bridge/agent-roster.md`); a second top-level `bridge/` would duplicate it.
- **Pre-existing `runtime.json` / `scripts/set-runtime.sh`** (a prior, narrower manual-override mechanism) were left untouched and folded into `orchestration/policy.md` as condition 1 ("manual override present") of the firm ordered table — the two systems are complementary, not competing.

## Dispatch decision flow (reference)

```
task
 → classify: size + stakes + domain(→profile)
 → model  = models.yaml[tier], bumped up if high-stakes, floored per profile
 → backend = firm table (above)
 → execute:
     small/med + not independence-critical → advisor wears profile inline (no spawn)
     large/heavy                           → spawn tiered runtime + profile
     high-stakes review (MUST trigger)     → spawn SEPARATE oversight profile, fresh context, narrow handoff
```

## Open follow-ups

- Local model selections + enabling them in `models.yaml` (user-owned, when the PC is built).
- Bridge capability for the 9 currently Claude-only profiles (separate project, `komodo-mcp-bridge-api`).
- Per-task model binding beyond profile/task-class overrides, if tier assignment proves too coarse in practice.
